package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"axis-flow-back/internal/bolsatrabajo"
	"axis-flow-back/internal/bolsatrabajo/handler"
	"axis-flow-back/internal/middleware"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ── mock ──────────────────────────────────────────────────────────────────────

type mockTrabajoService struct {
	createFn         func(ctx context.Context, t *bolsatrabajo.Trabajo, empresaID uuid.UUID) (*bolsatrabajo.Trabajo, error)
	activeJobsFn     func(ctx context.Context, search string, empresaID *uuid.UUID, page, pageSize int) ([]*bolsatrabajo.Trabajo, int, error)
	byEmpresaFn      func(ctx context.Context, empresaID uuid.UUID, filter, page, pageSize int) ([]*bolsatrabajo.Trabajo, int, error)
	recentFn         func(ctx context.Context, page, pageSize int) ([]*bolsatrabajo.Trabajo, int, error)
	switchEstatusFn  func(ctx context.Context, id, empresaID uuid.UUID, newEstatus int) (*bolsatrabajo.Trabajo, error)
	closeAllFn       func(ctx context.Context, empresaID uuid.UUID) error
}

func (m *mockTrabajoService) Create(ctx context.Context, t *bolsatrabajo.Trabajo, empresaID uuid.UUID) (*bolsatrabajo.Trabajo, error) {
	return m.createFn(ctx, t, empresaID)
}
func (m *mockTrabajoService) GetActiveJobs(ctx context.Context, search string, empresaID *uuid.UUID, page, pageSize int) ([]*bolsatrabajo.Trabajo, int, error) {
	return m.activeJobsFn(ctx, search, empresaID, page, pageSize)
}
func (m *mockTrabajoService) GetByEmpresa(ctx context.Context, empresaID uuid.UUID, filter, page, pageSize int) ([]*bolsatrabajo.Trabajo, int, error) {
	return m.byEmpresaFn(ctx, empresaID, filter, page, pageSize)
}
func (m *mockTrabajoService) GetRecent(ctx context.Context, page, pageSize int) ([]*bolsatrabajo.Trabajo, int, error) {
	return m.recentFn(ctx, page, pageSize)
}
func (m *mockTrabajoService) SwitchEstatus(ctx context.Context, id, empresaID uuid.UUID, newEstatus int) (*bolsatrabajo.Trabajo, error) {
	return m.switchEstatusFn(ctx, id, empresaID, newEstatus)
}
func (m *mockTrabajoService) CloseAllByEmpresa(ctx context.Context, empresaID uuid.UUID) error {
	return m.closeAllFn(ctx, empresaID)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func buildTrabajoRouter(svc handler.TrabajoServicer) http.Handler {
	r := chi.NewRouter()
	h := handler.NewTrabajoHandler(svc)
	r.Post("/bolsa-trabajo/trabajo", h.Create)
	r.Get("/bolsa-trabajo/trabajo/activeJobs", h.ActiveJobs)
	r.Get("/bolsa-trabajo/trabajo/by-empresa/{id}", h.ByEmpresa)
	r.Get("/bolsa-trabajo/trabajo/recent", h.Recent)
	r.Patch("/bolsa-trabajo/trabajo/switch/{id}", h.SwitchEstatus)
	return r
}

func withTenant(r *http.Request, id uuid.UUID) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.ContextKeyTenantID, id.String())
	return r.WithContext(ctx)
}

// ── tests ─────────────────────────────────────────────────────────────────────

// POST no JWT → 401
func TestCreate_NoJWT(t *testing.T) {
	svc := &mockTrabajoService{}
	router := buildTrabajoRouter(svc)

	body, _ := json.Marshal(map[string]any{
		"titulo":          "Dev",
		"descripcion":     "desc",
		"fecha_caducar":   time.Now().Add(24 * time.Hour).Format(time.RFC3339),
		"estatus_vacante": 1,
	})
	req := httptest.NewRequest(http.MethodPost, "/bolsa-trabajo/trabajo", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

// POST past fechaCaducar → 400
func TestCreate_PastFechaCaducar(t *testing.T) {
	svc := &mockTrabajoService{
		createFn: func(_ context.Context, _ *bolsatrabajo.Trabajo, _ uuid.UUID) (*bolsatrabajo.Trabajo, error) {
			return nil, errors.New("past date: " + bolsatrabajo.ErrNotFound.Error())
		},
	}
	router := buildTrabajoRouter(svc)

	tenantID := uuid.New()
	body, _ := json.Marshal(map[string]any{
		"titulo":          "Dev",
		"descripcion":     "desc",
		"fecha_caducar":   time.Now().Add(-24 * time.Hour).Format(time.RFC3339),
		"estatus_vacante": 1,
	})
	req := httptest.NewRequest(http.MethodPost, "/bolsa-trabajo/trabajo", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withTenant(req, tenantID)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// POST valid → 201 with requestId in meta
func TestCreate_Valid(t *testing.T) {
	created := &bolsatrabajo.Trabajo{
		ID:             uuid.New(),
		EmpresaID:      uuid.New(),
		Titulo:         "Dev",
		Descripcion:    "desc",
		EstatusVacante: 1,
		FechaCaducar:   time.Now().Add(48 * time.Hour),
	}
	svc := &mockTrabajoService{
		createFn: func(_ context.Context, _ *bolsatrabajo.Trabajo, _ uuid.UUID) (*bolsatrabajo.Trabajo, error) {
			return created, nil
		},
	}
	router := buildTrabajoRouter(svc)

	tenantID := uuid.New()
	body, _ := json.Marshal(map[string]any{
		"titulo":          "Dev",
		"descripcion":     "desc",
		"fecha_caducar":   time.Now().Add(48 * time.Hour).Format(time.RFC3339),
		"estatus_vacante": 1,
	})
	req := httptest.NewRequest(http.MethodPost, "/bolsa-trabajo/trabajo", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withTenant(req, tenantID)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d — body: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	meta, ok := resp["meta"].(map[string]any)
	if !ok {
		t.Fatalf("expected meta object in response, got: %v", resp)
	}
	if meta["requestId"] == "" || meta["requestId"] == nil {
		t.Fatalf("expected requestId in meta, got: %v", meta)
	}
	_ = io.Discard
}
