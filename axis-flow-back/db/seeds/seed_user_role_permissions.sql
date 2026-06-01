-- =============================================================
-- Seed: Users & Roles permissions
-- Target user id: cf0f44e9-c062-49c6-b718-d7788706c589
--
-- Strategy:
--   1. Insert all users/roles permissions into identity_permissions.
--   2. Assign them to the ADMIN_CHECK_ON role via identity_role_permissions.
--   3. Ensure the target user has the ADMIN_CHECK_ON role.
-- =============================================================

DO $$
DECLARE
    v_user_id   uuid := 'cf0f44e9-c062-49c6-b718-d7788706c589';
    v_role_id   uuid;
    v_perm_id   uuid;
BEGIN

    -- ─── 1. Resolve ADMIN_CHECK_ON role ──────────────────────────────────────
    SELECT id INTO v_role_id
    FROM users.identity_roles
    WHERE code = 'ADMIN_CHECK_ON';

    IF v_role_id IS NULL THEN
        RAISE EXCEPTION 'Role ADMIN_CHECK_ON not found. Run V2__seed_roles.sql first.';
    END IF;

    -- ─── 2. Confirm target user exists ───────────────────────────────────────
    IF NOT EXISTS (SELECT 1 FROM users.identity_users WHERE id = v_user_id) THEN
        RAISE EXCEPTION 'User % not found.', v_user_id;
    END IF;

    -- ─── 3. Seed permissions (module: users) ─────────────────────────────────

    -- users:list — list all users / filter by role
    INSERT INTO users.identity_permissions (id, code, name, module, description)
    VALUES (gen_random_uuid(), 'users:list', 'List Users', 'users', 'View and filter the full user list')
    ON CONFLICT (code) DO NOTHING;

    -- users:read — view a single user profile
    INSERT INTO users.identity_permissions (id, code, name, module, description)
    VALUES (gen_random_uuid(), 'users:read', 'Read User', 'users', 'Read a single user profile and details')
    ON CONFLICT (code) DO NOTHING;

    -- users:create — create new users (internal / admin / mobile / client)
    INSERT INTO users.identity_permissions (id, code, name, module, description)
    VALUES (gen_random_uuid(), 'users:create', 'Create User', 'users', 'Create internal, admin, app and client users')
    ON CONFLICT (code) DO NOTHING;

    -- users:update — update user data (firebase token, profile fields)
    INSERT INTO users.identity_permissions (id, code, name, module, description)
    VALUES (gen_random_uuid(), 'users:update', 'Update User', 'users', 'Update user profile and firebase token')
    ON CONFLICT (code) DO NOTHING;

    -- users:activate — activate account / reset password
    INSERT INTO users.identity_permissions (id, code, name, module, description)
    VALUES (gen_random_uuid(), 'users:activate', 'Activate User', 'users', 'Activate accounts and process password resets')
    ON CONFLICT (code) DO NOTHING;

    -- users:delete — soft-delete / deactivate users
    INSERT INTO users.identity_permissions (id, code, name, module, description)
    VALUES (gen_random_uuid(), 'users:delete', 'Delete User', 'users', 'Soft-delete or deactivate user accounts')
    ON CONFLICT (code) DO NOTHING;

    -- users:delete_all — destructive dev/test reset (only non-production)
    INSERT INTO users.identity_permissions (id, code, name, module, description)
    VALUES (gen_random_uuid(), 'users:delete_all', 'Delete All Data', 'users', 'Destructive reset — dev/test only, disabled in production')
    ON CONFLICT (code) DO NOTHING;

    -- ─── 4. Seed permissions (module: roles) ─────────────────────────────────

    -- roles:list — list all roles
    INSERT INTO users.identity_permissions (id, code, name, module, description)
    VALUES (gen_random_uuid(), 'roles:list', 'List Roles', 'roles', 'View all available roles')
    ON CONFLICT (code) DO NOTHING;

    -- roles:read — view a single role and its permissions
    INSERT INTO users.identity_permissions (id, code, name, module, description)
    VALUES (gen_random_uuid(), 'roles:read', 'Read Role', 'roles', 'Read a single role with its assigned permissions')
    ON CONFLICT (code) DO NOTHING;

    -- roles:assign — assign a role to a user
    INSERT INTO users.identity_permissions (id, code, name, module, description)
    VALUES (gen_random_uuid(), 'roles:assign', 'Assign Role', 'roles', 'Assign or change a role for a user')
    ON CONFLICT (code) DO NOTHING;

    -- roles:revoke — remove a role from a user
    INSERT INTO users.identity_permissions (id, code, name, module, description)
    VALUES (gen_random_uuid(), 'roles:revoke', 'Revoke Role', 'roles', 'Remove a role assignment from a user')
    ON CONFLICT (code) DO NOTHING;

    -- roles:read_permissions — view permissions assigned to a role
    INSERT INTO users.identity_permissions (id, code, name, module, description)
    VALUES (gen_random_uuid(), 'roles:read_permissions', 'Read Role Permissions', 'roles', 'View the permission matrix for a role')
    ON CONFLICT (code) DO NOTHING;

    -- ─── 5. Assign all users+roles permissions to ADMIN_CHECK_ON role ─────────

    INSERT INTO users.identity_role_permissions (role_id, permission_id)
    SELECT v_role_id, id
    FROM users.identity_permissions
    WHERE module IN ('users', 'roles')
    ON CONFLICT (role_id, permission_id) DO NOTHING;

    RAISE NOTICE 'Permissions assigned to role ADMIN_CHECK_ON: % rows',
        (SELECT count(*) FROM users.identity_role_permissions
         WHERE role_id = v_role_id
           AND permission_id IN (
               SELECT id FROM users.identity_permissions WHERE module IN ('users', 'roles')
           ));

    -- ─── 6. Ensure target user has the ADMIN_CHECK_ON role ───────────────────

    INSERT INTO users.identity_user_roles (user_id, role_id, assigned_at)
    VALUES (v_user_id, v_role_id, now())
    ON CONFLICT (user_id, role_id) DO NOTHING;

    RAISE NOTICE 'User % → role ADMIN_CHECK_ON: ready', v_user_id;

END;
$$;

-- ─── Verification query ────────────────────────────────────────────────────────
-- Run this to confirm the permissions are in place:
SELECT
    u.email,
    r.code   AS role,
    p.module,
    p.code   AS permission,
    p.name
FROM users.identity_users         u
JOIN users.identity_user_roles    ur ON ur.user_id    = u.id
JOIN users.identity_roles         r  ON r.id          = ur.role_id
JOIN users.identity_role_permissions rp ON rp.role_id = r.id
JOIN users.identity_permissions   p  ON p.id          = rp.permission_id
WHERE u.id = 'cf0f44e9-c062-49c6-b718-d7788706c589'
  AND p.module IN ('users', 'roles')
ORDER BY p.module, p.code;
