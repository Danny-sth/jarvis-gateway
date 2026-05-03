package rbac

import (
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"
)

// Service handles RBAC operations with caching
type Service struct {
	db       *sql.DB
	cache    *cache
	cacheTTL time.Duration
}

// cache stores user permissions with TTL
type cache struct {
	mu    sync.RWMutex
	items map[int64]*cacheItem
}

type cacheItem struct {
	permissions []string
	tools       []string
	expiresAt   time.Time
}

// NewService creates a new RBAC service
// cacheTTLMin: cache TTL in minutes (default 5 if 0)
func NewService(db *sql.DB, cacheTTLMin int) *Service {
	ttl := time.Duration(cacheTTLMin) * time.Minute
	if ttl == 0 {
		ttl = 5 * time.Minute // fallback default
	}
	return &Service{
		db: db,
		cache: &cache{
			items: make(map[int64]*cacheItem),
		},
		cacheTTL: ttl,
	}
}

// GetUserPermissions returns all permissions for a user (including inherited)
func (s *Service) GetUserPermissions(userID int64) ([]string, error) {
	// Check cache first
	if perms := s.cache.get(userID); perms != nil {
		return perms.permissions, nil
	}

	// Query database - get direct permissions + inherited via roles
	query := `
		WITH RECURSIVE role_tree AS (
			-- Direct user roles
			SELECT role_id FROM user_roles WHERE user_id = $1
			UNION
			-- Group roles
			SELECT gr.role_id
			FROM user_groups ug
			JOIN group_roles gr ON ug.group_id = gr.group_id
			WHERE ug.user_id = $1
			UNION
			-- Inherited roles (recursive)
			SELECT ri.child_role_id
			FROM role_tree rt
			JOIN role_inheritance ri ON rt.role_id = ri.parent_role_id
		)
		SELECT DISTINCT p.name
		FROM role_tree rt
		JOIN role_permissions rp ON rt.role_id = rp.role_id
		JOIN permissions p ON rp.permission_id = p.id
	`

	rows, err := s.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query permissions: %w", err)
	}
	defer rows.Close()

	var permissions []string
	for rows.Next() {
		var perm string
		if err := rows.Scan(&perm); err != nil {
			return nil, fmt.Errorf("failed to scan permission: %w", err)
		}
		permissions = append(permissions, perm)
	}

	// Get tools for these permissions
	tools := GetToolsForPermissions(permissions)

	// Cache the result
	s.cache.set(userID, permissions, tools, s.cacheTTL)

	log.Printf("[rbac] User %d has %d permissions, %d tools", userID, len(permissions), len(tools))
	return permissions, nil
}

// GetAllowedTools returns list of MCP tool names the user can access
func (s *Service) GetAllowedTools(userID int64) ([]string, error) {
	// Check cache first
	if cached := s.cache.get(userID); cached != nil {
		return cached.tools, nil
	}

	// Get permissions (this will also cache tools)
	_, err := s.GetUserPermissions(userID)
	if err != nil {
		return nil, err
	}

	// Now cache should have the tools
	if cached := s.cache.get(userID); cached != nil {
		return cached.tools, nil
	}

	return nil, nil
}

// HasPermission checks if user has a specific permission
func (s *Service) HasPermission(userID int64, permission string) (bool, error) {
	permissions, err := s.GetUserPermissions(userID)
	if err != nil {
		return false, err
	}

	for _, p := range permissions {
		if p == permission {
			return true, nil
		}
	}
	return false, nil
}

// InvalidateCache removes user from cache (call when roles change)
func (s *Service) InvalidateCache(userID int64) {
	s.cache.delete(userID)
}

// InvalidateAllCache clears entire cache
func (s *Service) InvalidateAllCache() {
	s.cache.clear()
}

