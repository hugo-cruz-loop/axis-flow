// Package service — EventoService: evento header CRUD + check-in lifecycle.
//
// PR-3 (Services) — task 3.2. Gherkin 2 (Llenado en campo de checklist con
// fotos y geolocalización por operario): the check-in step (POST
// /evento_iniciado) is the IniciarEvento entry point. The spec publishes
// EventoIniciado on check-in, not on header creation — CreateEvento
// therefore intentionally does NOT publish.
package service

import (
	"context"
	"strings"
	"time"

	"axis-flow-back/internal/formularios"
	"axis-flow-back/internal/formularios/events"

	"github.com/google/uuid"
)

// EventoService defines the use-case operations exposed to the HTTP
// handler layer for Evento headers and their check-in (EventoIniciado)
// lifecycle.
type EventoService interface {
	// CreateEvento validates the event, persists the header and the M:N
	// asociaciones to formularios, and returns the persisted event.
	// No event is published (the spec publishes only on check-in).
	CreateEvento(ctx context.Context, e *formularios.Evento, formularioIDs []uuid.UUID, empresaID uuid.UUID) (*formularios.Evento, error)

	// GetEvento returns an evento scoped to the caller's empresa. Foreign
	// tenants receive formularios.ErrNotFound (no leak).
	GetEvento(ctx context.Context, id, empresaID uuid.UUID) (*formularios.Evento, error)

	// GetEventosByEmpCte lists eventos visible to a (empresa, cliente)
	// pair, optionally filtered by status. Pure delegation.
	GetEventosByEmpCte(ctx context.Context, empresaID, clienteID uuid.UUID, status *string, page, pageSize int) ([]*formularios.Evento, int, error)

	// IniciarEvento persists a check-in (employee start of execution)
	// for an evento the caller owns, publishes EventoIniciado, and
	// invalidates the cached pendientes list for the (empresa, cliente)
	// pair. The parent evento's ownership is re-verified here even
	// though the repository enforces it again at the FK level — defence
	// in depth.
	IniciarEvento(ctx context.Context, ei *formularios.EventoIniciado, empresaID uuid.UUID) (*formularios.EventoIniciado, error)
}

// eventoService is the concrete implementation.
type eventoService struct {
	repo  formularios.EventoRepository
	pub   events.EventPublisher
	cache CacheInvalidator
}

// NewEventoService constructs an EventoService.
func NewEventoService(
	repo formularios.EventoRepository,
	pub events.EventPublisher,
	cache CacheInvalidator,
) EventoService {
	return &eventoService{repo: repo, pub: pub, cache: cache}
}

// ---------------------------------------------------------------------------
// CreateEvento
// ---------------------------------------------------------------------------

func (s *eventoService) CreateEvento(
	ctx context.Context,
	e *formularios.Evento,
	formularioIDs []uuid.UUID,
	empresaID uuid.UUID,
) (*formularios.Evento, error) {
	if err := validateEvento(e); err != nil {
		return nil, err
	}
	if e.EmpresaID != empresaID {
		return nil, formularios.ErrForbidden
	}
	if err := s.repo.Create(ctx, e, formularioIDs); err != nil {
		return nil, err
	}
	// No publish, no cache invalidation on header creation. The
	// pendiente list is invalidated on IniciarEvento (which is the spec's
	// emission boundary) so creating a pendiente keeps the cached list
	// consistent with reality.
	return e, nil
}

// validateEvento checks the structural invariants. Status must already be
// set by the caller (the openapi does not expose it; the service defaults
// to pendiente on the wire path in PR-4).
func validateEvento(e *formularios.Evento) error {
	if e == nil {
		return formularios.ErrInvalidInput
	}
	if strings.TrimSpace(e.Nombre) == "" {
		return formularios.ErrInvalidInput
	}
	if e.FechaProgramada.IsZero() {
		return formularios.ErrInvalidInput
	}
	if e.ClienteID == uuid.Nil {
		return formularios.ErrInvalidInput
	}
	return nil
}

