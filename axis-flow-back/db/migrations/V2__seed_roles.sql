-- Seed system roles
INSERT INTO users.identity_roles (id, code, name, description, scope, is_system)
VALUES
    (uuid_generate_v4(), 'ADMIN_CHECK_ON',  'AdminCheckOn',  'Full platform administration',              'GLOBAL', TRUE),
    (uuid_generate_v4(), 'CLIENTE',         'Cliente',       'End customer with limited access',          'GLOBAL', TRUE),
    (uuid_generate_v4(), 'EMPLEADO',        'Empleado',      'Internal employee',                         'GLOBAL', TRUE),
    (uuid_generate_v4(), 'SUPERVISOR',      'Supervisor',    'Team supervisor with elevated permissions',  'GLOBAL', TRUE),
    (uuid_generate_v4(), 'OPERACIONES',     'Operaciones',   'Operations team member',                    'GLOBAL', TRUE),
    (uuid_generate_v4(), 'GESTOR',          'Gestor',        'Case or account manager',                   'GLOBAL', TRUE),
    (uuid_generate_v4(), 'ADMINISTRADOR',   'Administrador', 'Tenant administrator',                      'GLOBAL', TRUE),
    (uuid_generate_v4(), 'RH',              'RH',            'Human resources',                           'GLOBAL', TRUE)
ON CONFLICT (code) DO NOTHING;
