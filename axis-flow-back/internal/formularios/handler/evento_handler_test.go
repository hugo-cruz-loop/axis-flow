// Package handler_test — evento handler tests (PR-4 task 4.2).
//
// Mirrors internal/atencionseguimiento/handler/queja_handler_test.go.
package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"axis-flow-back/internal/formularios"
	formshandler "axis-flow-back/internal/formularios/handler"
	"axis-flow-back/internal/middleware"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// mockEventoService — implements service.EventoService.
// ---------------------------------------------------------------------------

type mockEventoService struct {
	createEventoFn     func(ctx context.Context, e *formularios.Evento, formularioIDs []uuid.UUID, empresaID uuid.UUID) (*formularios.Evento, error)
	getEventoFn        func(ctx context.Context, id, empresaID uuid.UUID) (*formularios.Evento, error)
	getEventosByEmpCteFn func(ctx context.Context, empresaID, clienteID uuid.UUID, status *string, page, pageSize int) ([]*formularios.Evento, int, error)
	iniciarEventoFn    func(ctx context.Context, ei *formularios.EventoIniciado, empresaID uuid.UUID) (*formularios.EventoIniciado, error)
}

func (m *mockEventoService) CreateEvento(ctx context.Context, e *formularios.Evento, formularioIDs []uuid.UUID, empresaID uuid.UUID) (*formularios.Evento, error) {
	return m.createEventoFn(ctx, e, formularioIDs, empresaID)
}
func (m *mockEventoService) GetEvento(ctx context.Context, id, empresaID uuid.UUID) (*formularios.Evento, error) {
	return m.getEventoFn(ctx, id, empresaID)
}
func (m *mockEventoService) GetEventosByEmpCte(ctx context.Context, empresaID, clienteID uuid.UUID, status *string, page, pageSize int) ([]*formularios.Evento, int, error) {
	return m.getEventosByEmpCteFn(ctx, empresaID, clienteID, status, page, pageSize)
}
func (m *mockEventoService) IniciarEvento(ctx context.Context, ei *formularios.EventoIniciado, empresaID uuid.UUID) (*formularios.EventoIniciado, error) {
	return m.iniciarEventoFn(ctx, ei, empresaID)
}

// eventoRouter mounts the three evento routes on a chi router.
func eventoRouter(h *formshandler.EventoHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Post("/evento", h.CreateEvento)
	r.Post("/evento_iniciado", h.IniciarEvento)
	r.Get("/evento/byEmpId/{empId}/{cteId}", h.GetEventosByEmpCte)
	return r
}

// ---------------------------------------------------------------------------
// POST /evento — CreateEvento.
// ---------------------------------------------------------------------------

