-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "citext";

-- Create schema
CREATE SCHEMA IF NOT EXISTS users;

-- ─── identity_users ──────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS users.identity_users (
    id                    UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id             UUID        NOT NULL,
    email                 CITEXT      NOT NULL,
    password_hash         TEXT        NOT NULL,
    first_name            VARCHAR(100) NOT NULL,
    last_name             VARCHAR(100) NOT NULL,
    phone                 VARCHAR(30),
    status                VARCHAR(30)  NOT NULL DEFAULT 'PENDING_ACTIVATION'
                              CHECK (status IN ('PENDING_ACTIVATION','ACTIVE','INACTIVE','SUSPENDED','LOCKED','DELETED')),
    failed_login_attempts INT         NOT NULL DEFAULT 0,
    locked_until          TIMESTAMPTZ,
    last_login_at         TIMESTAMPTZ,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at            TIMESTAMPTZ,
    created_by            UUID,
    updated_by            UUID,
    CONSTRAINT uq_identity_users_email UNIQUE (email)
);

CREATE INDEX IF NOT EXISTS idx_identity_users_tenant_id ON users.identity_users (tenant_id);
CREATE INDEX IF NOT EXISTS idx_identity_users_status    ON users.identity_users (status);
CREATE INDEX IF NOT EXISTS idx_identity_users_email     ON users.identity_users (email);

-- ─── identity_roles ──────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS users.identity_roles (
    id          UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    code        VARCHAR(50) NOT NULL,
    name        VARCHAR(100) NOT NULL,
    description TEXT,
    scope       VARCHAR(20) NOT NULL DEFAULT 'GLOBAL'
                    CHECK (scope IN ('GLOBAL','TENANT')),
    is_system   BOOLEAN     NOT NULL DEFAULT FALSE,
    CONSTRAINT uq_identity_roles_code UNIQUE (code)
);

-- ─── identity_permissions ────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS users.identity_permissions (
    id          UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    code        VARCHAR(100) NOT NULL,
    name        VARCHAR(100) NOT NULL,
    module      VARCHAR(50),
    description TEXT,
    CONSTRAINT uq_identity_permissions_code UNIQUE (code)
);

-- ─── identity_user_roles ─────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS users.identity_user_roles (
    user_id     UUID        NOT NULL REFERENCES users.identity_users(id) ON DELETE CASCADE,
    role_id     UUID        NOT NULL REFERENCES users.identity_roles(id) ON DELETE CASCADE,
    assigned_by UUID,
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, role_id)
);

CREATE INDEX IF NOT EXISTS idx_identity_user_roles_user_id ON users.identity_user_roles (user_id);
CREATE INDEX IF NOT EXISTS idx_identity_user_roles_role_id ON users.identity_user_roles (role_id);

-- ─── identity_role_permissions ───────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS users.identity_role_permissions (
    role_id       UUID NOT NULL REFERENCES users.identity_roles(id) ON DELETE CASCADE,
    permission_id UUID NOT NULL REFERENCES users.identity_permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

-- ─── identity_sessions ───────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS users.identity_sessions (
    id                 UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id            UUID        NOT NULL REFERENCES users.identity_users(id) ON DELETE CASCADE,
    refresh_token_hash TEXT        NOT NULL,
    device_id          VARCHAR(255),
    device_type        VARCHAR(20) NOT NULL DEFAULT 'WEB'
                           CHECK (device_type IN ('WEB','ANDROID','IOS')),
    ip_address         INET,
    revoked_at         TIMESTAMPTZ,
    expires_at         TIMESTAMPTZ NOT NULL,
    last_used_at       TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_identity_sessions_user_id           ON users.identity_sessions (user_id);
CREATE INDEX IF NOT EXISTS idx_identity_sessions_refresh_token_hash ON users.identity_sessions (refresh_token_hash);

-- ─── identity_tokens ─────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS users.identity_tokens (
    id         UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id    UUID        NOT NULL REFERENCES users.identity_users(id) ON DELETE CASCADE,
    token_hash TEXT        NOT NULL,
    type       VARCHAR(30) NOT NULL
                   CHECK (type IN ('ACTIVATION','PASSWORD_RESET','EMAIL_VERIFICATION')),
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_identity_tokens_user_id    ON users.identity_tokens (user_id);
CREATE INDEX IF NOT EXISTS idx_identity_tokens_token_hash ON users.identity_tokens (token_hash);

-- ─── identity_audit_log ──────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS users.identity_audit_log (
    id             UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    actor_user_id  UUID,
    target_user_id UUID,
    action         VARCHAR(100) NOT NULL,
    metadata       JSONB,
    ip_address     INET,
    trace_id       VARCHAR(128),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_identity_audit_log_actor_user_id  ON users.identity_audit_log (actor_user_id);
CREATE INDEX IF NOT EXISTS idx_identity_audit_log_target_user_id ON users.identity_audit_log (target_user_id);
CREATE INDEX IF NOT EXISTS idx_identity_audit_log_action         ON users.identity_audit_log (action);
