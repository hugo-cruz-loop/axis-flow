package service

import (
	"context"
	"encoding/json"
	"fmt"

	"axis-flow-back/internal/clientes"
	clicache "axis-flow-back/internal/clientes/cache"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// LocalidadRepository is the subset of repository.LocalidadRepository used by the service layer.
type LocalidadRepository interface {
	Create(ctx context.Context, l *clientes.Localidad) error
	FindByID(ctx context.Context, id uuid.UUID, empresaID int64) (*clientes.Localidad, error)
	ListByCliente(ctx context.Context, clienteID uuid.UUID, empresaID int64) ([]clientes.Localidad, error)
	Update(ctx context.Context, l *clientes.Localidad) error
	Delete(ctx context.Context, id uuid.UUID, empresaID int64) error
}

// SiteConfigRepository is the subset of repository.SiteConfigRepository used by the service layer.
type SiteConfigRepository interface {
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

// LocalidadServicer defines business operations for Localidad and its sub-entities.
type LocalidadServicer interface {
	CreateLocalidad(ctx context.Context, l *clientes.Localidad, empresaID int64) (*clientes.Localidad, error)
	GetLocalidad(ctx context.Context, id uuid.UUID, empresaID int64) (*clientes.Localidad, error)
	ListLocalidades(ctx context.Context, clienteID uuid.UUID, empresaID int64) ([]clientes.Localidad, error)
	UpdateLocalidad(ctx context.Context, l *clientes.Localidad, empresaID int64) (*clientes.Localidad, error)
	DeleteLocalidad(ctx context.Context, id uuid.UUID, empresaID int64) error

	AddServicio(ctx context.Context, localidadID uuid.UUID, servicioID int64, empresaID int64) error
	RemoveServicio(ctx context.Context, localidadID uuid.UUID, servicioID int64, empresaID int64) error
	ListServicios(ctx context.Context, localidadID uuid.UUID, empresaID int64) ([]clientes.ServiciosLocalidad, error)

	CreateHorario(ctx context.Context, h *clientes.Horario, empresaID int64) (*clientes.Horario, error)
	ListHorarios(ctx context.Context, localidadID uuid.UUID, empresaID int64) ([]clientes.Horario, error)
	UpdateHorario(ctx context.Context, h *clientes.Horario, empresaID int64) (*clientes.Horario, error)
	DeleteHorario(ctx context.Context, id uuid.UUID, empresaID int64) error

	CreateHerramienta(ctx context.Context, h *clientes.Herramienta, empresaID int64) (*clientes.Herramienta, error)
	ListHerramientas(ctx context.Context, localidadID uuid.UUID, empresaID int64) ([]clientes.Herramienta, error)
	UpdateHerramienta(ctx context.Context, h *clientes.Herramienta, empresaID int64) (*clientes.Herramienta, error)
	DeleteHerramienta(ctx context.Context, id uuid.UUID, empresaID int64) error

	CreateActividad(ctx context.Context, a *clientes.Actividad, empresaID int64) (*clientes.Actividad, error)
	ListActividades(ctx context.Context, localidadID uuid.UUID, empresaID int64) ([]clientes.Actividad, error)
	UpdateActividad(ctx context.Context, a *clientes.Actividad, empresaID int64) (*clientes.Actividad, error)
	DeleteActividad(ctx context.Context, id uuid.UUID, empresaID int64) error
}

// LocalidadService implements LocalidadServicer.
type LocalidadService struct {
	repo       LocalidadRepository
	siteRepo   SiteConfigRepository
	rdb        *redis.Client
}

// NewLocalidadService constructs a LocalidadService.
func NewLocalidadService(repo LocalidadRepository, siteRepo SiteConfigRepository, rdb *redis.Client) *LocalidadService {
	return &LocalidadService{repo: repo, siteRepo: siteRepo, rdb: rdb}
}

// ---- Localidad CRUD ---------------------------------------------------------

func (s *LocalidadService) CreateLocalidad(ctx context.Context, l *clientes.Localidad, _ int64) (*clientes.Localidad, error) {
	if err := s.repo.Create(ctx, l); err != nil {
		return nil, fmt.Errorf("localidadService.CreateLocalidad: %w", err)
	}
	_ = clicache.InvalidateLocalidad(ctx, s.rdb, l.ID)
	return l, nil
}

func (s *LocalidadService) GetLocalidad(ctx context.Context, id uuid.UUID, empresaID int64) (*clientes.Localidad, error) {
	key := clicache.LocalidadCacheKey(id)

	raw, err := s.rdb.Get(ctx, key).Bytes()
	if err == nil {
		var l clientes.Localidad
		if jsonErr := json.Unmarshal(raw, &l); jsonErr == nil {
			return &l, nil
		}
	}

	l, err := s.repo.FindByID(ctx, id, empresaID)
	if err != nil {
		return nil, err
	}

	if data, jsonErr := json.Marshal(l); jsonErr == nil {
		_ = s.rdb.Set(ctx, key, data, clienteCacheTTL).Err()
	}

	return l, nil
}

func (s *LocalidadService) ListLocalidades(ctx context.Context, clienteID uuid.UUID, empresaID int64) ([]clientes.Localidad, error) {
	return s.repo.ListByCliente(ctx, clienteID, empresaID)
}

func (s *LocalidadService) UpdateLocalidad(ctx context.Context, l *clientes.Localidad, _ int64) (*clientes.Localidad, error) {
	if err := s.repo.Update(ctx, l); err != nil {
		return nil, fmt.Errorf("localidadService.UpdateLocalidad: %w", err)
	}
	_ = clicache.InvalidateLocalidad(ctx, s.rdb, l.ID)
	return l, nil
}

func (s *LocalidadService) DeleteLocalidad(ctx context.Context, id uuid.UUID, empresaID int64) error {
	if err := s.repo.Delete(ctx, id, empresaID); err != nil {
		return fmt.Errorf("localidadService.DeleteLocalidad: %w", err)
	}
	_ = clicache.InvalidateLocalidad(ctx, s.rdb, id)
	return nil
}

// ---- Servicios --------------------------------------------------------------

func (s *LocalidadService) AddServicio(ctx context.Context, localidadID uuid.UUID, servicioID int64, _ int64) error {
	return s.siteRepo.AddServicio(ctx, localidadID, servicioID)
}

func (s *LocalidadService) RemoveServicio(ctx context.Context, localidadID uuid.UUID, servicioID int64, _ int64) error {
	return s.siteRepo.RemoveServicio(ctx, localidadID, servicioID)
}

func (s *LocalidadService) ListServicios(ctx context.Context, localidadID uuid.UUID, empresaID int64) ([]clientes.ServiciosLocalidad, error) {
	return s.siteRepo.ListServicios(ctx, localidadID, empresaID)
}

// ---- Horarios ---------------------------------------------------------------

func (s *LocalidadService) CreateHorario(ctx context.Context, h *clientes.Horario, _ int64) (*clientes.Horario, error) {
	if err := s.siteRepo.CreateHorario(ctx, h); err != nil {
		return nil, fmt.Errorf("localidadService.CreateHorario: %w", err)
	}
	return h, nil
}

func (s *LocalidadService) ListHorarios(ctx context.Context, localidadID uuid.UUID, empresaID int64) ([]clientes.Horario, error) {
	return s.siteRepo.ListHorarios(ctx, localidadID, empresaID)
}

func (s *LocalidadService) UpdateHorario(ctx context.Context, h *clientes.Horario, _ int64) (*clientes.Horario, error) {
	if err := s.siteRepo.UpdateHorario(ctx, h); err != nil {
		return nil, fmt.Errorf("localidadService.UpdateHorario: %w", err)
	}
	return h, nil
}

func (s *LocalidadService) DeleteHorario(ctx context.Context, id uuid.UUID, empresaID int64) error {
	return s.siteRepo.DeleteHorario(ctx, id, empresaID)
}

// ---- Herramientas -----------------------------------------------------------

func (s *LocalidadService) CreateHerramienta(ctx context.Context, h *clientes.Herramienta, _ int64) (*clientes.Herramienta, error) {
	if err := s.siteRepo.CreateHerramienta(ctx, h); err != nil {
		return nil, fmt.Errorf("localidadService.CreateHerramienta: %w", err)
	}
	return h, nil
}

func (s *LocalidadService) ListHerramientas(ctx context.Context, localidadID uuid.UUID, empresaID int64) ([]clientes.Herramienta, error) {
	return s.siteRepo.ListHerramientas(ctx, localidadID, empresaID)
}

func (s *LocalidadService) UpdateHerramienta(ctx context.Context, h *clientes.Herramienta, _ int64) (*clientes.Herramienta, error) {
	if err := s.siteRepo.UpdateHerramienta(ctx, h); err != nil {
		return nil, fmt.Errorf("localidadService.UpdateHerramienta: %w", err)
	}
	return h, nil
}

func (s *LocalidadService) DeleteHerramienta(ctx context.Context, id uuid.UUID, empresaID int64) error {
	return s.siteRepo.DeleteHerramienta(ctx, id, empresaID)
}

// ---- Actividades ------------------------------------------------------------

func (s *LocalidadService) CreateActividad(ctx context.Context, a *clientes.Actividad, _ int64) (*clientes.Actividad, error) {
	if err := s.siteRepo.CreateActividad(ctx, a); err != nil {
		return nil, fmt.Errorf("localidadService.CreateActividad: %w", err)
	}
	return a, nil
}

func (s *LocalidadService) ListActividades(ctx context.Context, localidadID uuid.UUID, empresaID int64) ([]clientes.Actividad, error) {
	return s.siteRepo.ListActividades(ctx, localidadID, empresaID)
}

func (s *LocalidadService) UpdateActividad(ctx context.Context, a *clientes.Actividad, _ int64) (*clientes.Actividad, error) {
	if err := s.siteRepo.UpdateActividad(ctx, a); err != nil {
		return nil, fmt.Errorf("localidadService.UpdateActividad: %w", err)
	}
	return a, nil
}

func (s *LocalidadService) DeleteActividad(ctx context.Context, id uuid.UUID, empresaID int64) error {
	return s.siteRepo.DeleteActividad(ctx, id, empresaID)
}