func TestCreateEvento_NoJWT_Returns401(t *testing.T) {
	svc := &mockEventoService{}
	h := formshandler.NewEventoHandler(svc)

	body, _ := json.Marshal(map[string]any{
		"empresa_id":       uuid.New(),
		"cliente_id":       uuid.New(),
		"nombre":           "Ronda",
		"fecha_programada": "2026-06-10T22:00:00Z",
	})
	r := httptest.NewRequest(http.MethodPost, "/evento", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.CreateEvento(w, r)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestCreateEvento_EmptyNombre_Returns422(t *testing.T) {
	svc := &mockEventoService{}
	h := formshandler.NewEventoHandler(svc)

	tenantID := uuid.New()
	userID := uuid.New()
	body, _ := json.Marshal(map[string]any{
		"empresa_id":       tenantID,
		"cliente_id":       uuid.New(),
		"nombre":           "",
		"fecha_programada": "2026-06-10T22:00:00Z",
	})
	r := httptest.NewRequest(http.MethodPost, "/evento", bytes.NewReader(body))
	r = injectFormulariosCtx(r, tenantID.String(), userID.String(), 1, "Manager")
	w := httptest.NewRecorder()
	h.CreateEvento(w, r)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestCreateEvento_MissingClienteID_Returns422(t *testing.T) {
	svc := &mockEventoService{}
	h := formshandler.NewEventoHandler(svc)

	tenantID := uuid.New()
	userID := uuid.New()
	body, _ := json.Marshal(map[string]any{
		"empresa_id":       tenantID,
		"nombre":           "Ronda",
		"fecha_programada": "2026-06-10T22:00:00Z",
		// cliente_id is missing → uuid.Nil
	})
	r := httptest.NewRequest(http.MethodPost, "/evento", bytes.NewReader(body))
	r = injectFormulariosCtx(r, tenantID.String(), userID.String(), 1, "Manager")
	w := httptest.NewRecorder()
	h.CreateEvento(w, r)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestCreateEvento_TenantMismatch_Returns403(t *testing.T) {
	svc := &mockEventoService{
		createEventoFn: func(_ context.Context, _ *formularios.Evento, _ []uuid.UUID, _ uuid.UUID) (*formularios.Evento, error) {
			t.Fatal("service must not be called when tenant mismatches")
			return nil, nil
		},
	}
	h := formshandler.NewEventoHandler(svc)

	jwtTenant := uuid.New()
	bodyTenant := uuid.New()
	userID := uuid.New()
	body, _ := json.Marshal(map[string]any{
		"empresa_id":       bodyTenant,
		"cliente_id":       uuid.New(),
		"nombre":           "Ronda",
		"fecha_programada": "2026-06-10T22:00:00Z",
	})
	r := httptest.NewRequest(http.MethodPost, "/evento", bytes.NewReader(body))
	r = injectFormulariosCtx(r, jwtTenant.String(), userID.String(), 1, "Manager")
	w := httptest.NewRecorder()
	h.CreateEvento(w, r)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestCreateEvento_ValidRequest_Returns201WithData(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()
	clienteID := uuid.New()
	created := &formularios.Evento{
		ID:              uuid.New(),
		EmpresaID:       tenantID,
		ClienteID:       clienteID,
		Nombre:          "Ronda Nocturna",
		FechaProgramada: time.Date(2026, 6, 10, 22, 0, 0, 0, time.UTC),
		Status:          formularios.EventoStatusPendiente,
	}
	svc := &mockEventoService{
		createEventoFn: func(_ context.Context, e *formularios.Evento, formulariosAsoc []uuid.UUID, _ uuid.UUID) (*formularios.Evento, error) {
			require.Equal(t, tenantID, e.EmpresaID, "handler must propagate JWT tenant as EmpresaID")
			require.Equal(t, clienteID, e.ClienteID)
			require.Equal(t, "Ronda Nocturna", e.Nombre)
			require.Len(t, formulariosAsoc, 1, "handler must forward formularios_asociados")
			return created, nil
		},
	}
	h := formshandler.NewEventoHandler(svc)

	formID := uuid.New()
	body, _ := json.Marshal(map[string]any{
		"empresa_id":            tenantID,
		"cliente_id":            clienteID,
		"nombre":                "Ronda Nocturna",
		"fecha_programada":      "2026-06-10T22:00:00Z",
		"formularios_asociados": []string{formID.String()},
	})
	r := httptest.NewRequest(http.MethodPost, "/evento", bytes.NewReader(body))
	r = injectFormulariosCtx(r, tenantID.String(), userID.String(), 1, "Manager")
	w := httptest.NewRecorder()
	h.CreateEvento(w, r)

	require.Equal(t, http.StatusCreated, w.Code, "happy path → 201, body: %s", w.Body.String())
	var env map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
	require.Equal(t, true, env["success"])
	data, ok := env["data"].(map[string]any)
	require.True(t, ok, "expected data object, got %T", env["data"])
	assert.Equal(t, created.ID.String(), data["id"])
	assert.Equal(t, tenantID.String(), data["empresa_id"])
	assert.Equal(t, clienteID.String(), data["cliente_id"])
	assert.Equal(t, "Ronda Nocturna", data["nombre"])
}

// ---------------------------------------------------------------------------
// POST /evento_iniciado — IniciarEvento.
// ---------------------------------------------------------------------------

func TestIniciarEvento_NoJWT_Returns401(t *testing.T) {
	svc := &mockEventoService{}
	h := formshandler.NewEventoHandler(svc)

	body, _ := json.Marshal(map[string]any{
		"evento_id":   uuid.New(),
		"empleado_id": 99,
	})
	r := httptest.NewRequest(http.MethodPost, "/evento_iniciado", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.IniciarEvento(w, r)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestIniciarEvento_MissingEmpleadoID_Returns401(t *testing.T) {
	// PR-4 AMEND (FIX 5): the handler MUST require the empleado_id
	// claim from the JWT context. If the claim is absent OR zero, the
	// handler short-circuits with 401 UNAUTHORIZED "missing empleado
	// claim" — body fallback is no longer accepted.
	svc := &mockEventoService{}
	h := formshandler.NewEventoHandler(svc)

	tenantID := uuid.New()
	userID := uuid.New()
	body, _ := json.Marshal(map[string]any{
		"evento_id": uuid.New(),
		// empleado_id is missing in body
	})
	r := httptest.NewRequest(http.MethodPost, "/evento_iniciado", bytes.NewReader(body))
	// JWT has no empleadoID either (pass 0 to injectFormulariosCtx).
	r = injectFormulariosCtx(r, tenantID.String(), userID.String(), 0, "Empleado")
	w := httptest.NewRecorder()
	h.IniciarEvento(w, r)

	assert.Equal(t, http.StatusUnauthorized, w.Code,
		"missing/zero JWT empleado_id → 401 UNAUTHORIZED, body: %s", w.Body.String())
	assert.Contains(t, w.Body.String(), `"UNAUTHORIZED"`)
	assert.Contains(t, w.Body.String(), "missing empleado claim")
}

// TestIniciarEvento_BodyEmpleadoIDOverriddenByJWT verifies FIX 5's
// invariant: the body field is IGNORED. The service receives the
// JWT's empleado_id, never the body's.
func TestIniciarEvento_BodyEmpleadoIDOverriddenByJWT(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()
	const jwtEmpleadoID int64 = 42
	const bodyEmpleadoID int64 = 99 // would-be attacker tries to spoof

	svc := &mockEventoService{
		iniciarEventoFn: func(_ context.Context, ei *formularios.EventoIniciado, _ uuid.UUID) (*formularios.EventoIniciado, error) {
			require.Equal(t, jwtEmpleadoID, ei.EmpleadoID,
				"service MUST receive the JWT empleado_id (%d), NOT the body's (%d) — FIX 5 anti-spoof",
				jwtEmpleadoID, bodyEmpleadoID)
			ei.ID = uuid.New()
			return ei, nil
		},
	}
	h := formshandler.NewEventoHandler(svc)

	body, _ := json.Marshal(map[string]any{
		"evento_id":   uuid.New(),
		"empleado_id": bodyEmpleadoID, // attacker value
	})
	r := httptest.NewRequest(http.MethodPost, "/evento_iniciado", bytes.NewReader(body))
	// JWT sets a different empleado_id.
	r = injectFormulariosCtx(r, tenantID.String(), userID.String(), jwtEmpleadoID, "Empleado")
	w := httptest.NewRecorder()
	h.IniciarEvento(w, r)

	require.Equal(t, http.StatusCreated, w.Code, "happy path → 201, body: %s", w.Body.String())
}

// TestIniciarEvento_MissingJWTClaim_Returns401 verifies FIX 5's hard
// requirement: when the JWT context has no empleado_id claim (not
// even 0), the handler returns 401 — regardless of any body field
// that might be present.
func TestIniciarEvento_MissingJWTClaim_Returns401(t *testing.T) {
	svc := &mockEventoService{
		iniciarEventoFn: func(_ context.Context, _ *formularios.EventoIniciado, _ uuid.UUID) (*formularios.EventoIniciado, error) {
			t.Fatal("service must NOT be called when JWT has no empleado_id claim")
			return nil, nil
		},
	}
	h := formshandler.NewEventoHandler(svc)

	tenantID := uuid.New()
	userID := uuid.New()
	body, _ := json.Marshal(map[string]any{
		"evento_id":   uuid.New(),
		"empleado_id": 99, // body has a value, but the JWT doesn't
	})
	r2 := httptest.NewRequest(http.MethodPost, "/evento_iniciado", bytes.NewReader(body))
	r2 = r2.WithContext(context.WithValue(r2.Context(), middleware.ContextKeyTenantID, tenantID.String()))
	r2 = r2.WithContext(context.WithValue(r2.Context(), middleware.ContextKeyUserID, userID.String()))
	r2 = r2.WithContext(context.WithValue(r2.Context(), middleware.ContextKeyRole, "Empleado"))
	// Deliberately no ContextKeyEmpleadoID.

	w := httptest.NewRecorder()
	h.IniciarEvento(w, r2)

	assert.Equal(t, http.StatusUnauthorized, w.Code,
		"missing JWT empleado_id claim → 401 regardless of body field")
}

func TestIniciarEvento_InvalidGeoLat_Returns422(t *testing.T) {
	svc := &mockEventoService{}
	h := formshandler.NewEventoHandler(svc)

	tenantID := uuid.New()
	userID := uuid.New()
	body, _ := json.Marshal(map[string]any{
		"evento_id":   uuid.New(),
		"empleado_id": 99,
		"geolocalizacion_inicio": map[string]any{
			"latitud":  999.0, // out of range
			"longitud": -58.3816,
		},
	})
	r := httptest.NewRequest(http.MethodPost, "/evento_iniciado", bytes.NewReader(body))
	r = injectFormulariosCtx(r, tenantID.String(), userID.String(), 1, "Empleado")
	w := httptest.NewRecorder()
	h.IniciarEvento(w, r)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code, "lat out of range → 422")
}

func TestIniciarEvento_ServiceErrNotFound_Returns404(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()
	svc := &mockEventoService{
		iniciarEventoFn: func(_ context.Context, _ *formularios.EventoIniciado, _ uuid.UUID) (*formularios.EventoIniciado, error) {
			return nil, formularios.ErrNotFound
		},
	}
	h := formshandler.NewEventoHandler(svc)

	body, _ := json.Marshal(map[string]any{
		"evento_id":   uuid.New(),
		"empleado_id": 99,
	})
	r := httptest.NewRequest(http.MethodPost, "/evento_iniciado", bytes.NewReader(body))
	r = injectFormulariosCtx(r, tenantID.String(), userID.String(), 1, "Empleado")
	w := httptest.NewRecorder()
	h.IniciarEvento(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), `"NOT_FOUND"`)
}

func TestIniciarEvento_ValidRequest_Returns201WithData(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()
	empleadoID := int64(42)
	created := &formularios.EventoIniciado{
		ID:          uuid.New(),
		EventoID:    uuid.New(),
		EmpleadoID:  empleadoID,
		CheckInTime: time.Date(2026, 6, 10, 8, 0, 0, 0, time.UTC),
		Status:      formularios.IniciadoStatus,
	}
	lat, lon := -34.6037, -58.3816
	svc := &mockEventoService{
		iniciarEventoFn: func(_ context.Context, ei *formularios.EventoIniciado, _ uuid.UUID) (*formularios.EventoIniciado, error) {
			require.Equal(t, empleadoID, ei.EmpleadoID)
			require.NotNil(t, ei.GeolocalizacionInicioLat, "handler must forward lat")
			require.Equal(t, lat, *ei.GeolocalizacionInicioLat)
			require.Equal(t, lon, *ei.GeolocalizacionInicioLon)
			return created, nil
		},
	}
	h := formshandler.NewEventoHandler(svc)

	body, _ := json.Marshal(map[string]any{
		"evento_id":   created.EventoID,
		"empleado_id": empleadoID,
		"geolocalizacion_inicio": map[string]any{
			"latitud":  lat,
			"longitud": lon,
		},
	})
	r := httptest.NewRequest(http.MethodPost, "/evento_iniciado", bytes.NewReader(body))
	r = injectFormulariosCtx(r, tenantID.String(), userID.String(), empleadoID, "Empleado")
	w := httptest.NewRecorder()
	h.IniciarEvento(w, r)

	require.Equal(t, http.StatusCreated, w.Code, "happy path → 201, body: %s", w.Body.String())
	var env map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
	require.Equal(t, true, env["success"])
	data, ok := env["data"].(map[string]any)
	require.True(t, ok, "expected data object, got %T", env["data"])
	assert.Equal(t, created.ID.String(), data["id"])
	assert.Equal(t, created.EventoID.String(), data["evento_id"])
	assert.EqualValues(t, empleadoID, data["empleado_id"])
	assert.Equal(t, formularios.IniciadoStatus, data["estatus"])
}

// ---------------------------------------------------------------------------
// GET /evento/byEmpId/{empId}/{cteId} — GetEventosByEmpCte.
// ---------------------------------------------------------------------------

func TestGetEventosByEmpCte_NoJWT_Returns401(t *testing.T) {
	svc := &mockEventoService{}
	h := formshandler.NewEventoHandler(svc)
	router := eventoRouter(h)

	r := httptest.NewRequest(http.MethodGet, "/evento/byEmpId/"+uuid.New().String()+"/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestGetEventosByEmpCte_TenantMismatch_Returns403(t *testing.T) {
	svc := &mockEventoService{
		getEventosByEmpCteFn: func(_ context.Context, _, _ uuid.UUID, _ *string, _, _ int) ([]*formularios.Evento, int, error) {
			t.Fatal("service must not be called when tenant mismatches")
			return nil, 0, nil
		},
	}
	h := formshandler.NewEventoHandler(svc)
	router := eventoRouter(h)

	jwtTenant := uuid.New()
	pathEmp := uuid.New() // different
	cte := uuid.New()
	userID := uuid.New()
	r := httptest.NewRequest(http.MethodGet, "/evento/byEmpId/"+pathEmp.String()+"/"+cte.String(), nil)
	r = injectFormulariosCtx(r, jwtTenant.String(), userID.String(), 1, "Empleado")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestGetEventosByEmpCte_ValidRequest_Returns200Paginated(t *testing.T) {
	tenantID := uuid.New()
	clienteID := uuid.New()
	userID := uuid.New()
	items := []*formularios.Evento{
		{ID: uuid.New(), EmpresaID: tenantID, ClienteID: clienteID, Nombre: "E1", Status: formularios.EventoStatusPendiente},
	}
	svc := &mockEventoService{
		getEventosByEmpCteFn: func(_ context.Context, _ uuid.UUID, _ uuid.UUID, status *string, _, _ int) ([]*formularios.Evento, int, error) {
			require.NotNil(t, status, "handler must forward estatus query")
			require.Equal(t, formularios.EventoStatusPendiente, *status)
			return items, 1, nil
		},
	}
	h := formshandler.NewEventoHandler(svc)
	router := eventoRouter(h)

	r := httptest.NewRequest(http.MethodGet, "/evento/byEmpId/"+tenantID.String()+"/"+clienteID.String()+"?estatus=pendiente&page=1&limit=5", nil)
	r = injectFormulariosCtx(r, tenantID.String(), userID.String(), 1, "Empleado")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	require.Equal(t, http.StatusOK, w.Code, "happy path → 200, body: %s", w.Body.String())
	var env map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
	data, ok := env["data"].([]any)
	require.True(t, ok, "expected data array, got %T", env["data"])
	assert.Len(t, data, 1)
	meta, ok := env["meta"].(map[string]any)
	require.True(t, ok)
	assert.EqualValues(t, 1, meta["page"])
	assert.EqualValues(t, 5, meta["limit"])
	assert.EqualValues(t, 1, meta["total_records"])
}

func TestGetEventosByEmpCte_InvalidUUID_Returns422(t *testing.T) {
	svc := &mockEventoService{}
	h := formshandler.NewEventoHandler(svc)
	router := eventoRouter(h)

	tenantID := uuid.New()
	userID := uuid.New()
	r := httptest.NewRequest(http.MethodGet, "/evento/byEmpId/not-a-uuid/"+uuid.New().String(), nil)
	r = injectFormulariosCtx(r, tenantID.String(), userID.String(), 1, "Empleado")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}
