CREATE SCHEMA IF NOT EXISTS empresas;

-- 1. Main company table
CREATE TABLE IF NOT EXISTS empresas.empresas_empresa (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    nombre VARCHAR(255) NOT NULL,
    direccion TEXT,
    telefono VARCHAR(20),
    representante_id UUID NOT NULL,
    plan_id BIGINT NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'PENDING_PAYMENT'
        CHECK (status IN ('PENDING_PAYMENT','ACTIVE','INACTIVE','SUSPENDED')),
    vigencia DATE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_empresa_representante
        FOREIGN KEY (representante_id) REFERENCES users.identity_users(id) ON DELETE RESTRICT,
    CONSTRAINT fk_empresa_plan
        FOREIGN KEY (plan_id) REFERENCES catalogos.catalog_subscription_plans(id) ON DELETE RESTRICT
);

-- 2. Fiscal data (1-to-1 with empresa)
CREATE TABLE IF NOT EXISTS empresas.empresas_datosfiscales (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    empresa_id BIGINT NOT NULL UNIQUE,
    rfc VARCHAR(13) NOT NULL UNIQUE,
    razon_social VARCHAR(255) NOT NULL,
    logo_url VARCHAR(1024),
    imss_patronal VARCHAR(50),
    repse VARCHAR(50),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_datosfiscales_empresa
        FOREIGN KEY (empresa_id) REFERENCES empresas.empresas_empresa(id) ON DELETE CASCADE
);

-- 3. Payment history
CREATE TABLE IF NOT EXISTS empresas.empresas_pagos (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    empresa_id BIGINT NOT NULL,
    token_pago VARCHAR(255) NOT NULL UNIQUE,
    stripe_session_id VARCHAR(255),
    estatus_pago VARCHAR(50) NOT NULL DEFAULT 'PENDING'
        CHECK (estatus_pago IN ('PENDING','PAID','FAILED','REFUNDED')),
    monto NUMERIC(12,2) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_pagos_empresa
        FOREIGN KEY (empresa_id) REFERENCES empresas.empresas_empresa(id) ON DELETE RESTRICT
);

-- 4. Legal representatives
CREATE TABLE IF NOT EXISTS empresas.empresas_apoderado (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    empresa_id BIGINT NOT NULL,
    nombre VARCHAR(255) NOT NULL,
    curp VARCHAR(18) NOT NULL,
    rfc VARCHAR(13) NOT NULL,
    email VARCHAR(254) NOT NULL,
    telefono VARCHAR(20),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_apoderado_empresa
        FOREIGN KEY (empresa_id) REFERENCES empresas.empresas_empresa(id) ON DELETE CASCADE
);

-- 5. Company service catalog
CREATE TABLE IF NOT EXISTS empresas.empresas_servicio (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    empresa_id BIGINT NOT NULL,
    nombre VARCHAR(255) NOT NULL,
    descripcion TEXT,
    precio NUMERIC(12,2) NOT NULL,
    status_activo BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_servicio_empresa
        FOREIGN KEY (empresa_id) REFERENCES empresas.empresas_empresa(id) ON DELETE CASCADE
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_empresa_representante ON empresas.empresas_empresa(representante_id);
CREATE INDEX IF NOT EXISTS idx_empresa_plan ON empresas.empresas_empresa(plan_id);
CREATE INDEX IF NOT EXISTS idx_pagos_empresa ON empresas.empresas_pagos(empresa_id);
CREATE INDEX IF NOT EXISTS idx_apoderado_empresa ON empresas.empresas_apoderado(empresa_id);
CREATE INDEX IF NOT EXISTS idx_servicio_empresa ON empresas.empresas_servicio(empresa_id);