// ---------------------------------------------------------------------------
// GetEvento
// ---------------------------------------------------------------------------

func (s *eventoService) GetEvento(ctx context.Context, id, empresaID uuid.UUID) (*formularios.Evento, error) {
	return s.repo.GetByID(ctx, id, empresaID)
}

// ---------------------------------------------------------------------------
// GetEventosByEmpCte
// ---------------------------------------------------------------------------

func (s *eventoService) GetEventosByEmpCte(
	ctx context.Context,
	empresaID, clienteID uuid.UUID,
	status *string,
	page, pageSize int,
) ([]*formularios.Evento, int, error) {
	return s.repo.ListByEmpCte(ctx, empresaID, clienteID, status, page, pageSize)
}

// ---------------------------------------------------------------------------
// IniciarEvento
// ---------------------------------------------------------------------------

func (s *eventoService) IniciarEvento(
	ctx context.Context,
	ei *formularios.EventoIniciado,
	empresaID uuid.UUID,
) (*formularios.EventoIniciado, error) {
	if err := validateIniciado(ei); err != nil {
		return nil, err
	}
	// IDOR: confirm the parent evento belongs to the caller's empresa.
	// The repository GetByID returns formularios.ErrNotFound for foreign
	// tenants so this call also doubles as the "not found" check for
	// unknown evento IDs.
	parent, err := s.repo.GetByID(ctx, ei.EventoID, empresaID)
	if err != nil {
		return nil, err
	}
	// Stamp the check-in time if the caller did not.
	if ei.CheckInTime.IsZero() {
		ei.CheckInTime = time.Now().UTC()
	}
	if err := s.repo.CreateIniciado(ctx, ei); err != nil {
		return nil, err
	}

	// Build the payload once — used by both the publish and the
	// post-write hook. Geo coords and check_in_time are NOT PII: they
	// are operational telemetry that the Reporting consumer
	// (AtencionSeguimiento) needs.
	lat, lon := geoOrNil(ei.GeolocalizacionInicioLat, ei.GeolocalizacionInicioLon)
	postWriteHook(ctx, s.pub, func(c context.Context) error {
		return s.cache.UnlinkEventosByEmpCte(c, parent.EmpresaID, parent.ClienteID)
	}, events.StreamEventoIniciado, map[string]any{
		"evento_id":                  parent.ID,
		"evento_iniciado_id":         ei.ID,
		"empresa_id":                 parent.EmpresaID,
		"empleado_id":                ei.EmpleadoID,
		"check_in_time":              ei.CheckInTime,
		"geolocalizacion_inicio_lat": lat,
		"geolocalizacion_inicio_lon": lon,
	})

	return ei, nil
}

// validateIniciado checks the structural invariants of a check-in. The
// SQL CHECK on status and geo is enforced in the repository; we add the
// EmpleadoID > 0 check here so the error envelope is uniform with the
// service's other validation paths.
func validateIniciado(ei *formularios.EventoIniciado) error {
	if ei == nil {
		return formularios.ErrInvalidInput
	}
	if ei.EventoID == uuid.Nil {
		return formularios.ErrInvalidInput
	}
	if ei.EmpleadoID <= 0 {
		return formularios.ErrInvalidInput
	}
	if ei.Status == "" {
		return formularios.ErrInvalidInput
	}
	return nil
}

// geoOrNil returns (lat, lon) with both as float64, or (0, 0) when either
// is nil. The wire payload always has the keys so consumers do not have
// to branch on missing fields. 0/0 is a valid lat/lon (Gulf of Guinea) so
// the spec consumer can distinguish a present-zero from a missing key by
// the fact that the keys are always present in the payload.
func geoOrNil(lat, lon *float64) (float64, float64) {
	if lat == nil || lon == nil {
		return 0, 0
	}
	return *lat, *lon
}
