-- =============================================================
-- Seed: Initial Admin User
-- Password: Admin1234! (change before production)
-- Uses pgcrypto crypt() with bcrypt (bf, cost 12) — same
-- algorithm the Go service uses via golang.org/x/crypto/bcrypt.
-- =============================================================

DO $$
DECLARE
    v_user_id uuid := gen_random_uuid();
    v_role_id uuid;
BEGIN
    -- Resolve the AdminCheckOn role id
    SELECT id INTO v_role_id
    FROM users.identity_roles
    WHERE code = 'ADMIN_CHECK_ON';

    IF v_role_id IS NULL THEN
        RAISE EXCEPTION 'Role ADMIN_CHECK_ON not found. Run V2__seed_roles.sql first.';
    END IF;

    -- Insert the admin user (idempotent — skip if email already exists)
    INSERT INTO users.identity_users (
        id,
        email,
        password_hash,
        first_name,
        last_name,
        status,
        created_at,
        updated_at
    )
    VALUES (
        v_user_id,
        'admin@checkOn.com',
        crypt('Admin1234!', gen_salt('bf', 12)),
        'Admin',
        'CheckOn',
        'ACTIVE',
        now(),
        now()
    )
    ON CONFLICT (email) DO NOTHING;

    -- Re-resolve user id in case the row already existed
    SELECT id INTO v_user_id
    FROM users.identity_users
    WHERE email = 'admin@checkOn.com';

    -- Assign role (idempotent)
    INSERT INTO users.identity_user_roles (user_id, role_id, assigned_at)
    VALUES (v_user_id, v_role_id, now())
    ON CONFLICT (user_id, role_id) DO NOTHING;

    RAISE NOTICE 'Admin user ready: admin@checkOn.com (user_id: %)', v_user_id;
END;
$$;
