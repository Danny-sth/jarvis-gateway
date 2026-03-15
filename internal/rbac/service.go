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
	db    *sql.DB
	cache *cache
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

const cacheTTL = 5 * time.Minute

// NewService creates a new RBAC service
func NewService(db *sql.DB) *Service {
	return &Service{
		db: db,
		cache: &cache{
			items: make(map[int64]*cacheItem),
		},
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
	s.cache.set(userID, permissions, tools)

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

// EnsureUser creates user if not exists
func (s *Service) EnsureUser(userID int64, username, firstName, lastName string) error {
	query := `
		INSERT INTO users (telegram_id, username, first_name, last_name)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (telegram_id) DO UPDATE SET
			username = COALESCE(EXCLUDED.username, users.username),
			first_name = COALESCE(EXCLUDED.first_name, users.first_name),
			last_name = COALESCE(EXCLUDED.last_name, users.last_name),
			updated_at = NOW()
	`
	_, err := s.db.Exec(query, userID, nullString(username), nullString(firstName), nullString(lastName))
	return err
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

func (c *cache) set(userID int64, permissions, tools []string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[userID] = &cacheItem{
		permissions: permissions,
		tools:       tools,
		expiresAt:   time.Now().Add(cacheTTL),
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
