package service_test

import (
	"context"
	"testing"
	"time"

	empleados "axis-flow-back/internal/empleados"
	"axis-flow-back/internal/empleados/repository"
	"axis-flow-back/internal/empleados/service"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type fakeEmpleadoRepo struct {
	createParams repository.CreateEmpleadoParams
	created      *empleados.Empleado
	createErr    error
	getByID      *empleados.Empleado
	updated      *empleados.Empleado
	softDeleted  struct {
		numEmpleado int64
		empresaID   int64
	}
}

func (f *fakeEmpleadoRepo) Create(ctx context.Context, p repository.CreateEmpleadoParams) (*empleados.Empleado, error) {
	f.createParams = p
	if f.createErr != nil {
		return nil, f.createErr
	}
	if f.created != nil {
		return f.created, nil
	}
	return &empleados.Empleado{NumEmpleado: 105, IDEmpleado: p.IDEmpleado, UsuarioID: p.UsuarioID, EmpresaID: p.EmpresaID, Nombre: p.Nombre, ApellidoPaterno: p.ApellidoPaterno, ApellidoMaterno: p.ApellidoMaterno, Status: empleados.EmpleadoStatusIncompleto, CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
}

func (f *fakeEmpleadoRepo) GetByID(ctx context.Context, numEmpleado, empresaID int64) (*empleados.Empleado, error) {
	return f.getByID, nil
}

func (f *fakeEmpleadoRepo) GetByEmpresa(ctx context.Context, empresaID int64) ([]empleados.Empleado, error) {
	return nil, nil
}

func (f *fakeEmpleadoRepo) GetByUserID(ctx context.Context, userID uuid.UUID, empresaID int64) (*empleados.Empleado, error) {
	return nil, nil
}

func (f *fakeEmpleadoRepo) Update(ctx context.Context, e *empleados.Empleado) error {
	copy := *e
	f.updated = &copy
	return nil
}

func (f *fakeEmpleadoRepo) SoftDelete(ctx context.Context, numEmpleado, empresaID int64) error {
	f.softDeleted.numEmpleado = numEmpleado
	f.softDeleted.empresaID = empresaID
	return nil
}

type fakeTenantResolver struct {
	tenantID uuid.UUID
	calls    int
}

func (f *fakeTenantResolver) ResolveTenantID(ctx context.Context, empresaID int64) (uuid.UUID, error) {
	f.calls++
	return f.tenantID, nil
}

func TestEmpleadoServiceCreateAcceptsExplicitTenantID(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()
	repo := &fakeEmpleadoRepo{}
	resolver := &fakeTenantResolver{tenantID: uuid.New()}
	svc := service.NewEmpleadoService(repo, resolver)

	got, err := svc.CreateEmpleado(context.Background(), service.CreateEmpleadoRequest{
		TenantID:        tenantID,
		UsuarioID:       userID,
		Email:           "carlos.sanchez@empresa.com",
		IDEmpleado:      "EMP-2026-901",
		EmpresaID:       12,
		Nombre:          "Carlos",
		ApellidoPaterno: "Sanchez",
		ApellidoMaterno: "Gomez",
	})

	require.NoError(t, err)
	require.Equal(t, int64(105), got.NumEmpleado)
	require.Equal(t, tenantID, repo.createParams.TenantID)
	require.Equal(t, userID, repo.createParams.UsuarioID)
	require.Equal(t, "EMP-2026-901", repo.createParams.IDEmpleado)
	require.Zero(t, resolver.calls)
}

func TestEmpleadoServiceCreateResolvesTenantIDWhenMissing(t *testing.T) {
	tenantID := uuid.New()
	repo := &fakeEmpleadoRepo{}
	resolver := &fakeTenantResolver{tenantID: tenantID}
	svc := service.NewEmpleadoService(repo, resolver)

	_, err := svc.CreateEmpleado(context.Background(), service.CreateEmpleadoRequest{Email: "ana@empresa.com", IDEmpleado: "EMP-002", EmpresaID: 12, Nombre: "Ana", ApellidoPaterno: "Lopez"})

	require.NoError(t, err)
	require.Equal(t, 1, resolver.calls)
	require.Equal(t, tenantID, repo.createParams.TenantID)
}

func TestEmpleadoServiceUpdateAppliesProfileChangesWithinEmpresaScope(t *testing.T) {
	repo := &fakeEmpleadoRepo{getByID: &empleados.Empleado{NumEmpleado: 105, EmpresaID: 12, UsuarioID: uuid.New(), IDEmpleado: "EMP-001", Nombre: "Carlos", ApellidoPaterno: "Sanchez", Status: empleados.EmpleadoStatusIncompleto}}
	svc := service.NewEmpleadoService(repo, nil)

	got, err := svc.UpdateEmpleado(context.Background(), 105, 12, service.UpdateEmpleadoRequest{Nombre: ptrString("Carla"), ApellidoPaterno: ptrString("Gomez"), Status: ptrInt(empleados.EmpleadoStatusActivo)})

	require.NoError(t, err)
	require.Equal(t, "Carla", got.Nombre)
	require.Equal(t, "Gomez", repo.updated.ApellidoPaterno)
	require.Equal(t, empleados.EmpleadoStatusActivo, repo.updated.Status)
	require.Equal(t, int64(12), repo.updated.EmpresaID)
}

func TestEmpleadoServiceDeleteSoftDeletesWithinEmpresaScope(t *testing.T) {
	repo := &fakeEmpleadoRepo{}
	svc := service.NewEmpleadoService(repo, nil)

	err := svc.DeleteEmpleado(context.Background(), 105, 12)

	require.NoError(t, err)
	require.Equal(t, int64(105), repo.softDeleted.numEmpleado)
	require.Equal(t, int64(12), repo.softDeleted.empresaID)
}

func ptrString(v string) *string { return &v }
func ptrInt(v int) *int          { return &v }
