CREATE SCHEMA IF NOT EXISTS atencion_seguimiento;

-- Timestamp trigger function
CREATE OR REPLACE FUNCTION atencion_seguimiento.fn_set_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Table: solicitudes_queja (employee labor complaints)
CREATE TABLE IF NOT EXISTS atencion_seguimiento.solicitudes_queja (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    empresa_id UUID NOT NULL,
    empleado_id BIGINT NOT NULL,
    tipo_queja_id UUID NOT NULL,
    titulo VARCHAR(255) NOT NULL,
    descripcion TEXT NOT NULL,
    estatus INT NOT NULL DEFAULT 1,
    fecha_vigencia TIMESTAMPTZ,
    ultima_resp INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_solicitudes_queja_estatus CHECK (estatus IN (1, 2, 3)),
    CONSTRAINT chk_solicitudes_queja_ultima_resp CHECK (ultima_resp IN (1, 2, 3))
);

-- Table: respuestas_queja (complaint chat messages)
CREATE TABLE IF NOT EXISTS atencion_seguimiento.respuestas_queja (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    solicitud_id UUID NOT NULL,
    remitente_id UUID NOT NULL,
    rol_respuesta INT NOT NULL,
    mensaje TEXT NOT NULL,
    archivo_adjunto_url VARCHAR(512),
    leido BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_respuestas_queja_solicitud FOREIGN KEY (solicitud_id)
        REFERENCES atencion_seguimiento.solicitudes_queja(id) ON DELETE CASCADE,
    CONSTRAINT chk_respuestas_queja_rol CHECK (rol_respuesta IN (1, 2, 3))
);

-- Table: tickets_servicio (client service tickets)
CREATE TABLE IF NOT EXISTS atencion_seguimiento.tickets_servicio (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    empresa_id UUID NOT NULL,
    cliente_id UUID NOT NULL,
    localidad_id UUID NOT NULL,
    asunto VARCHAR(255) NOT NULL,
    descripcion TEXT NOT NULL,
    estatus INT NOT NULL DEFAULT 1,
    ultima_resp INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_tickets_servicio_estatus CHECK (estatus IN (1, 2, 3)),
    CONSTRAINT chk_tickets_servicio_ultima_resp CHECK (ultima_resp IN (1, 2))
);

-- Table: respuestas_servicio (service ticket chat messages)
CREATE TABLE IF NOT EXISTS atencion_seguimiento.respuestas_servicio (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ticket_id UUID NOT NULL,
    remitente_id UUID NOT NULL,
    rol_respuesta INT NOT NULL,
    mensaje TEXT NOT NULL,
    archivo_adjunto_url VARCHAR(512),
    leido BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_respuestas_servicio_ticket FOREIGN KEY (ticket_id)
        REFERENCES atencion_seguimiento.tickets_servicio(id) ON DELETE CASCADE,
    CONSTRAINT chk_respuestas_servicio_rol CHECK (rol_respuesta IN (1, 2))
);

-- Table: incidencias_supervisor (supervisor incident reports)
CREATE TABLE IF NOT EXISTS atencion_seguimiento.incidencias_supervisor (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    empresa_id UUID NOT NULL,
    supervisor_id UUID NOT NULL,
    empleado_id BIGINT NOT NULL,
    localidad_id UUID NOT NULL,
    tipo_incidencia_id UUID NOT NULL,
    descripcion TEXT NOT NULL,
    sancion_sugerida TEXT,
    evidencia_url VARCHAR(512),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_solicitudes_queja_empresa ON atencion_seguimiento.solicitudes_queja (empresa_id);
CREATE INDEX IF NOT EXISTS idx_solicitudes_queja_empleado ON atencion_seguimiento.solicitudes_queja (empleado_id);
CREATE INDEX IF NOT EXISTS idx_solicitudes_queja_estatus ON atencion_seguimiento.solicitudes_queja (estatus);
CREATE INDEX IF NOT EXISTS idx_respuestas_queja_solicitud ON atencion_seguimiento.respuestas_queja (solicitud_id);
CREATE INDEX IF NOT EXISTS idx_respuestas_queja_unread ON atencion_seguimiento.respuestas_queja (solicitud_id) WHERE leido = FALSE;
CREATE INDEX IF NOT EXISTS idx_tickets_servicio_empresa ON atencion_seguimiento.tickets_servicio (empresa_id);
CREATE INDEX IF NOT EXISTS idx_tickets_servicio_cliente ON atencion_seguimiento.tickets_servicio (cliente_id);
CREATE INDEX IF NOT EXISTS idx_tickets_servicio_estatus ON atencion_seguimiento.tickets_servicio (estatus);
CREATE INDEX IF NOT EXISTS idx_respuestas_servicio_ticket ON atencion_seguimiento.respuestas_servicio (ticket_id);
CREATE INDEX IF NOT EXISTS idx_respuestas_servicio_unread ON atencion_seguimiento.respuestas_servicio (ticket_id) WHERE leido = FALSE;
CREATE INDEX IF NOT EXISTS idx_incidencias_supervisor_empresa ON atencion_seguimiento.incidencias_supervisor (empresa_id);
CREATE INDEX IF NOT EXISTS idx_incidencias_supervisor_empleado ON atencion_seguimiento.incidencias_supervisor (empleado_id);

-- Updated_at triggers
CREATE TRIGGER trg_set_timestamp_solicitudes_queja
    BEFORE UPDATE ON atencion_seguimiento.solicitudes_queja
    FOR EACH ROW EXECUTE FUNCTION atencion_seguimiento.fn_set_timestamp();

CREATE TRIGGER trg_set_timestamp_tickets_servicio
    BEFORE UPDATE ON atencion_seguimiento.tickets_servicio
    FOR EACH ROW EXECUTE FUNCTION atencion_seguimiento.fn_set_timestamp();

CREATE TRIGGER trg_set_timestamp_incidencias_supervisor
    BEFORE UPDATE ON atencion_seguimiento.incidencias_supervisor
    FOR EACH ROW EXECUTE FUNCTION atencion_seguimiento.fn_set_timestamp();

-- SLA auto-calculation trigger
CREATE OR REPLACE FUNCTION atencion_seguimiento.fn_calcular_fecha_vigencia()
RETURNS TRIGGER AS $$
DECLARE
    v_sla_days INT := 5;
BEGIN
    IF NEW.fecha_vigencia IS NULL THEN
        BEGIN
            SELECT COALESCE(dias_vigencia, 5) INTO v_sla_days
            FROM parametrizacion.buzon_quejas
            WHERE empresa_id = NEW.empresa_id;
        EXCEPTION
            WHEN OTHERS THEN
                BEGIN
                    SELECT COALESCE(dias_sla, 5) INTO v_sla_days
                    FROM catalogos.catalog_complaint_types
                    WHERE id::text = NEW.tipo_queja_id::text;
                EXCEPTION
                    WHEN OTHERS THEN
                        v_sla_days := 5;
                END;
        END;
        NEW.fecha_vigencia := CURRENT_TIMESTAMP + (v_sla_days || ' days')::INTERVAL;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_calcular_fecha_vigencia
    BEFORE INSERT ON atencion_seguimiento.solicitudes_queja
    FOR EACH ROW EXECUTE FUNCTION atencion_seguimiento.fn_calcular_fecha_vigencia();
