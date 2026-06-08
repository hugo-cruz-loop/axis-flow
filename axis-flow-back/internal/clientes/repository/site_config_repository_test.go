package repository_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"axis-flow-back/internal/clientes"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// SiteConfigRepository interface
// ---------------------------------------------------------------------------

type SiteConfigRepositorier interface {
	AddServicio(ctx context.Context, localidadID uuid.UUID, servicioID int64) error
	RemoveServicio(ctx context.Context, localidadID uuid.UUID, servicioID int64) error
	ListServicios(ctx context.Context, localidadID uuid.UUID, empresaID int64) ([]clientes.ServiciosLocalidad, error)
	CreateHorario(ctx context.Context, h *clientes.Horario) error
	ListHorarios(ctx context.Context, localidadID uuid.UUID, empresaID int64) ([]clientes.Horario, error)
	UpdateHorario(ctx context.Context, h *clientes.Horario) error
	DeleteHorario(ctx context.Context, id uuid.UUID, empresaID int64) error
	CreateHerramienta(ctx context.Context, h *clientes.Herramienta) error
	ListHerramientas(ctx context.Context, localidadID uuid.UUID, empresaID int64) ([]clientes.Herramienta, error)
	UpdateHerramienta(ctx context.Context, h *clientes.Herramienta) error
	DeleteHerramienta(ctx context.Context, id uuid.UUID, empresaID int64) error
	CreateActividad(ctx context.Context, a *clientes.Actividad) error
	ListActividades(ctx context.Context, localidadID uuid.UUID, empresaID int64) ([]clientes.Actividad, error)
	UpdateActividad(ctx context.Context, a *clientes.Actividad) error
	DeleteActividad(ctx context.Context, id uuid.UUID, empresaID int64) error
}

// ---------------------------------------------------------------------------
// In-memory mock
// ---------------------------------------------------------------------------

type mockSiteConfigRepo struct {
	servicios    map[string]*clientes.ServiciosLocalidad // key: localidadID+servicioID
	horarios     map[string]*clientes.Horario
	herramientas map[string]*clientes.Herramienta
	actividades  map[string]*clientes.Actividad
	// localidad → empresa mapping for tenant isolation
	empresaByLocalidad map[string]int64
}

func newMockSiteConfigRepo() *mockSiteConfigRepo {
	return &mockSiteConfigRepo{
		servicios:          make(map[string]*clientes.ServiciosLocalidad),
		horarios:           make(map[string]*clientes.Horario),
		herramientas:       make(map[string]*clientes.Herramienta),
		actividades:        make(map[string]*clientes.Actividad),
		empresaByLocalidad: make(map[string]int64),
	}
}

func (m *mockSiteConfigRepo) registerLocalidad(localidadID uuid.UUID, empresaID int64) {
	m.empresaByLocalidad[localidadID.String()] = empresaID
}

func (m *mockSiteConfigRepo) checkLocalidadTenant(localidadID uuid.UUID, empresaID int64) bool {
	eid, ok := m.empresaByLocalidad[localidadID.String()]
	return ok && eid == empresaID
}

func (m *mockSiteConfigRepo) AddServicio(_ context.Context, localidadID uuid.UUID, servicioID int64) error {
	key := fmt.Sprintf("%s:%d", localidadID, servicioID)
	if _, exists := m.servicios[key]; exists {
		return fmt.Errorf("siteConfigRepository.AddServicio: servicio already assigned to localidad")
	}
	m.servicios[key] = &clientes.ServiciosLocalidad{
		ID:           uuid.New(),
		LocalidadID:  localidadID,
		ServicioID:   servicioID,
		StatusActivo: true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	return nil
}

func (m *mockSiteConfigRepo) RemoveServicio(_ context.Context, localidadID uuid.UUID, servicioID int64) error {
	key := fmt.Sprintf("%s:%d", localidadID, servicioID)
	delete(m.servicios, key)
	return nil
}

func (m *mockSiteConfigRepo) ListServicios(_ context.Context, localidadID uuid.UUID, empresaID int64) ([]clientes.ServiciosLocalidad, error) {
	if !m.checkLocalidadTenant(localidadID, empresaID) {
		return nil, nil
	}
	var out []clientes.ServiciosLocalidad
	prefix := localidadID.String() + ":"
	for k, s := range m.servicios {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			out = append(out, *s)
		}
	}
	return out, nil
}

func (m *mockSiteConfigRepo) CreateHorario(_ context.Context, h *clientes.Horario) error {
	h.ID = uuid.New()
	h.CreatedAt = time.Now()
	h.UpdatedAt = time.Now()
	cp := *h
	m.horarios[h.ID.String()] = &cp
	return nil
}

func (m *mockSiteConfigRepo) ListHorarios(_ context.Context, localidadID uuid.UUID, empresaID int64) ([]clientes.Horario, error) {
	if !m.checkLocalidadTenant(localidadID, empresaID) {
		return nil, nil
	}
	var out []clientes.Horario
	for _, h := range m.horarios {
		if h.LocalidadID == localidadID {
			out = append(out, *h)
		}
	}
	return out, nil
}

func (m *mockSiteConfigRepo) UpdateHorario(_ context.Context, h *clientes.Horario) error {
	if _, ok := m.horarios[h.ID.String()]; !ok {
		return clientes.ErrLocalidadNotFound
	}
	cp := *h
	m.horarios[h.ID.String()] = &cp
	return nil
}

