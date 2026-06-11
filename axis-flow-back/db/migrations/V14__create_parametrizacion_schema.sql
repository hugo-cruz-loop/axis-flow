-- V14: Create Parametrizacion service schema and integrity rules.
--
-- PR1 scope for 13_Parametrizacion_Service_Spec:
--   * Owns schema parametrizacion.
--   * Uses BIGINT identifiers to match empresas and catalogos tables.
--   * Foreign keys use ON DELETE RESTRICT to avoid cross-service cascading.
--   * Unique constraints encode the tenant-specific singleton rules.
--   * Tables with updated_at get BEFORE UPDATE triggers.

CREATE SCHEMA IF NOT EXISTS parametrizacion;

CREATE OR REPLACE FUNCTION parametrizacion.fn_set_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TABLE IF NOT EXISTS parametrizacion.parametrizacion_sistema (
    clave_parametro VARCHAR(255) PRIMARY KEY,
    valor TEXT NOT NULL,
    descripcion TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_clave_parametro_format CHECK (clave_parametro ~ '^[A-Z0-9_]+$')
);

CREATE TABLE IF NOT EXISTS parametrizacion.parametrizacion_evaluacion_servicio (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    empresa_id BIGINT NOT NULL,
    servicio_id BIGINT NOT NULL,
    periodicidad_id BIGINT NOT NULL,
    activa BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_evaluacion_servicio_empresa_servicio UNIQUE (empresa_id, servicio_id),
    CONSTRAINT fk_evaluacion_servicio_empresa FOREIGN KEY (empresa_id) REFERENCES empresas.empresas_empresa (id) ON DELETE RESTRICT,
    CONSTRAINT fk_evaluacion_servicio_servicio FOREIGN KEY (servicio_id) REFERENCES catalogos.catalog_services (id) ON DELETE RESTRICT,
    CONSTRAINT fk_evaluacion_servicio_periodicidad FOREIGN KEY (periodicidad_id) REFERENCES catalogos.catalog_date_periodicities (id) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS parametrizacion.parametrizacion_evaluacion_personal (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    empresa_id BIGINT NOT NULL,
    periodicidad_id BIGINT NOT NULL,
    activa BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_evaluacion_personal_empresa UNIQUE (empresa_id),
    CONSTRAINT fk_evaluacion_personal_empresa FOREIGN KEY (empresa_id) REFERENCES empresas.empresas_empresa (id) ON DELETE RESTRICT,
    CONSTRAINT fk_evaluacion_personal_periodicidad FOREIGN KEY (periodicidad_id) REFERENCES catalogos.catalog_date_periodicities (id) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS parametrizacion.parametrizacion_dias_inactivos (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    empresa_id BIGINT NOT NULL,
    fecha DATE NOT NULL,
    descripcion VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_dias_inactivos_empresa_fecha UNIQUE (empresa_id, fecha),
    CONSTRAINT fk_dias_inactivos_empresa FOREIGN KEY (empresa_id) REFERENCES empresas.empresas_empresa (id) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS parametrizacion.parametrizacion_dias_inactivos_umbral (
    empresa_id BIGINT PRIMARY KEY,
    umbral_dias INTEGER NOT NULL DEFAULT 5 CHECK (umbral_dias >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_dias_inactivos_umbral_empresa FOREIGN KEY (empresa_id) REFERENCES empresas.empresas_empresa (id) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_eval_servicio_servicio_id
    ON parametrizacion.parametrizacion_evaluacion_servicio (servicio_id);

CREATE INDEX IF NOT EXISTS idx_eval_servicio_periodicidad_id
    ON parametrizacion.parametrizacion_evaluacion_servicio (periodicidad_id);

CREATE INDEX IF NOT EXISTS idx_eval_personal_periodicidad_id
    ON parametrizacion.parametrizacion_evaluacion_personal (periodicidad_id);

CREATE INDEX IF NOT EXISTS idx_dias_inactivos_empresa_fecha
    ON parametrizacion.parametrizacion_dias_inactivos (empresa_id, fecha);

CREATE TRIGGER trg_set_timestamp_sistema
    BEFORE UPDATE ON parametrizacion.parametrizacion_sistema
    FOR EACH ROW EXECUTE FUNCTION parametrizacion.fn_set_timestamp();

CREATE TRIGGER trg_set_timestamp_evaluacion_servicio
    BEFORE UPDATE ON parametrizacion.parametrizacion_evaluacion_servicio
    FOR EACH ROW EXECUTE FUNCTION parametrizacion.fn_set_timestamp();

CREATE TRIGGER trg_set_timestamp_evaluacion_personal
    BEFORE UPDATE ON parametrizacion.parametrizacion_evaluacion_personal
    FOR EACH ROW EXECUTE FUNCTION parametrizacion.fn_set_timestamp();

CREATE TRIGGER trg_set_timestamp_dias_inactivos
    BEFORE UPDATE ON parametrizacion.parametrizacion_dias_inactivos
    FOR EACH ROW EXECUTE FUNCTION parametrizacion.fn_set_timestamp();

CREATE TRIGGER trg_set_timestamp_dias_inactivos_umbral
    BEFORE UPDATE ON parametrizacion.parametrizacion_dias_inactivos_umbral
    FOR EACH ROW EXECUTE FUNCTION parametrizacion.fn_set_timestamp();
