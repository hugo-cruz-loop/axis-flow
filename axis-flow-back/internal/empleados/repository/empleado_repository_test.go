package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	identity "axis-flow-back/internal/domain"
	empleados "axis-flow-back/internal/empleados"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmpleadoRepositoryCreateIsAtomicWithIdentityUser(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	userID := uuid.New()
	tenantID := uuid.New()
	tx := &fakeTx{fakeDB: fakeDB{queryRows: []rowResult{{values: []any{int64(105), empleados.EmpleadoStatusIncompleto, now, now}}}}}
	db := &fakeDB{tx: tx}
	repo := NewEmpleadoRepository(db)

	created, err := repo.Create(ctx, CreateEmpleadoParams{
		UsuarioID:       userID,
		TenantID:        tenantID,
		Email:           "carlos.sanchez@empresa.com",
		PasswordHash:    "activation-placeholder",
		IDEmpleado:      "EMP-2026-901",
		EmpresaID:       12,
		Nombre:          "Carlos",
		ApellidoPaterno: "Sanchez",
		ApellidoMaterno: "Gomez",
		CreatedBy:       userID,
	})

	require.NoError(t, err)
	require.True(t, db.beginCalled, "Create must start a transaction")
	require.True(t, tx.committed, "Create must commit when both records succeed")
	require.False(t, tx.rolledBack, "Create must not rollback after commit")
	require.NotNil(t, created)
	assert.Equal(t, int64(105), created.NumEmpleado)
	assert.Equal(t, empleados.EmpleadoStatusIncompleto, created.Status)
	require.Len(t, tx.calls, 3)
	assert.True(t, assertSQLContains(tx.calls[0].sql, "insert into users.identity_users"))
	assert.Contains(t, tx.calls[0].args, string(identity.StatusPendingActivation))
	assert.True(t, assertSQLContains(tx.calls[1].sql, "insert into empleados.empleados_empleado"))
	assert.True(t, assertSQLContains(tx.calls[2].sql, "insert into users.identity_user_roles"))
	assert.Contains(t, tx.calls[2].args, "EMPLEADO")
}

func TestEmpleadoRepositoryCreateRollsBackWhenEmpleadoInsertFails(t *testing.T) {
	ctx := context.Background()
	tx := &fakeTx{fakeDB: fakeDB{queryRows: []rowResult{{err: errors.New("empleado insert failed")}}}}
	db := &fakeDB{tx: tx}
	repo := NewEmpleadoRepository(db)

	_, err := repo.Create(ctx, CreateEmpleadoParams{UsuarioID: uuid.New(), TenantID: uuid.New(), Email: "x@y.com", PasswordHash: "hash", IDEmpleado: "EMP-1", EmpresaID: 12, Nombre: "X", ApellidoPaterno: "Y"})

	require.Error(t, err)
	assert.True(t, tx.rolledBack)
	assert.False(t, tx.committed)
}

func TestEmpleadoRepositoryGetByIDFiltersByEmpresa(t *testing.T) {
	now := time.Now()
	userID := uuid.New()
	db := &fakeDB{queryRows: []rowResult{empleadoRow(empleados.Empleado{NumEmpleado: 105, IDEmpleado: "EMP-901", UsuarioID: userID, EmpresaID: 12, Nombre: "Carlos", ApellidoPaterno: "Sanchez", ApellidoMaterno: "Gomez", Status: empleados.EmpleadoStatusActivo, CreatedAt: now, UpdatedAt: now})}}
	repo := NewEmpleadoRepository(db)

	got, err := repo.GetByID(context.Background(), 105, 12)

	require.NoError(t, err)
	assert.Equal(t, int64(12), got.EmpresaID)
	require.Len(t, db.calls, 1)
	assert.True(t, assertSQLContains(db.calls[0].sql, "where num_empleado=$1 and empresa_id=$2"), db.calls[0].sql)
}

