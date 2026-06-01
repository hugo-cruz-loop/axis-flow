-- V3: Add missing columns per spec and fix nullability gaps.
-- This is an additive migration — V1 and V2 are NOT modified.

CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- identity_users
ALTER TABLE users.identity_users ADD COLUMN IF NOT EXISTS password_changed_at TIMESTAMPTZ NULL;
ALTER TABLE users.identity_users ALTER COLUMN tenant_id DROP NOT NULL;

-- identity_sessions
ALTER TABLE users.identity_sessions ADD COLUMN IF NOT EXISTS revoked_reason VARCHAR(150) NULL;
ALTER TABLE users.identity_sessions ADD COLUMN IF NOT EXISTS device_name VARCHAR(150) NULL;
ALTER TABLE users.identity_sessions ADD COLUMN IF NOT EXISTS user_agent TEXT NULL;

-- identity_tokens
ALTER TABLE users.identity_tokens ADD COLUMN IF NOT EXISTS created_ip INET NULL;

-- identity_audit_log
ALTER TABLE users.identity_audit_log ADD COLUMN IF NOT EXISTS user_agent TEXT NULL;
