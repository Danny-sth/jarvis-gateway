-- Add password_hash column to existing users table for web authentication
ALTER TABLE users ADD COLUMN IF NOT EXISTS password_hash VARCHAR(255);

-- Change username column to be unique (currently it's not unique, only telegram_id is)
-- First, make sure existing usernames are unique or null
UPDATE users SET username = 'telegram_' || telegram_id WHERE username IS NULL OR username = '';

-- Now make username unique
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_username_unique;
ALTER TABLE users ADD CONSTRAINT users_username_unique UNIQUE (username);

-- No default admin user - existing users can set their password_hash manually