func TestEmpleadoRepositoryGetByEmpresaReturnsEmpresaScopedRows(t *testing.T) {
	now := time.Now()
	db := &fakeDB{queryResults: []*fakeRows{{rows: []rowResult{
		empleadoRow(empleados.Empleado{NumEmpleado: 1, IDEmpleado: "E-1", UsuarioID: uuid.New(), EmpresaID: 12, Nombre: "A", ApellidoPaterno: "B", Status: empleados.EmpleadoStatusActivo, CreatedAt: now, UpdatedAt: now}),
		empleadoRow(empleados.Empleado{NumEmpleado: 2, IDEmpleado: "E-2", UsuarioID: uuid.New(), EmpresaID: 12, Nombre: "C", ApellidoPaterno: "D", Status: empleados.EmpleadoStatusIncompleto, CreatedAt: now, UpdatedAt: now}),
	}}}}
	repo := NewEmpleadoRepository(db)

	got, err := repo.GetByEmpresa(context.Background(), 12)

	require.NoError(t, err)
	assert.Len(t, got, 2)
	assert.Equal(t, int64(12), got[0].EmpresaID)
	assert.True(t, assertSQLContains(db.calls[0].sql, "where empresa_id=$1"), db.calls[0].sql)
}

func TestEmpleadoRepositoryGetByUserIDFiltersByEmpresa(t *testing.T) {
	now := time.Now()
	userID := uuid.New()
	db := &fakeDB{queryRows: []rowResult{empleadoRow(empleados.Empleado{NumEmpleado: 105, IDEmpleado: "EMP-901", UsuarioID: userID, EmpresaID: 12, Nombre: "Carlos", ApellidoPaterno: "Sanchez", Status: empleados.EmpleadoStatusActivo, CreatedAt: now, UpdatedAt: now})}}
	repo := NewEmpleadoRepository(db)

	got, err := repo.GetByUserID(context.Background(), userID, 12)

	require.NoError(t, err)
	assert.Equal(t, userID, got.UsuarioID)
	assert.True(t, assertSQLContains(db.calls[0].sql, "where usuario_id=$1 and empresa_id=$2"), db.calls[0].sql)
}

func TestEmpleadoRepositorySoftDeleteMarksEmpleadoAndDeactivatesUserAtomically(t *testing.T) {
	userID := uuid.New()
	tx := &fakeTx{fakeDB: fakeDB{queryRows: []rowResult{{values: []any{userID}}}}}
	db := &fakeDB{tx: tx}
	repo := NewEmpleadoRepository(db)

	err := repo.SoftDelete(context.Background(), 105, 12)

	require.NoError(t, err)
	assert.True(t, tx.committed)
	require.Len(t, tx.calls, 2)
	assert.True(t, assertSQLContains(tx.calls[0].sql, "update empleados.empleados_empleado"))
	assert.True(t, assertSQLContains(tx.calls[0].sql, "where num_empleado=$2 and empresa_id=$3"), tx.calls[0].sql)
	assert.Equal(t, empleados.EmpleadoStatusBaja, tx.calls[0].args[0])
	assert.True(t, assertSQLContains(tx.calls[1].sql, "update users.identity_users"))
	assert.Contains(t, tx.calls[1].args, string(identity.StatusInactive))
}

func TestEmpleadoRepositoryUpdateIsEmpresaScoped(t *testing.T) {
	db := &fakeDB{execAffected: []int64{1}}
	repo := NewEmpleadoRepository(db)
	e := &empleados.Empleado{NumEmpleado: 105, EmpresaID: 12, Nombre: "Carlos", ApellidoPaterno: "Sanchez", ApellidoMaterno: "Gomez", Status: empleados.EmpleadoStatusActivo}

	err := repo.Update(context.Background(), e)

	require.NoError(t, err)
	assert.True(t, assertSQLContains(db.calls[0].sql, "where num_empleado=$5 and empresa_id=$6"), db.calls[0].sql)
}
