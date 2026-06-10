package unit_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const atencionMigration = "V12__create_atencion_seguimiento_schema.sql"

func readAtencionMigration(t *testing.T) string {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join("..", "..", "db", "migrations", atencionMigration))
	require.NoError(t, err)
	return string(contents)
}

// TestAtencionSeguimientoMigrationDefinesSchema asserts the schema and all five
// tables exist in the migration file.
func TestAtencionSeguimientoMigrationDefinesSchema(t *testing.T) {
	sql := compactSQL(readAtencionMigration(t))

	assert.Contains(t, sql, "create schema if not exists atencion_seguimiento")

	for _, table := range []string{
		"atencion_seguimiento.solicitudes_queja",
		"atencion_seguimiento.respuestas_queja",
		"atencion_seguimiento.tickets_servicio",
		"atencion_seguimiento.respuestas_servicio",
		"atencion_seguimiento.incidencias_supervisor",
	} {
		t.Run(table, func(t *testing.T) {
			assert.Contains(t, sql, "create table if not exists "+table)
		})
	}
}

// TestAtencionSeguimientoMigrationEnforcesConstraints asserts CHECK constraints
// on estatus, ultima_resp, and rol_respuesta columns.
func TestAtencionSeguimientoMigrationEnforcesConstraints(t *testing.T) {
	sql := compactSQL(readAtencionMigration(t))

	for _, required := range []string{
		"constraint chk_solicitudes_queja_estatus check (estatus in (1, 2, 3))",
		"constraint chk_solicitudes_queja_ultima_resp check (ultima_resp in (1, 2, 3))",
		"constraint chk_respuestas_queja_rol check (rol_respuesta in (1, 2, 3))",
		"constraint chk_tickets_servicio_estatus check (estatus in (1, 2, 3))",
		"constraint chk_tickets_servicio_ultima_resp check (ultima_resp in (1, 2))",
		"constraint chk_respuestas_servicio_rol check (rol_respuesta in (1, 2))",
		"constraint fk_respuestas_queja_solicitud foreign key (solicitud_id)",
		"references atencion_seguimiento.solicitudes_queja(id) on delete cascade",
		"constraint fk_respuestas_servicio_ticket foreign key (ticket_id)",
		"references atencion_seguimiento.tickets_servicio(id) on delete cascade",
	} {
		t.Run(required, func(t *testing.T) {
			assert.Contains(t, sql, required)
		})
	}
}

// TestAtencionSeguimientoMigrationDefinesIndexes asserts the 12 lookup/partial
// indexes are present.
func TestAtencionSeguimientoMigrationDefinesIndexes(t *testing.T) {
	sql := compactSQL(readAtencionMigration(t))

	for _, required := range []string{
		"idx_solicitudes_queja_empresa",
		"idx_solicitudes_queja_empleado",
		"idx_solicitudes_queja_estatus",
		"idx_respuestas_queja_solicitud",
		"idx_respuestas_queja_unread",
		"idx_tickets_servicio_empresa",
		"idx_tickets_servicio_cliente",
		"idx_tickets_servicio_estatus",
		"idx_respuestas_servicio_ticket",
		"idx_respuestas_servicio_unread",
		"idx_incidencias_supervisor_empresa",
		"idx_incidencias_supervisor_empleado",
		"where leido = false",
	} {
		t.Run(required, func(t *testing.T) {
			assert.Contains(t, sql, required)
		})
	}
}

// TestAtencionSeguimientoMigrationDefinesTriggers asserts updated_at and SLA
// triggers are wired.
func TestAtencionSeguimientoMigrationDefinesTriggers(t *testing.T) {
	sql := compactSQL(readAtencionMigration(t))

	for _, required := range []string{
		"create or replace function atencion_seguimiento.fn_set_timestamp()",
		"create trigger trg_set_timestamp_solicitudes_queja",
		"create trigger trg_set_timestamp_tickets_servicio",
		"create trigger trg_set_timestamp_incidencias_supervisor",
		"create or replace function atencion_seguimiento.fn_calcular_fecha_vigencia()",
		"create trigger trg_calcular_fecha_vigencia",
		"before insert on atencion_seguimiento.solicitudes_queja",
	} {
		t.Run(required, func(t *testing.T) {
			assert.Contains(t, sql, required)
		})
	}
}
