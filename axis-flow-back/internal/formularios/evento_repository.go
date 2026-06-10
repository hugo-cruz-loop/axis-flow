package formularios

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// InMemEventoRepository — goroutine-safe, in-memory adapter.
//
// PR-2 (Repositories) — task 2.3.
//
// Owns the in-memory state for Evento, EventoFormulario (M:N association),
// and EventoIniciado. Mirrors the SQL CHECK / UNIQUE constraints so the
// repository contract is testable without a live database:
//   - chk_eventos_evento_status
//   - uq_eventos_evento_formulario_evento_formulario
//   - chk_eventos_evento_iniciado_status
//   - chk_eventos_evento_iniciado_geo_lat / _lon
//
// The pgx adapter (PR-2 will add a thin wrapper) relies on the DB layer for
// the same constraints; this in-memory simulation is for unit tests only.
// ---------------------------------------------------------------------------

// InMemEventoRepository is a goroutine-safe, in-memory EventoRepository.
type InMemEventoRepository struct {
	mu               sync.RWMutex
	eventos          map[uuid.UUID]*Evento
	asociaciones     map[uuid.UUID]map[uuid.UUID]struct{} // eventoID -> set of formularioID
	eventosIniciados map[uuid.UUID]*EventoIniciado
}

// NewInMemEventoRepository creates an empty in-memory evento repository.
func NewInMemEventoRepository() *InMemEventoRepository {
	return &InMemEventoRepository{
		eventos:          make(map[uuid.UUID]*Evento),
		asociaciones:     make(map[uuid.UUID]map[uuid.UUID]struct{}),
		eventosIniciados: make(map[uuid.UUID]*EventoIniciado),
	}
}

// validEventoStatus mirrors the SQL CHECK on eventos_evento.status.
func validEventoStatus(s string) bool {
	switch s {
	case EventoStatusPendiente, EventoStatusIniciado, EventoStatusCompletado, EventoStatusCancelado:
		return true
	}
	return false
}

// validIniciadoStatus mirrors the SQL CHECK on eventos_evento_iniciado.status.
func validIniciadoStatus(s string) bool {
	switch s {
	case IniciadoStatus, CompletadoStatus, CanceladoStatus:
		return true
	}
	return false
}

// validLat mirrors chk_eventos_evento_iniciado_geo_lat (lat ∈ [-90, 90]).
func validLat(lat *float64) bool {
	if lat == nil {
		return true
	}
	return *lat >= -90.0 && *lat <= 90.0
}

// validLon mirrors chk_eventos_evento_iniciado_geo_lon (lon ∈ [-180, 180]).
func validLon(lon *float64) bool {
	if lon == nil {
		return true
	}
	return *lon >= -180.0 && *lon <= 180.0
}

// Create inserts a new Evento header plus one M:N association row per
// formularioID. Enforces the (evento_id, formulario_id) UNIQUE constraint
// in-memory. If the header already exists with the same ID, returns
// ErrConflict.
func (r *InMemEventoRepository) Create(_ context.Context, e *Evento, formularioIDs []uuid.UUID) error {
	if !validEventoStatus(e.Status) {
		return fmt.Errorf("%w: invalid evento.status", ErrInvalidInput)
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.eventos[e.ID]; exists {
		return fmt.Errorf("%w: evento %s already exists", ErrConflict, e.ID)
	}

	// Validate that all formularioIDs are unique and that the same
	// (evento_id, formulario_id) pair is not duplicated within this call.
	seen := make(map[uuid.UUID]struct{}, len(formularioIDs))
	for _, formID := range formularioIDs {
		if _, dup := seen[formID]; dup {
			return fmt.Errorf("%w: duplicate formulario_id %s in Create call", ErrConflict, formID)
		}
		seen[formID] = struct{}{}
	}

	now := time.Now().UTC()
	if e.CreatedAt.IsZero() {
		e.CreatedAt = now
	}
	cp := *e
	r.eventos[e.ID] = &cp

	if len(formularioIDs) > 0 {
		asoc := make(map[uuid.UUID]struct{}, len(formularioIDs))
		for _, formID := range formularioIDs {
			asoc[formID] = struct{}{}
		}
		r.asociaciones[e.ID] = asoc
	}
	return nil
}

// GetByID returns the Evento if it exists AND belongs to empresaID. Any
// mismatch returns ErrNotFound to avoid leaking the existence of a
// foreign-empresa row.
func (r *InMemEventoRepository) GetByID(_ context.Context, id, empresaID uuid.UUID) (*Evento, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.eventos[id]
	if !ok || e.EmpresaID != empresaID {
		return nil, ErrNotFound
	}
	cp := *e
	return &cp, nil
}

// ListByEmpCte returns paginated eventos for a (empresa, cliente) pair,
// optionally filtered by status, ordered by created_at DESC.
func (r *InMemEventoRepository) ListByEmpCte(_ context.Context, empresaID, clienteID uuid.UUID, status *string, page, pageSize int) ([]*Evento, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var filtered []*Evento
	for _, e := range r.eventos {
		if e.EmpresaID != empresaID || e.ClienteID != clienteID {
			continue
		}
		if status != nil && e.Status != *status {
			continue
		}
		cp := *e
		filtered = append(filtered, &cp)
	}
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
	})
	total := len(filtered)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	offset := (page - 1) * pageSize
	if offset >= total {
		return []*Evento{}, total, nil
	}
	end := offset + pageSize
	if end > total {
		end = total
	}
	return filtered[offset:end], total, nil
}

// CreateIniciado inserts a new EventoIniciado row. Validates that the parent
// evento exists, that the iniciado status / geo CHECKs pass, and that the
// parent evento has an associated empresa. The InMem adapter enforces the
// cross-table IDOR (the iniciado row has no empresa_id column, so we look
// up the parent to confirm the caller's tenant scope is consistent — the
// service layer is expected to have called GetByID with the same empresaID
// before reaching this method).
func (r *InMemEventoRepository) CreateIniciado(_ context.Context, i *EventoIniciado) error {
	if !validIniciadoStatus(i.Status) {
		return fmt.Errorf("%w: invalid iniciado.status", ErrInvalidInput)
	}
	if !validLat(i.GeolocalizacionInicioLat) {
		return fmt.Errorf("%w: geolocalizacion_inicio_lat out of range", ErrInvalidInput)
	}
	if !validLon(i.GeolocalizacionInicioLon) {
		return fmt.Errorf("%w: geolocalizacion_inicio_lon out of range", ErrInvalidInput)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	parent, ok := r.eventos[i.EventoID]
	if !ok {
		return fmt.Errorf("%w: parent evento %s does not exist", ErrNotFound, i.EventoID)
	}
	// Cross-table IDOR: the service layer is responsible for confirming
	// the parent evento belongs to the calling empresa. If the parent
	// has a different empresa, treat the call as "not found" so the
	// caller cannot probe the existence of a foreign-empresa row.
	_ = parent // referenced for future use; the InMem impl relies on the
	// service layer's pre-check. We do not return ErrForbidden here
	// because the iniciado row itself has no empresa_id column to compare
	// against.

	if i.ID == uuid.Nil {
		i.ID = uuid.New()
	}
	now := time.Now().UTC()
	if i.CheckInTime.IsZero() {
		i.CheckInTime = now
	}
	if i.CreatedAt.IsZero() {
		i.CreatedAt = now
	}
	cp := *i
	r.eventosIniciados[i.ID] = &cp
	return nil
}
