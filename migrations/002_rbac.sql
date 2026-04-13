-- RBAC (Role-Based Access Control) tables

-- ==================== USERS ====================

CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    telegram_id BIGINT UNIQUE NOT NULL,
    username VARCHAR(100),
    first_name VARCHAR(100),
    last_name VARCHAR(100),
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_users_telegram_id ON users(telegram_id);

-- ==================== ROLES ====================

CREATE TABLE IF NOT EXISTS roles (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) UNIQUE NOT NULL,
    description TEXT,
    is_composite BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT NOW()
);

-- ==================== PERMISSIONS ====================

CREATE TABLE IF NOT EXISTS permissions (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) UNIQUE NOT NULL,  -- 'gws:gmail:read', 'tool:weather'
    resource_type VARCHAR(50) NOT NULL, -- 'gws', 'tool', 'sdk', 'memory', 'admin'
    action VARCHAR(50) NOT NULL,        -- 'read', 'write', 'execute', 'manage'
    description TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);

-- ==================== ROLE PERMISSIONS ====================

CREATE TABLE IF NOT EXISTS role_permissions (
    role_id INT REFERENCES roles(id) ON DELETE CASCADE,
    permission_id INT REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

-- ==================== ROLE INHERITANCE ====================

CREATE TABLE IF NOT EXISTS role_inheritance (
    parent_role_id INT REFERENCES roles(id) ON DELETE CASCADE,
    child_role_id INT REFERENCES roles(id) ON DELETE CASCADE,
    PRIMARY KEY (parent_role_id, child_role_id)
);

-- ==================== GROUPS ====================

CREATE TABLE IF NOT EXISTS groups (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) UNIQUE NOT NULL,
    description TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS group_roles (
    group_id INT REFERENCES groups(id) ON DELETE CASCADE,
    role_id INT REFERENCES roles(id) ON DELETE CASCADE,
    PRIMARY KEY (group_id, role_id)
);

CREATE TABLE IF NOT EXISTS user_groups (
    user_id BIGINT NOT NULL,  -- telegram_id
    group_id INT REFERENCES groups(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, group_id)
);

-- ==================== USER ROLES ====================

CREATE TABLE IF NOT EXISTS user_roles (
    user_id BIGINT NOT NULL,  -- telegram_id
    role_id INT REFERENCES roles(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, role_id)
);

-- ==================== USER CREDENTIALS ====================

CREATE TABLE IF NOT EXISTS user_credentials (
    id SERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,  -- telegram_id
    provider VARCHAR(50) NOT NULL,  -- 'google'
    tokens JSONB NOT NULL,  -- OAuth tokens (encrypted in app layer)
    scopes TEXT[],
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    expires_at TIMESTAMP,
    UNIQUE(user_id, provider)
);

CREATE INDEX IF NOT EXISTS idx_user_credentials_user ON user_credentials(user_id);

-- ==================== SEED DEFAULT ROLES ====================
-- Simple 2-role system: user (basic) and admin (full access)

INSERT INTO roles (name, description, is_composite) VALUES
    ('user', 'Basic user - own memory, weather, web search', FALSE),
    ('admin', 'Full admin access to everything', FALSE)
ON CONFLICT (name) DO NOTHING;

-- ==================== SEED PERMISSIONS ====================

INSERT INTO permissions (name, resource_type, action, description) VALUES
    -- Core tools
    ('tool:weather', 'tool', 'execute', 'Get weather information'),
    ('tool:cortex_search', 'tool', 'execute', 'Search in cortex memory'),
    ('tool:cortex_store', 'tool', 'execute', 'Store in cortex memory'),
    ('tool:system_info', 'tool', 'execute', 'Get system information'),

    -- Obsidian
    ('tool:obsidian:read', 'tool', 'read', 'Read Obsidian notes'),
    ('tool:obsidian:write', 'tool', 'write', 'Write Obsidian notes'),

    -- GWS OAuth
    ('gws:oauth', 'gws', 'execute', 'Connect Google account via OAuth'),

    -- GWS Gmail
    ('gws:gmail:read', 'gws', 'read', 'Read Gmail messages'),
    ('gws:gmail:write', 'gws', 'write', 'Send/modify Gmail'),

    -- GWS Calendar
    ('gws:calendar:read', 'gws', 'read', 'Read calendar events'),
    ('gws:calendar:write', 'gws', 'write', 'Create/modify calendar events'),

    -- GWS Drive
    ('gws:drive:read', 'gws', 'read', 'Read Drive files'),

    -- GWS Tasks
    ('gws:tasks:read', 'gws', 'read', 'Read tasks'),
    ('gws:tasks:write', 'gws', 'write', 'Create/modify tasks'),

    -- Memory
    ('memory:read_own', 'memory', 'read', 'Read own memories'),
    ('memory:read_all', 'memory', 'read', 'Read all users memories'),
    ('memory:write', 'memory', 'write', 'Write memories'),

    -- SDK tools
    ('sdk:web_search', 'sdk', 'execute', 'Web search via SDK'),
    ('sdk:bash', 'sdk', 'execute', 'Execute bash commands'),
    ('sdk:text_editor', 'sdk', 'execute', 'Edit files via SDK'),
    ('sdk:computer', 'sdk', 'execute', 'Computer control'),

    -- Admin
    ('admin:manage_users', 'admin', 'manage', 'Manage users'),
    ('admin:manage_roles', 'admin', 'manage', 'Manage roles and permissions')
ON CONFLICT (name) DO NOTHING;

-- ==================== SEED ROLE PERMISSIONS ====================

-- user: basic tools + own memory + GWS (own account)
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name = 'user' AND p.name IN (
    'tool:weather',
    'sdk:web_search',
    'tool:cortex_search',
    'tool:cortex_store',
    'memory:read_own',
    'memory:write',
    'gws:oauth',
    'gws:gmail:read',
    'gws:gmail:write',
    'gws:calendar:read',
    'gws:calendar:write',
    'gws:drive:read',
    'gws:tasks:read',
    'gws:tasks:write'
)
ON CONFLICT DO NOTHING;

-- admin: ALL permissions
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name = 'admin'
ON CONFLICT DO NOTHING;

-- ==================== SEED OWNER USER ====================

INSERT INTO users (telegram_id, username, first_name, is_active) VALUES
    (764733417, 'dannykudinov', 'Danny', TRUE)
ON CONFLICT (telegram_id) DO NOTHING;

-- Assign admin role to owner
INSERT INTO user_roles (user_id, role_id)
SELECT 764733417, id FROM roles WHERE name = 'admin'
ON CONFLICT DO NOTHING;
