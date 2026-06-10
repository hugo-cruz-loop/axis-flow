-- V4: Create Asignacion service schema and integrity rules.

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE SCHEMA IF NOT EXISTS asignacion;

CREATE TABLE IF NOT EXISTS asignacion.asignacion_asignacion (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    empresa_id UUID NOT NULL,
    empleado_id UUID NOT NULL,
    localidad_id UUID NOT NULL,
    servicio_id UUID NOT NULL,
    turno_id UUID NOT NULL,
    estatus INT NOT NULL DEFAULT 1 CONSTRAINT chk_asignacion_estatus CHECK (estatus IN (1, 2)),
    ultima_asignacion BOOLEAN NOT NULL DEFAULT TRUE,
    fecha_inicio DATE NOT NULL,
    fecha_fin DATE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_asignacion_fechas CHECK (fecha_fin IS NULL OR fecha_fin >= fecha_inicio)
);

CREATE TABLE IF NOT EXISTS asignacion.asignacion_asignaactividad (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    asignacion_id UUID NOT NULL REFERENCES asignacion.asignacion_asignacion(id) ON DELETE CASCADE,
    actividad_id UUID NOT NULL,
    descripcion VARCHAR(500) NOT NULL,
    frecuencia VARCHAR(100) NOT NULL,
    orden INT NOT NULL DEFAULT 0,
    estatus INT NOT NULL DEFAULT 1 CONSTRAINT chk_asignaactividad_estatus CHECK (estatus IN (1, 2, 3)),
    comentarios TEXT,
    evidencia_1 VARCHAR(2048),
    evidencia_2 VARCHAR(2048),
    evidencia_3 VARCHAR(2048),
    ubicacion_carga_lat NUMERIC(9,6),
    ubicacion_carga_lon NUMERIC(9,6),
    fecha_ejecucion TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_ubicacion_lat CHECK (ubicacion_carga_lat IS NULL OR (ubicacion_carga_lat >= -90.0 AND ubicacion_carga_lat <= 90.0)),
    CONSTRAINT chk_ubicacion_lon CHECK (ubicacion_carga_lon IS NULL OR (ubicacion_carga_lon >= -180.0 AND ubicacion_carga_lon <= 180.0))
);

CREATE TABLE IF NOT EXISTS asignacion.asignacion_asignaherramienta (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    asignacion_id UUID NOT NULL REFERENCES asignacion.asignacion_asignacion(id) ON DELETE CASCADE,
    herramienta_id UUID NOT NULL,
    nombre VARCHAR(255) NOT NULL,
    cantidad INT NOT NULL DEFAULT 1 CONSTRAINT chk_asignaherramienta_cantidad CHECK (cantidad > 0),
    especificaciones TEXT,
    estatus_entrega INT NOT NULL DEFAULT 1 CONSTRAINT chk_asignaherramienta_estatus CHECK (estatus_entrega IN (1, 2, 3)),
    fecha_entrega TIMESTAMPTZ,
    fecha_devolucion TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS asignacion.asignacion_evaluacionempleado (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    asignacion_id UUID NOT NULL REFERENCES asignacion.asignacion_asignacion(id) ON DELETE CASCADE,
    empleado_id UUID NOT NULL,
    evaluador_id UUID NOT NULL,
    calificacion INT NOT NULL CONSTRAINT chk_evaluacion_calificacion CHECK (calificacion >= 1 AND calificacion <= 5),
    cumple_actividades BOOLEAN NOT NULL DEFAULT TRUE,
    comentarios TEXT,
    fecha_evaluacion DATE NOT NULL DEFAULT CURRENT_DATE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE OR REPLACE FUNCTION asignacion.fn_asignacion_exclusividad()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE asignacion.asignacion_asignacion
    SET ultima_asignacion = FALSE,
        updated_at = NOW()
    WHERE empleado_id = NEW.empleado_id
      AND id <> NEW.id
      AND ultima_asignacion = TRUE;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_asignacion_exclusividad ON asignacion.asignacion_asignacion;
CREATE TRIGGER trg_asignacion_exclusividad
BEFORE INSERT OR UPDATE ON asignacion.asignacion_asignacion
FOR EACH ROW
WHEN (NEW.ultima_asignacion = TRUE)
EXECUTE FUNCTION asignacion.fn_asignacion_exclusividad();

CREATE UNIQUE INDEX IF NOT EXISTS uq_asignacion_empleado_ultima_asignacion
ON asignacion.asignacion_asignacion (empleado_id)
WHERE ultima_asignacion = TRUE;

CREATE INDEX IF NOT EXISTS idx_asignacion_empresa
ON asignacion.asignacion_asignacion (empresa_id);

CREATE INDEX IF NOT EXISTS idx_asignaactividad_asignacion
ON asignacion.asignacion_asignaactividad (asignacion_id);

CREATE INDEX IF NOT EXISTS idx_asignaherramienta_asignacion
ON asignacion.asignacion_asignaherramienta (asignacion_id);

CREATE INDEX IF NOT EXISTS idx_evaluacionempleado_asignacion
ON asignacion.asignacion_evaluacionempleado (asignacion_id);
