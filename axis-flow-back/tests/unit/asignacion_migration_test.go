package unit_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const asignacionMigration = "V4__create_asignacion_schema.sql"

func readAsignacionMigration(t *testing.T) string {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join("..", "..", "db", "migrations", asignacionMigration))
	require.NoError(t, err)
	return string(contents)
}

func compactSQL(sql string) string {
	return strings.Join(strings.Fields(strings.ToLower(sql)), " ")
}

func TestAsignacionMigrationDefinesOwnedSchemaAndTables(t *testing.T) {
	sql := compactSQL(readAsignacionMigration(t))

	assert.Contains(t, sql, "create schema if not exists asignacion")
	for _, table := range []string{
		"asignacion.asignacion_asignacion",
		"asignacion.asignacion_asignaactividad",
		"asignacion.asignacion_asignaherramienta",
		"asignacion.asignacion_evaluacionempleado",
	} {
		t.Run(table, func(t *testing.T) {
			assert.Contains(t, sql, "create table if not exists "+table)
		})
	}
}

func TestAsignacionMigrationEnforcesAssignmentIntegrity(t *testing.T) {
	sql := compactSQL(readAsignacionMigration(t))

	for _, required := range []string{
		"empleado_id uuid not null",
		"localidad_id uuid not null",
		"servicio_id uuid not null",
		"turno_id uuid not null",
		"constraint chk_asignacion_estatus check (estatus in (1, 2))",
		"constraint chk_asignacion_fechas check (fecha_fin is null or fecha_fin >= fecha_inicio)",
		"where ultima_asignacion = true",
	} {
		t.Run(required, func(t *testing.T) {
			assert.Contains(t, sql, required)
		})
	}
}

func TestAsignacionMigrationEnforcesActivityToolAndEvaluationChecks(t *testing.T) {
	sql := compactSQL(readAsignacionMigration(t))

	for _, required := range []string{
		"constraint chk_asignaactividad_estatus check (estatus in (1, 2, 3))",
		"constraint chk_ubicacion_lat check (ubicacion_carga_lat is null or (ubicacion_carga_lat >= -90.0 and ubicacion_carga_lat <= 90.0))",
		"constraint chk_ubicacion_lon check (ubicacion_carga_lon is null or (ubicacion_carga_lon >= -180.0 and ubicacion_carga_lon <= 180.0))",
		"constraint chk_asignaherramienta_cantidad check (cantidad > 0)",
		"constraint chk_asignaherramienta_estatus check (estatus_entrega in (1, 2, 3))",
		"constraint chk_evaluacion_calificacion check (calificacion >= 1 and calificacion <= 5)",
	} {
		t.Run(required, func(t *testing.T) {
			assert.Contains(t, sql, required)
		})
	}
}

func TestAsignacionMigrationDefinesCurrentAssignmentExclusivityInvariant(t *testing.T) {
	sql := compactSQL(readAsignacionMigration(t))

	assert.Contains(t, sql, "create unique index if not exists uq_asignacion_empleado_ultima_asignacion")
	assert.Contains(t, sql, "on asignacion.asignacion_asignacion (empleado_id) where ultima_asignacion = true")
	assert.Contains(t, sql, "create or replace function asignacion.fn_asignacion_exclusividad()")
	assert.Contains(t, sql, "create trigger trg_asignacion_exclusividad")
	assert.Contains(t, sql, "before insert or update on asignacion.asignacion_asignacion")
	assert.Contains(t, sql, "when (new.ultima_asignacion = true)")
	assert.Contains(t, sql, "and id <> new.id")
	assert.Contains(t, sql, "set ultima_asignacion = false")
}

func TestAsignacionMigrationDefinesLookupIndexes(t *testing.T) {
	sql := compactSQL(readAsignacionMigration(t))

	for _, required := range []string{
		"create index if not exists idx_asignacion_empresa",
		"create index if not exists idx_asignaactividad_asignacion",
		"create index if not exists idx_asignaherramienta_asignacion",
		"create index if not exists idx_evaluacionempleado_asignacion",
	} {
		t.Run(required, func(t *testing.T) {
			assert.Contains(t, sql, required)
		})
	}
}
