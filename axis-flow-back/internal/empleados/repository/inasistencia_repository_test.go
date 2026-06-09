package repository

import (
	"context"
	"testing"
	"time"

	empleados "axis-flow-back/internal/empleados"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func inasistenciaRow(i empleados.Inasistencia) rowResult {
	return rowResult{values: []any{i.ID, i.EmpleadoID, i.TipoIncidencia, i.FechaInicio, i.FechaFin, i.JustificanteURL, i.Aprobado, i.Observaciones, i.ResolvedBy, i.ResolvedAt, i.CreatedAt, i.UpdatedAt}}
}

func TestInasistenciaRepositoryCreateAndGetByEmpleado(t *testing.T) {
	id := uuid.New()
	start := time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 0, 1)
	now := time.Now()
	justificante := "minio://justificante.pdf"
	db := &fakeDB{queryRows: []rowResult{{values: []any{id, false, now, now}}}, queryResults: []*fakeRows{{rows: []rowResult{inasistenciaRow(empleados.Inasistencia{ID: id, EmpleadoID: 105, TipoIncidencia: "INCAPACIDAD", FechaInicio: start, FechaFin: end, JustificanteURL: &justificante, Aprobado: false, CreatedAt: now, UpdatedAt: now})}}}}
	repo := NewInasistenciaRepository(db)
	incidence := &empleados.Inasistencia{EmpleadoID: 105, TipoIncidencia: "INCAPACIDAD", FechaInicio: start, FechaFin: end, JustificanteURL: &justificante}

	err := repo.Create(context.Background(), incidence, 12)
	require.NoError(t, err)
	assert.Equal(t, id, incidence.ID)
	assert.False(t, incidence.Aprobado)
	assert.True(t, assertSQLContains(db.calls[0].sql, "empresa_id=$"), db.calls[0].sql)

	got, err := repo.GetByEmpleado(context.Background(), 105, 12)
	require.NoError(t, err)
	assert.Len(t, got, 1)
	assert.True(t, assertSQLContains(db.calls[1].sql, "empleado_id=$1"), db.calls[1].sql)
	assert.True(t, assertSQLContains(db.calls[1].sql, "empresa_id=$2"), db.calls[1].sql)
}

func TestInasistenciaRepositoryUpdateStatusIsEmpresaScoped(t *testing.T) {
	resolver := uuid.New()
	obs := "approved by RH"
	db := &fakeDB{execAffected: []int64{1}}
	repo := NewInasistenciaRepository(db)

	err := repo.UpdateStatus(context.Background(), uuid.New(), 12, true, &obs, resolver)

	require.NoError(t, err)
	assert.True(t, assertSQLContains(db.calls[0].sql, "empresa_id=$"), db.calls[0].sql)
	assert.Contains(t, db.calls[0].args, true)
	assert.Contains(t, db.calls[0].args, resolver)
}
