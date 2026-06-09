package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	empleados "axis-flow-back/internal/empleados"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpedienteRepositoryUbicacionGetAndUpsertAreEmpresaScoped(t *testing.T) {
	now := time.Now()
	db := &fakeDB{queryRows: []rowResult{{values: []any{int64(105), "CURP12345678901234", "12345678901", "Main", "10", "2B", "Centro", "01000", int64(1), int64(2), int64(3), now, now}}}, execAffected: []int64{1}}
	repo := NewExpedienteRepository(db)

	got, err := repo.GetUbicacion(context.Background(), 105, 12)
	require.NoError(t, err)
	assert.Equal(t, "CURP12345678901234", got.CURP)
	assert.True(t, assertSQLContains(db.calls[0].sql, "empresa_id=$2"), db.calls[0].sql)

	err = repo.UpsertUbicacion(context.Background(), &empleados.Ubicacion{EmpleadoID: 105, CURP: "CURP12345678901234", NSS: "12345678901", Calle: "Main", NumeroExterior: "10", Colonia: "Centro", CodigoPostal: "01000", CiudadID: 1, EstadoID: 2, PaisID: 3}, 12)
	require.NoError(t, err)
	assert.True(t, assertSQLContains(db.calls[1].sql, "empresa_id=$"), db.calls[1].sql)
	assert.True(t, assertSQLContains(db.calls[1].sql, "on conflict (empleado_id) do update"), db.calls[1].sql)
}

func TestExpedienteRepositorySurfacesCURPAndNSSUniqueViolations(t *testing.T) {
	repoCURP := NewExpedienteRepository(&fakeDB{execErrs: []error{&pgconn.PgError{Code: "23505", ConstraintName: "uq_empleados_ubicacion_curp"}}})
	err := repoCURP.UpsertUbicacion(context.Background(), &empleados.Ubicacion{EmpleadoID: 105, CURP: "duplicate", NSS: "12345678901"}, 12)
	assert.ErrorIs(t, err, empleados.ErrDuplicateCURP)

	repoNSS := NewExpedienteRepository(&fakeDB{execErrs: []error{&pgconn.PgError{Code: "23505", ConstraintName: "uq_empleados_ubicacion_nss"}}})
	err = repoNSS.UpsertUbicacion(context.Background(), &empleados.Ubicacion{EmpleadoID: 105, CURP: "CURP", NSS: "duplicate"}, 12)
	assert.ErrorIs(t, err, empleados.ErrDuplicateNSS)
}

func TestExpedienteRepositoryAdicionalesGetAndUpsert(t *testing.T) {
	now := time.Now()
	beneficiarios := []byte(`[{"nombre":"Ana","porcentaje":100}]`)
	db := &fakeDB{queryRows: []rowResult{{values: []any{int64(105), "Ana", "555", "Hermana", beneficiarios, now, now}}}, execAffected: []int64{1}}
	repo := NewExpedienteRepository(db)

	got, err := repo.GetAdicionales(context.Background(), 105, 12)
	require.NoError(t, err)
	assert.Equal(t, "Ana", got.ContactoEmergenciaNombre)
	assert.Len(t, got.Beneficiarios, 1)
	assert.True(t, assertSQLContains(db.calls[0].sql, "empresa_id=$2"), db.calls[0].sql)

	err = repo.UpsertAdicionales(context.Background(), &empleados.Adicionales{EmpleadoID: 105, ContactoEmergenciaNombre: "Ana", ContactoEmergenciaTelefono: "555", ContactoEmergenciaParentesco: "Hermana", Beneficiarios: []map[string]any{{"nombre": "Ana", "porcentaje": float64(100)}}}, 12)
	require.NoError(t, err)
	assert.True(t, assertSQLContains(db.calls[1].sql, "on conflict (empleado_id) do update"), db.calls[1].sql)
}

func TestExpedienteRepositoryDocumentosGetAndUpsert(t *testing.T) {
	now := time.Now()
	acta := "minio://docs/acta.pdf"
	ine := "minio://docs/ine.pdf"
	db := &fakeDB{queryRows: []rowResult{{values: []any{int64(105), acta, ine, nil, nil, nil, nil, 1, now, now}}}, execAffected: []int64{1}}
	repo := NewExpedienteRepository(db)

	got, err := repo.GetDocumentos(context.Background(), 105, 12)
	require.NoError(t, err)
	require.NotNil(t, got.ActaURL)
	assert.Equal(t, acta, *got.ActaURL)
	assert.True(t, assertSQLContains(db.calls[0].sql, "empresa_id=$2"), db.calls[0].sql)

	err = repo.UpsertDocumentos(context.Background(), &empleados.Documentos{EmpleadoID: 105, ActaURL: &acta, INEURL: &ine, EstatusValidacion: 1}, 12)
	require.NoError(t, err)
	assert.True(t, assertSQLContains(db.calls[1].sql, "on conflict (empleado_id) do update"), db.calls[1].sql)
}

func TestExpedienteRepositoryGetMissingReturnsEmpleadoNotFound(t *testing.T) {
	repo := NewExpedienteRepository(&fakeDB{queryRows: []rowResult{{err: errors.New("unexpected")}}})
	_, err := repo.GetUbicacion(context.Background(), 105, 12)
	assert.Error(t, err)
}
