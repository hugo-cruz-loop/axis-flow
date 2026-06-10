-- V13: Create Formularios service schema and integrity rules.
--
-- Mirrors V12 (atencion_seguimiento) conventions:
--   * Schema name 'formularios' (ownership boundary per security spec).
--   * All tables use UUID PKs with gen_random_uuid() default.
--   * Tables with updated_at get a BEFORE UPDATE trigger wired to
--     formularios.fn_set_timestamp() (this file, below).
--   * FKs with ON DELETE CASCADE for parent-owned child rows
--     (pregunta/evento_formulario/iniciado/respuesta), per spec DDL.
--   * CHECK constraints on enum-like INT or status columns.
--   * Idempotent CREATE IF NOT EXISTS / CREATE OR REPLACE so this migration
--     can be re-applied in dev/CI without dropping the schema.
--
-- Six tables, per specification.md §Datos (Proposed DDL):
--   1. formularios_formulario          (form header)
--   2. formularios_pregunta             (questions belonging to a form)
--   3. eventos_evento                  (operational event)
--   4. eventos_evento_formulario       (M:N event <-> form association)
--   5. eventos_evento_iniciado         (employee check-in)
--   6. formularios_respuestas          (field answers to a question)

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE SCHEMA IF NOT EXISTS formularios;

