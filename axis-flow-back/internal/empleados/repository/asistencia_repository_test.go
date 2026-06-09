package repository

import (
	"context"
	"reflect"
	"testing"
	"time"

	empleados "axis-flow-back/internal/empleados"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func asistenciaRow(a empleados.Asistencia) rowResult {
	return rowResult{values: []any{a.ID, a.EmpleadoID, a.Geolocalizacion, a.EstatusRango, a.FotoEntradaURL, a.EstatusObservacionEntrada, a.SimilitudFacial, a.TipoRegistro, a.Fecha, a.HoraEntrada, a.HoraSalida, a.CreatedAt, a.UpdatedAt}}
}

func TestAsistenciaRepositoryCreateDefaultsToPending(t *testing.T) {
	id := uuid.New()
	today := time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC)
	now := time.Now()
	db := &fakeDB{queryRows: []rowResult{{values: []any{id, empleados.ObservacionPendiente, today, now, now}}}}
	repo := NewAsistenciaRepository(db)
	a := &empleados.Asistencia{EmpleadoID: 105, Geolocalizacion: "19.2891,-99.6534", EstatusRango: empleados.RangoEnRango, FotoEntradaURL: "minio://foto.jpg", TipoRegistro: empleados.TipoEntradaLaboral, HoraEntrada: "14:03:46"}

	err := repo.Create(context.Background(), a, 12)

	require.NoError(t, err)
	assert.Equal(t, empleados.ObservacionPendiente, a.EstatusObservacionEntrada)
	assert.Equal(t, id, a.ID)
	assert.True(t, assertSQLContains(db.calls[0].sql, "empresa_id=$"), db.calls[0].sql)
	assert.Contains(t, db.calls[0].args, empleados.ObservacionPendiente)
}

func TestAsistenciaRepositoryGetByEmpleadoAndEmpresaAreScoped(t *testing.T) {
	now := time.Now()
	id := uuid.New()
	db := &fakeDB{queryResults: []*fakeRows{
		{rows: []rowResult{asistenciaRow(empleados.Asistencia{ID: id, EmpleadoID: 105, Geolocalizacion: "geo", EstatusRango: 1, FotoEntradaURL: "url", EstatusObservacionEntrada: 1, TipoRegistro: empleados.TipoEntradaLaboral, Fecha: now, HoraEntrada: "08:00:00", CreatedAt: now, UpdatedAt: now})}},
		{rows: []rowResult{asistenciaRow(empleados.Asistencia{ID: uuid.New(), EmpleadoID: 106, Geolocalizacion: "geo2", EstatusRango: 2, FotoEntradaURL: "url2", EstatusObservacionEntrada: 1, TipoRegistro: empleados.TipoSalidaLaboral, Fecha: now, HoraEntrada: "18:00:00", CreatedAt: now, UpdatedAt: now})}},
	}}
	repo := NewAsistenciaRepository(db)

	byEmpleado, err := repo.GetByEmpleado(context.Background(), 105, 12)
	require.NoError(t, err)
	assert.Len(t, byEmpleado, 1)
	assert.True(t, assertSQLContains(db.calls[0].sql, "empleado_id=$1"), db.calls[0].sql)
	assert.True(t, assertSQLContains(db.calls[0].sql, "empresa_id=$2"), db.calls[0].sql)

	byEmpresa, err := repo.GetByEmpresa(context.Background(), 12)
	require.NoError(t, err)
	assert.Len(t, byEmpresa, 1)
	assert.True(t, assertSQLContains(db.calls[1].sql, "empresa_id=$1"), db.calls[1].sql)
}

func TestAsistenciaRepositoryHasNoDeleteMethod(t *testing.T) {
	repoType := reflect.TypeOf(NewAsistenciaRepository(&fakeDB{}))
	_, ok := repoType.MethodByName("Delete")
	assert.False(t, ok, "asistencias are immutable and repository must not expose Delete")
}
