package unit_test

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestV10Migration_FileExists asserts that migration V10 file is present.
func TestV10Migration_FileExists(t *testing.T) {
	content, err := os.ReadFile("../../db/migrations/V10__create_bolsa_trabajo_schema.sql")
	require.NoError(t, err, "V10 migration file must exist")

	sql := string(content)

	// Schema
	assert.Contains(t, sql, "CREATE SCHEMA IF NOT EXISTS bolsa_trabajo")

	// Tables
	assert.Contains(t, sql, "bolsa_trabajo.trabajos")
	assert.Contains(t, sql, "bolsa_trabajo.postulaciones")
	assert.Contains(t, sql, "bolsa_trabajo.evaluaciones")

	// Indexes (5 total)
	assert.Contains(t, sql, "idx_trabajos_empresa")
	assert.Contains(t, sql, "idx_trabajos_estatus_caducar")
	assert.Contains(t, sql, "idx_postulaciones_trabajo")
	assert.Contains(t, sql, "idx_postulaciones_estatus")
	assert.Contains(t, sql, "idx_evaluaciones_postulacion")

	// CHECK constraints
	assert.Contains(t, sql, "estatus_vacante IN (1, 2, 3)")
	assert.Contains(t, sql, "estatus IN (1, 2, 3, 4, 5)")

	// FK constraint
	assert.Contains(t, sql, "REFERENCES bolsa_trabajo.trabajos(id)")

	// gen_random_uuid default
	assert.True(t, strings.Contains(sql, "gen_random_uuid()"))
}
