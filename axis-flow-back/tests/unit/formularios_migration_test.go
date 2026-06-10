package unit_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const formulariosMigration = "V13__create_formularios_schema.sql"

func readFormulariosMigration(t *testing.T) string {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join("..", "..", "db", "migrations", formulariosMigration))
	require.NoError(t, err)
	return string(contents)
}

// TestFormulariosMigrationDefinesSchema asserts the formularios schema and
// the six tables required by the service specification are created.
func TestFormulariosMigrationDefinesSchema(t *testing.T) {
	sql := compactSQL(readFormulariosMigration(t))

	assert.Contains(t, sql, "create schema if not exists formularios")

	for _, table := range []string{
		"formularios.formularios_formulario",
		"formularios.formularios_pregunta",
		"formularios.eventos_evento",
		"formularios.eventos_evento_formulario",
		"formularios.eventos_evento_iniciado",
		"formularios.formularios_respuestas",
	} {
		t.Run(table, func(t *testing.T) {
			assert.Contains(t, sql, "create table if not exists "+table)
		})
	}
}

// TestFormulariosMigrationEnforcesConstraints asserts the foreign-key and
// CHECK constraints that protect referential integrity and enum-like columns.
func TestFormulariosMigrationEnforcesConstraints(t *testing.T) {
	sql := compactSQL(readFormulariosMigration(t))

	for _, required := range []string{
		// FKs from the spec DDL with ON DELETE CASCADE on parent-owned children.
		"references formularios.formularios_formulario(id) on delete cascade",
		"references formularios.eventos_evento(id) on delete cascade",
		"references formularios.eventos_evento_iniciado(id) on delete cascade",
		"references formularios.formularios_pregunta(id) on delete cascade",
		// CHECK on the event status column.
		"constraint chk_eventos_evento_status check (status in ('pendiente', 'iniciado', 'completado', 'cancelado'))",
		// CHECK on the question type column (1=ShortText, 5=Matrix, 8=Camera, 11=Signature).
		"constraint chk_formularios_pregunta_tipo check (tipo_pregunta in (1, 2, 3, 5, 8, 11))",
		// CHECK on the iniciado status column.
		"constraint chk_eventos_evento_iniciado_status check (status in ('iniciado', 'completado', 'cancelado'))",
	} {
		t.Run(required, func(t *testing.T) {
			assert.Contains(t, sql, required)
		})
	}
}

// TestFormulariosMigrationDefinesLookupIndexes asserts the three minimum
// indexes required by task 1.2 cover empresa_id, FK list columns, and
// status columns used for filtering.
func TestFormulariosMigrationDefinesLookupIndexes(t *testing.T) {
	sql := compactSQL(readFormulariosMigration(t))

	for _, required := range []string{
		// empresa_id lookup for the ListByEmpresa API endpoint.
		"create index if not exists idx_formularios_formulario_empresa",
		"on formularios.formularios_formulario (empresa_id)",
		// FK column for ListByFormulario (questions grouped under a form).
		"create index if not exists idx_formularios_pregunta_formulario",
		"on formularios.formularios_pregunta (formulario_id)",
		// FK column for ListByIniciado (responses grouped under a check-in).
		"create index if not exists idx_formularios_respuestas_iniciado",
		"on formularios.formularios_respuestas (evento_iniciado_id)",
	} {
		t.Run(required, func(t *testing.T) {
			assert.Contains(t, sql, required)
		})
	}
}

// TestFormulariosMigrationDefinesUpdatedAtTriggers asserts the shared
// formularios.fn_set_timestamp() trigger function and the BEFORE UPDATE
// triggers wired to every table that owns an updated_at column.
func TestFormulariosMigrationDefinesUpdatedAtTriggers(t *testing.T) {
	sql := compactSQL(readFormulariosMigration(t))

	// The trigger function itself.
	assert.Contains(t, sql, "create or replace function formularios.fn_set_timestamp()")
	assert.Contains(t, sql, "new.updated_at = current_timestamp")

	// One BEFORE UPDATE trigger per table that carries updated_at.
	for _, required := range []string{
		"create trigger trg_set_timestamp_formularios_formulario",
		"before update on formularios.formularios_formulario",
		"create trigger trg_set_timestamp_formularios_pregunta",
		"before update on formularios.formularios_pregunta",
		"create trigger trg_set_timestamp_formularios_respuestas",
		"before update on formularios.formularios_respuestas",
	} {
		t.Run(required, func(t *testing.T) {
			assert.Contains(t, sql, required)
		})
	}
}