-- ---------------------------------------------------------------------------
-- Updated-at trigger function (task 1.3). Mirrors atencion_seguimiento.fn_set_timestamp.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION formularios.fn_set_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- ---------------------------------------------------------------------------
-- 1. formularios_formulario — form template header
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS formularios.formularios_formulario (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    empresa_id UUID NOT NULL,
    nombre VARCHAR(150) NOT NULL,
    descripcion TEXT,
    activo BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- ---------------------------------------------------------------------------
-- 2. formularios_pregunta — questions belonging to a form
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS formularios.formularios_pregunta (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    formulario_id UUID NOT NULL REFERENCES formularios.formularios_formulario(id) ON DELETE CASCADE,
    orden INT NOT NULL,
    tipo_pregunta INT NOT NULL, -- 1=ShortText, 2=Checkbox, 3=Rating, 5=Matrix, 8=Camera, 11=Signature
    texto_pregunta TEXT NOT NULL,
    obligatoria BOOLEAN NOT NULL DEFAULT FALSE,
    respuesta_predefinida JSONB, -- Matrix rows/columns, predefined choices, etc.
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_formularios_pregunta_tipo CHECK (tipo_pregunta IN (1, 2, 3, 5, 8, 11))
);

-- ---------------------------------------------------------------------------
-- 3. eventos_evento — operational event
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS formularios.eventos_evento (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    empresa_id UUID NOT NULL,
    cliente_id UUID NOT NULL,
    localidad_id UUID NOT NULL,
    nombre VARCHAR(150) NOT NULL,
    descripcion TEXT,
    fecha_programada TIMESTAMPTZ NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pendiente',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_eventos_evento_status CHECK (status IN ('pendiente', 'iniciado', 'completado', 'cancelado'))
);

-- ---------------------------------------------------------------------------
-- 4. eventos_evento_formulario — M:N association between events and forms
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS formularios.eventos_evento_formulario (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    evento_id UUID NOT NULL REFERENCES formularios.eventos_evento(id) ON DELETE CASCADE,
    formulario_id UUID NOT NULL REFERENCES formularios.formularios_formulario(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_eventos_evento_formulario_evento_formulario UNIQUE (evento_id, formulario_id)
);

-- ---------------------------------------------------------------------------
-- 5. eventos_evento_iniciado — employee check-in (form execution start)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS formularios.eventos_evento_iniciado (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    evento_id UUID NOT NULL REFERENCES formularios.eventos_evento(id) ON DELETE CASCADE,
    empleado_id BIGINT NOT NULL,
    geolocalizacion_inicio_lat NUMERIC(9,6),
    geolocalizacion_inicio_lon NUMERIC(9,6),
    check_in_time TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    status VARCHAR(50) NOT NULL DEFAULT 'iniciado',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_eventos_evento_iniciado_status CHECK (status IN ('iniciado', 'completado', 'cancelado')),
    CONSTRAINT chk_eventos_evento_iniciado_geo_lat CHECK (geolocalizacion_inicio_lat IS NULL OR (geolocalizacion_inicio_lat >= -90.0 AND geolocalizacion_inicio_lat <= 90.0)),
    CONSTRAINT chk_eventos_evento_iniciado_geo_lon CHECK (geolocalizacion_inicio_lon IS NULL OR (geolocalizacion_inicio_lon >= -180.0 AND geolocalizacion_inicio_lon <= 180.0))
);

-- ---------------------------------------------------------------------------
-- 6. formularios_respuestas — answers to a question for a check-in
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS formularios.formularios_respuestas (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    evento_iniciado_id UUID NOT NULL REFERENCES formularios.eventos_evento_iniciado(id) ON DELETE CASCADE,
    pregunta_id UUID NOT NULL REFERENCES formularios.formularios_pregunta(id) ON DELETE CASCADE,
    respuesta_texto TEXT,
    respuesta_lista JSONB,
    evidencia1 VARCHAR(512),
    evidencia2 VARCHAR(512),
    evidencia3 VARCHAR(512),
    documento_url VARCHAR(512),
    geolocalizacion_respuesta_lat NUMERIC(9,6),
    geolocalizacion_respuesta_lon NUMERIC(9,6),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_formularios_respuestas_geo_lat CHECK (geolocalizacion_respuesta_lat IS NULL OR (geolocalizacion_respuesta_lat >= -90.0 AND geolocalizacion_respuesta_lat <= 90.0)),
    CONSTRAINT chk_formularios_respuestas_geo_lon CHECK (geolocalizacion_respuesta_lon IS NULL OR (geolocalizacion_respuesta_lon >= -180.0 AND geolocalizacion_respuesta_lon <= 180.0))
);

-- ---------------------------------------------------------------------------
-- Indexes (task 1.2: at least 3; we cover the spec's lookup patterns).
-- ---------------------------------------------------------------------------

-- Required by task 1.2 + tests: empresa_id lookup for ListByEmpresa.
CREATE INDEX IF NOT EXISTS idx_formularios_formulario_empresa
    ON formularios.formularios_formulario (empresa_id);

-- Required by task 1.2 + tests: FK column for ListByFormulario.
CREATE INDEX IF NOT EXISTS idx_formularios_pregunta_formulario
    ON formularios.formularios_pregunta (formulario_id);

-- Required by task 1.2 + tests: FK column for ListByIniciado.
CREATE INDEX IF NOT EXISTS idx_formularios_respuestas_iniciado
    ON formularios.formularios_respuestas (evento_iniciado_id);

-- Additional spec-driven lookups beyond the 3 minimum.
CREATE INDEX IF NOT EXISTS idx_eventos_evento_empresa
    ON formularios.eventos_evento (empresa_id);

CREATE INDEX IF NOT EXISTS idx_eventos_evento_cliente
    ON formularios.eventos_evento (cliente_id);

CREATE INDEX IF NOT EXISTS idx_eventos_evento_status
    ON formularios.eventos_evento (status);

CREATE INDEX IF NOT EXISTS idx_eventos_evento_formulario_evento
    ON formularios.eventos_evento_formulario (evento_id);

CREATE INDEX IF NOT EXISTS idx_eventos_evento_iniciado_evento
    ON formularios.eventos_evento_iniciado (evento_id);

CREATE INDEX IF NOT EXISTS idx_eventos_evento_iniciado_empleado
    ON formularios.eventos_evento_iniciado (empleado_id);

-- ---------------------------------------------------------------------------
-- Updated-at triggers on the three tables that own an updated_at column.
-- ---------------------------------------------------------------------------
CREATE TRIGGER trg_set_timestamp_formularios_formulario
    BEFORE UPDATE ON formularios.formularios_formulario
    FOR EACH ROW EXECUTE FUNCTION formularios.fn_set_timestamp();

CREATE TRIGGER trg_set_timestamp_formularios_pregunta
    BEFORE UPDATE ON formularios.formularios_pregunta
    FOR EACH ROW EXECUTE FUNCTION formularios.fn_set_timestamp();

CREATE TRIGGER trg_set_timestamp_formularios_respuestas
    BEFORE UPDATE ON formularios.formularios_respuestas
    FOR EACH ROW EXECUTE FUNCTION formularios.fn_set_timestamp();