func (m *mockSiteConfigRepo) DeleteHorario(_ context.Context, id uuid.UUID, _ int64) error {
	if _, ok := m.horarios[id.String()]; !ok {
		return clientes.ErrLocalidadNotFound
	}
	delete(m.horarios, id.String())
	return nil
}

func (m *mockSiteConfigRepo) CreateHerramienta(_ context.Context, h *clientes.Herramienta) error {
	h.ID = uuid.New()
	h.CreatedAt = time.Now()
	h.UpdatedAt = time.Now()
	cp := *h
	m.herramientas[h.ID.String()] = &cp
	return nil
}

func (m *mockSiteConfigRepo) ListHerramientas(_ context.Context, localidadID uuid.UUID, empresaID int64) ([]clientes.Herramienta, error) {
	if !m.checkLocalidadTenant(localidadID, empresaID) {
		return nil, nil
	}
	var out []clientes.Herramienta
	for _, h := range m.herramientas {
		if h.LocalidadID == localidadID {
			out = append(out, *h)
		}
	}
	return out, nil
}

func (m *mockSiteConfigRepo) UpdateHerramienta(_ context.Context, h *clientes.Herramienta) error {
	if _, ok := m.herramientas[h.ID.String()]; !ok {
		return clientes.ErrLocalidadNotFound
	}
	cp := *h
	m.herramientas[h.ID.String()] = &cp
	return nil
}

func (m *mockSiteConfigRepo) DeleteHerramienta(_ context.Context, id uuid.UUID, _ int64) error {
	if _, ok := m.herramientas[id.String()]; !ok {
		return clientes.ErrLocalidadNotFound
	}
	delete(m.herramientas, id.String())
	return nil
}

func (m *mockSiteConfigRepo) CreateActividad(_ context.Context, a *clientes.Actividad) error {
	a.ID = uuid.New()
	a.CreatedAt = time.Now()
	a.UpdatedAt = time.Now()
	cp := *a
	m.actividades[a.ID.String()] = &cp
	return nil
}

func (m *mockSiteConfigRepo) ListActividades(_ context.Context, localidadID uuid.UUID, empresaID int64) ([]clientes.Actividad, error) {
	if !m.checkLocalidadTenant(localidadID, empresaID) {
		return nil, nil
	}
	var out []clientes.Actividad
	for _, a := range m.actividades {
		if a.LocalidadID == localidadID {
			out = append(out, *a)
		}
	}
	return out, nil
}

func (m *mockSiteConfigRepo) UpdateActividad(_ context.Context, a *clientes.Actividad) error {
	if _, ok := m.actividades[a.ID.String()]; !ok {
		return clientes.ErrLocalidadNotFound
	}
	cp := *a
	m.actividades[a.ID.String()] = &cp
	return nil
}

func (m *mockSiteConfigRepo) DeleteActividad(_ context.Context, id uuid.UUID, _ int64) error {
	if _, ok := m.actividades[id.String()]; !ok {
		return clientes.ErrLocalidadNotFound
	}
	delete(m.actividades, id.String())
	return nil
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestSiteConfigRepo_AddServicio_Duplicate(t *testing.T) {
	repo := newMockSiteConfigRepo()
	localidadID := uuid.New()
	repo.registerLocalidad(localidadID, 1)

	require.NoError(t, repo.AddServicio(context.Background(), localidadID, 10))
	err := repo.AddServicio(context.Background(), localidadID, 10)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already assigned")
}

func TestSiteConfigRepo_ListHorarios_TenantIsolation(t *testing.T) {
	repo := newMockSiteConfigRepo()
	localidadID := uuid.New()
	repo.registerLocalidad(localidadID, 1)

	h := &clientes.Horario{LocalidadID: localidadID, HoraEntrada: "08:00", HoraSalida: "17:00"}
	require.NoError(t, repo.CreateHorario(context.Background(), h))

	// Correct empresa → returns items
	list, err := repo.ListHorarios(context.Background(), localidadID, 1)
	require.NoError(t, err)
	assert.Len(t, list, 1)

	// Wrong empresa → empty (no cross-tenant leak)
	list2, err := repo.ListHorarios(context.Background(), localidadID, 999)
	require.NoError(t, err)
	assert.Empty(t, list2)
}

func TestSiteConfigRepo_CreateHerramienta_HappyPath(t *testing.T) {
	repo := newMockSiteConfigRepo()
	localidadID := uuid.New()
	h := &clientes.Herramienta{LocalidadID: localidadID, Nombre: "Taladro", Cantidad: 2}
	require.NoError(t, repo.CreateHerramienta(context.Background(), h))
	assert.NotEqual(t, uuid.Nil, h.ID)
}

func TestSiteConfigRepo_CreateActividad_HappyPath(t *testing.T) {
	repo := newMockSiteConfigRepo()
	localidadID := uuid.New()
	a := &clientes.Actividad{LocalidadID: localidadID, Descripcion: "Limpieza", Frecuencia: "diaria", Orden: 1}
	require.NoError(t, repo.CreateActividad(context.Background(), a))
	assert.NotEqual(t, uuid.Nil, a.ID)
}

// Compile-time interface check
var _ SiteConfigRepositorier = (*mockSiteConfigRepo)(nil)
