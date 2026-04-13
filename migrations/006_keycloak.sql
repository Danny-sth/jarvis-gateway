-- Add Keycloak integration fields to users table

-- Add keycloak_sub column (Keycloak user UUID)
ALTER TABLE users ADD COLUMN IF NOT EXISTS keycloak_sub VARCHAR(255) UNIQUE;

-- Add email column if not exists
ALTER TABLE users ADD COLUMN IF NOT EXISTS email VARCHAR(255);

-- Add role column for direct role mapping (user/admin)
-- This supplements the existing RBAC system for simpler JWT-based checks
ALTER TABLE users ADD COLUMN IF NOT EXISTS role VARCHAR(50) DEFAULT 'user';

-- Create index for keycloak_sub lookups
CREATE INDEX IF NOT EXISTS idx_users_keycloak_sub ON users(keycloak_sub);

-- Create index for email lookups
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

-- Update existing admin user with role
UPDATE users SET role = 'admin'
WHERE telegram_id IN (
    SELECT user_id FROM user_roles ur
    JOIN roles r ON ur.role_id = r.id
    WHERE r.name = 'admin'
);
