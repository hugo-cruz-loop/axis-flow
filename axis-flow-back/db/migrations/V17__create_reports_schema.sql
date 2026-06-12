-- V17: Create reports schema with geocoding cache table.
-- Rollback (run in reverse order):
--   DROP TRIGGER IF EXISTS tg_reports_direcciones_updated_at ON reports.reports_direcciones;
--   DROP FUNCTION IF EXISTS reports.fn_set_updated_at();
--   DROP INDEX IF EXISTS idx_reports_direcciones_coords;
--   DROP TABLE IF EXISTS reports.reports_direcciones;
--   DROP SCHEMA IF EXISTS reports;

CREATE SCHEMA IF NOT EXISTS reports;

CREATE TABLE reports.reports_direcciones (
    latitud   NUMERIC(9,6) NOT NULL,
    longitud  NUMERIC(9,6) NOT NULL,
    direccion VARCHAR(512) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT pk_reports_direcciones PRIMARY KEY (latitud, longitud)
);

CREATE INDEX idx_reports_direcciones_coords
    ON reports.reports_direcciones (latitud, longitud);

CREATE OR REPLACE FUNCTION reports.fn_set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER tg_reports_direcciones_updated_at
BEFORE UPDATE ON reports.reports_direcciones
FOR EACH ROW EXECUTE FUNCTION reports.fn_set_updated_at();
