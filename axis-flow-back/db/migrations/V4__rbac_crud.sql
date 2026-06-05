-- V4: Add updated_at columns to RBAC tables.
-- This is an additive migration — V1–V3 are NOT modified.

ALTER TABLE users.identity_roles ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
ALTER TABLE users.identity_permissions ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
