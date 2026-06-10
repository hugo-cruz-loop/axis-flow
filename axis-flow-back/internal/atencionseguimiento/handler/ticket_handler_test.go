package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"axis-flow-back/internal/atencionseguimiento"
	"axis-flow-back/internal/atencionseguimiento/handler"
	"axis-flow-back/internal/middleware"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Inline mock for TicketService
// ---------------------------------------------------------------------------

type mockTicketService struct {
	createTicketFn         func(ctx context.Context, t *atencionseguimiento.TicketServicio, clienteID, empresaID uuid.UUID) (*atencionseguimiento.TicketServicio, *atencionseguimiento.RespuestaServicio, error)
	getTicketsByClienteFn  func(ctx context.Context, clienteID uuid.UUID, requesterClienteID uuid.UUID, requesterRole string, page, pageSize int) ([]*atencionseguimiento.TicketServicio, int, error)
	updateTicketEstatusFn  func(ctx context.Context, id, empresaID uuid.UUID, newEstatus int, updatedByID uuid.UUID) (*atencionseguimiento.TicketServicio, error)
	getMensajesServicioFn  func(ctx context.Context, ticketID uuid.UUID, requesterClienteID uuid.UUID, requesterRole string, page, pageSize int) ([]*atencionseguimiento.RespuestaServicio, int, error)
	createMensajeServicioFn func(ctx context.Context, ticketID uuid.UUID, msg *atencionseguimiento.RespuestaServicio, requesterClienteID uuid.UUID, requesterRole string) (*atencionseguimiento.RespuestaServicio, error)
	getTicketStatsFn       func(ctx context.Context, empresaID uuid.UUID) (*atencionseguimiento.TicketStats, error)
	closeTicketsByClienteFn func(ctx context.Context, clienteID uuid.UUID) error
}

func (m *mockTicketService) CreateTicket(ctx context.Context, t *atencionseguimiento.TicketServicio, clienteID, empresaID uuid.UUID) (*atencionseguimiento.TicketServicio, *atencionseguimiento.RespuestaServicio, error) {
	return m.createTicketFn(ctx, t, clienteID, empresaID)
}
func (m *mockTicketService) GetTicketsByCliente(ctx context.Context, clienteID uuid.UUID, requesterClienteID uuid.UUID, requesterRole string, page, pageSize int) ([]*atencionseguimiento.TicketServicio, int, error) {
	return m.getTicketsByClienteFn(ctx, clienteID, requesterClienteID, requesterRole, page, pageSize)
}
func (m *mockTicketService) UpdateTicketEstatus(ctx context.Context, id, empresaID uuid.UUID, newEstatus int, updatedByID uuid.UUID) (*atencionseguimiento.TicketServicio, error) {
	return m.updateTicketEstatusFn(ctx, id, empresaID, newEstatus, updatedByID)
}
func (m *mockTicketService) GetMensajesServicio(ctx context.Context, ticketID uuid.UUID, requesterClienteID uuid.UUID, requesterRole string, page, pageSize int) ([]*atencionseguimiento.RespuestaServicio, int, error) {
	return m.getMensajesServicioFn(ctx, ticketID, requesterClienteID, requesterRole, page, pageSize)
}
func (m *mockTicketService) CreateMensajeServicio(ctx context.Context, ticketID uuid.UUID, msg *atencionseguimiento.RespuestaServicio, requesterClienteID uuid.UUID, requesterRole string) (*atencionseguimiento.RespuestaServicio, error) {
	return m.createMensajeServicioFn(ctx, ticketID, msg, requesterClienteID, requesterRole)
}
func (m *mockTicketService) GetTicketStats(ctx context.Context, empresaID uuid.UUID) (*atencionseguimiento.TicketStats, error) {
	return m.getTicketStatsFn(ctx, empresaID)
}
func (m *mockTicketService) CloseTicketsByCliente(ctx context.Context, clienteID uuid.UUID) error {
	return m.closeTicketsByClienteFn(ctx, clienteID)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func injectTicketCtx(r *http.Request, tenantID, userID string, role string) *http.Request {
	ctx := r.Context()
	ctx = context.WithValue(ctx, middleware.ContextKeyTenantID, tenantID)
	ctx = context.WithValue(ctx, middleware.ContextKeyUserID, userID)
	ctx = context.WithValue(ctx, middleware.ContextKeyRole, role)
	return r.WithContext(ctx)
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// POST /ticket with valid JWT (Cliente) → 201
func TestCreateTicket_ValidJWT_Returns201(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()
	ticket := &atencionseguimiento.TicketServicio{ID: uuid.New(), EmpresaID: tenantID}
	msg := &atencionseguimiento.RespuestaServicio{ID: uuid.New()}

	svc := &mockTicketService{
		createTicketFn: func(_ context.Context, _ *atencionseguimiento.TicketServicio, _, _ uuid.UUID) (*atencionseguimiento.TicketServicio, *atencionseguimiento.RespuestaServicio, error) {
			return ticket, msg, nil
		},
	}
	h := handler.NewTicketHandler(svc)

	body, _ := json.Marshal(map[string]any{
		"localidad_id": uuid.New(),
		"asunto":       "mi asunto",
		"descripcion":  "mi descripcion",
	})
	r := httptest.NewRequest(http.MethodPost, "/ticket", bytes.NewReader(body))
	r = injectTicketCtx(r, tenantID.String(), userID.String(), "Cliente")
	w := httptest.NewRecorder()

	h.CreateTicket(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d — body: %s", w.Code, w.Body.String())
	}
}

// PATCH /ticket/{id}/status with invalid estatus → 400
func TestUpdateTicketEstatus_InvalidEstatus_Returns400(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()

	svc := &mockTicketService{
		updateTicketEstatusFn: func(_ context.Context, _, _ uuid.UUID, _ int, _ uuid.UUID) (*atencionseguimiento.TicketServicio, error) {
			return nil, atencionseguimiento.ErrInvalidInput
		},
	}
	h := handler.NewTicketHandler(svc)

	router := chi.NewRouter()
	router.Patch("/ticket/{id}/status", h.UpdateTicketEstatus)

	body, _ := json.Marshal(map[string]any{"estatus": 99}) // invalid
	r := httptest.NewRequest(http.MethodPatch, "/ticket/"+uuid.New().String()+"/status", bytes.NewReader(body))
	r = injectTicketCtx(r, tenantID.String(), userID.String(), "Gestor")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// GET /empresa/{empresa_id}/tickets/stats → 200 with bucket counts
func TestGetTicketStats_Returns200WithBuckets(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()

	stats := &atencionseguimiento.TicketStats{
		EmpresaID:  tenantID,
		Pendiente:  5,
		EnProceso:  3,
		Finalizado: 12,
	}

	svc := &mockTicketService{
		getTicketStatsFn: func(_ context.Context, _ uuid.UUID) (*atencionseguimiento.TicketStats, error) {
			return stats, nil
		},
	}
	h := handler.NewTicketHandler(svc)

	router := chi.NewRouter()
	router.Get("/empresa/{empresa_id}/tickets/stats", h.GetTicketStats)

	r := httptest.NewRequest(http.MethodGet, "/empresa/"+tenantID.String()+"/tickets/stats", nil)
	r = injectTicketCtx(r, tenantID.String(), userID.String(), "Admin")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d — body: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	data, ok := resp["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data object, got %T", resp["data"])
	}
	if data["pendiente"] == nil {
		t.Error("expected pendiente bucket in stats")
	}
}