// EnsureUser updates existing user info (does NOT create new users)
// New users must register via /start command which creates them in Keycloak first
func (s *Service) EnsureUser(userID int64, username, firstName, lastName string) error {
	// Only update existing users - do NOT create new users without Keycloak
	// Keycloak is the primary source of truth for user identity
	query := `
		UPDATE users SET
			username = COALESCE(NULLIF($2, ''), username),
			first_name = COALESCE(NULLIF($3, ''), first_name),
			last_name = COALESCE(NULLIF($4, ''), last_name),
			updated_at = NOW()
		WHERE telegram_id = $1
	`
	result, err := s.db.Exec(query, userID, nullString(username), nullString(firstName), nullString(lastName))
	if err != nil {
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		// User doesn't exist - they need to register via /start
		log.Printf("[rbac] User %d not found, must register via /start", userID)
	}

	return nil
}

// AssignRole assigns a role to user
func (s *Service) AssignRole(userID int64, roleName string) error {
	query := `
		INSERT INTO user_roles (user_id, role_id)
		SELECT $1, id FROM roles WHERE name = $2
		ON CONFLICT DO NOTHING
	`
	_, err := s.db.Exec(query, userID, roleName)
	if err != nil {
		return err
	}
	s.InvalidateCache(userID)
	return nil
}

// GetUserRoles returns role names for a user
func (s *Service) GetUserRoles(userID int64) ([]string, error) {
	query := `
		SELECT r.name
		FROM user_roles ur
		JOIN roles r ON ur.role_id = r.id
		WHERE ur.user_id = $1
	`
	rows, err := s.db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []string
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	return roles, nil
}

// IsUserActive checks if user account is active
func (s *Service) IsUserActive(userID int64) (bool, error) {
	var isActive bool
	err := s.db.QueryRow(
		"SELECT is_active FROM users WHERE telegram_id = $1",
		userID,
	).Scan(&isActive)

	if err == sql.ErrNoRows {
		// User doesn't exist, create as inactive (needs role assignment)
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return isActive, nil
}

// GetUserIDByTelegramID returns internal users.id from telegram_id
// This is used to sync with RBAC tables which use users.id, not telegram_id
func (s *Service) GetUserIDByTelegramID(telegramID int64) (int64, error) {
	var userID int64
	err := s.db.QueryRow(
		"SELECT id FROM users WHERE telegram_id = $1",
		telegramID,
	).Scan(&userID)

	if err == sql.ErrNoRows {
		return 0, fmt.Errorf("user not found: telegram_id=%d", telegramID)
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get user ID: %w", err)
	}
	return userID, nil
}

// EnsureUserRole ensures user has at least the default 'user' role
// Call this after user registration to grant basic permissions
func (s *Service) EnsureUserRole(userID int64) error {
	// Check if user has any roles
	var count int
	err := s.db.QueryRow(
		"SELECT COUNT(*) FROM user_roles WHERE user_id = $1",
		userID,
	).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check user roles: %w", err)
	}

	if count == 0 {
		// Assign default 'user' role (id=1)
		_, err := s.db.Exec(
			"INSERT INTO user_roles (user_id, role_id) VALUES ($1, 1) ON CONFLICT DO NOTHING",
			userID,
		)
		if err != nil {
			return fmt.Errorf("failed to assign default role: %w", err)
		}
		log.Printf("[rbac] Assigned default 'user' role to user %d", userID)
		s.InvalidateCache(userID)
	}
	return nil
}

// cache methods
func (c *cache) get(userID int64) *cacheItem {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, ok := c.items[userID]
	if !ok {
		return nil
	}
	if time.Now().After(item.expiresAt) {
		return nil
	}
	return item
}

func (c *cache) set(userID int64, permissions, tools []string, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[userID] = &cacheItem{
		permissions: permissions,
		tools:       tools,
		expiresAt:   time.Now().Add(ttl),
	}
}

func (c *cache) delete(userID int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, userID)
}

func (c *cache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[int64]*cacheItem)
}

func nullString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
