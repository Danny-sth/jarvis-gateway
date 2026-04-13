-- Mobile sessions for QR-auth linking mobile app to Telegram profile
-- Token is stored as SHA256 hash for security (plaintext never stored)
CREATE TABLE IF NOT EXISTS mobile_sessions (
    id SERIAL PRIMARY KEY,
    telegram_id BIGINT NOT NULL,
    token_hash VARCHAR(64) NOT NULL UNIQUE,
    device_name VARCHAR(255),
    created_at TIMESTAMP DEFAULT NOW(),
    expires_at TIMESTAMP NOT NULL,
    last_used_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_mobile_sessions_token_hash ON mobile_sessions(token_hash);
CREATE INDEX IF NOT EXISTS idx_mobile_sessions_telegram_id ON mobile_sessions(telegram_id);

-- QR codes for auth flow (temporary, 5 min TTL)
CREATE TABLE IF NOT EXISTS qr_auth_codes (
    id SERIAL PRIMARY KEY,
    code VARCHAR(8) NOT NULL UNIQUE,
    telegram_id BIGINT NOT NULL,
    device_name VARCHAR(255),
    created_at TIMESTAMP DEFAULT NOW(),
    expires_at TIMESTAMP NOT NULL,
    used BOOLEAN DEFAULT FALSE
);

CREATE INDEX IF NOT EXISTS idx_qr_auth_codes_code ON qr_auth_codes(code);
