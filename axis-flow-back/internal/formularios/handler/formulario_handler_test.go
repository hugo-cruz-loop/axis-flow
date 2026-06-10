// Package handler_test — formulario handler tests (PR-4 task 4.1).
//
// Mirrors internal/atencionseguimiento/handler/queja_handler_test.go (PR-4
// of 09_AtencionSeguimiento_Service_Spec). The test harness uses an
// inline mock service + httptest.NewRecorder + http.NewRequest with JWT
// claims injected via context.WithValue so the handler is exercised
// without a real HTTP server, JWT signer, or DB.
//
// Each test asserts a SPECIFIC behaviour:
//   - Status code (401 / 403 / 422 / 201 / 200)
//   - JSON envelope shape (success:bool, data:object, meta:object on lists)
//   - Service method invocation (via the mock's call recorder)
package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"axis-flow-back/internal/formularios"
	formshandler "axis-flow-back/internal/formularios/handler"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// formularioRouter mounts the three formulario routes on a chi router so
// chi.URLParam can resolve the path parameter `{id}`.
func formularioRouter(h *formshandler.FormularioHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Post("/formulario", h.CreateFormulario)
	r.Get("/formulario/byempresa/{id}", h.GetFormulariosByEmpresa)
	r.Post("/pregunta", h.CreatePregunta)
	return r
}

// ---------------------------------------------------------------------------
// mockFormularioService — implements service.FormularioService.
// ---------------------------------------------------------------------------

type mockFormularioService struct {
	createFormularioFn       func(ctx context.Context, f *formularios.Formulario, empresaID uuid.UUID) (*formularios.Formulario, error)
	getFormulariosByEmpresaFn func(ctx context.Context, empresaID uuid.UUID, activo *bool, page, pageSize int) ([]*formularios.Formulario, int, error)
	addPreguntaFn            func(ctx context.Context, p *formularios.Pregunta, empresaID uuid.UUID) (*formularios.Pregunta, error)
}

func (m *mockFormularioService) CreateFormulario(ctx context.Context, f *formularios.Formulario, empresaID uuid.UUID) (*formularios.Formulario, error) {
	return m.createFormularioFn(ctx, f, empresaID)
}
func (m *mockFormularioService) GetFormulariosByEmpresa(ctx context.Context, empresaID uuid.UUID, activo *bool, page, pageSize int) ([]*formularios.Formulario, int, error) {
	return m.getFormulariosByEmpresaFn(ctx, empresaID, activo, page, pageSize)
}
func (m *mockFormularioService) AddPregunta(ctx context.Context, p *formularios.Pregunta, empresaID uuid.UUID) (*formularios.Pregunta, error) {
	return m.addPreguntaFn(ctx, p, empresaID)
}

// ---------------------------------------------------------------------------
// POST /formulario — CreateFormulario.
// ---------------------------------------------------------------------------

func TestCreateFormulario_NoJWT_Returns401(t *testing.T) {
	svc := &mockFormularioService{}
	h := formshandler.NewFormularioHandler(svc)

	body, _ := json.Marshal(map[string]any{
		"empresa_id": uuid.New(),
		"nombre":     "Test",
		"activo":     true,
	})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/formularios/formulario", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.CreateFormulario(w, r)

	require.Equal(t, http.StatusUnauthorized, w.Code, "missing tenant → 401")
	assert.Contains(t, w.Body.String(), `"success":false`)
}

func TestCreateFormulario_InvalidJSON_Returns422(t *testing.T) {
	svc := &mockFormularioService{}
	h := formshandler.NewFormularioHandler(svc)

	tenantID := uuid.New()
	userID := uuid.New()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/formularios/formulario", bytes.NewReader([]byte("{not-json")))
	r = injectFormulariosCtx(r, tenantID.String(), userID.String(), 1, "Admin")
	w := httptest.NewRecorder()

	h.CreateFormulario(w, r)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code, "bad JSON → 422")
	assert.Contains(t, w.Body.String(), `"VALIDATION_ERROR"`)
}

func TestCreateFormulario_EmptyNombre_Returns422(t *testing.T) {
	svc := &mockFormularioService{}
	h := formshandler.NewFormularioHandler(svc)

	tenantID := uuid.New()
	userID := uuid.New()
	body, _ := json.Marshal(map[string]any{
		"empresa_id": tenantID,
		"nombre":     "",
		"activo":     true,
	})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/formularios/formulario", bytes.NewReader(body))
	r = injectFormulariosCtx(r, tenantID.String(), userID.String(), 1, "Admin")
	w := httptest.NewRecorder()

	h.CreateFormulario(w, r)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code, "empty nombre → 422")
}

func TestCreateFormulario_NombreOverLimit_Returns422(t *testing.T) {
	svc := &mockFormularioService{}
	h := formshandler.NewFormularioHandler(svc)

	tenantID := uuid.New()
	userID := uuid.New()
	long := make([]byte, 151)
	for i := range long {
		long[i] = 'a'
	}
	body, _ := json.Marshal(map[string]any{
		"empresa_id": tenantID,
		"nombre":     string(long),
		"activo":     true,
	})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/formularios/formulario", bytes.NewReader(body))
	r = injectFormulariosCtx(r, tenantID.String(), userID.String(), 1, "Admin")
	w := httptest.NewRecorder()

	h.CreateFormulario(w, r)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code, "nombre over 150 chars → 422")
}

func TestCreateFormulario_TenantMismatch_Returns403(t *testing.T) {
	// Handler is the trust boundary: it MUST reject a body that
	// specifies a different empresa_id than the JWT's tenant.
	svc := &mockFormularioService{
		createFormularioFn: func(_ context.Context, _ *formularios.Formulario, _ uuid.UUID) (*formularios.Formulario, error) {
			t.Fatal("service must not be called when tenant mismatches")
			return nil, nil
		},
	}
	h := formshandler.NewFormularioHandler(svc)

	jwtTenant := uuid.New()
	bodyTenant := uuid.New()
	userID := uuid.New()
	body, _ := json.Marshal(map[string]any{
		"empresa_id": bodyTenant,
		"nombre":     "x",
		"activo":     true,
	})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/formularios/formulario", bytes.NewReader(body))
	r = injectFormulariosCtx(r, jwtTenant.String(), userID.String(), 1, "Admin")
	w := httptest.NewRecorder()

	h.CreateFormulario(w, r)

	assert.Equal(t, http.StatusForbidden, w.Code, "tenant mismatch → 403")
}

func TestCreateFormulario_ServiceErrInvalidInput_Returns422(t *testing.T) {
	svc := &mockFormularioService{
		createFormularioFn: func(_ context.Context, _ *formularios.Formulario, _ uuid.UUID) (*formularios.Formulario, error) {
			return nil, formularios.ErrInvalidInput
		},
	}
	h := formshandler.NewFormularioHandler(svc)

	tenantID := uuid.New()
	userID := uuid.New()
	body, _ := json.Marshal(map[string]any{
		"empresa_id": tenantID,
		"nombre":     "x",
		"activo":     true,
	})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/formularios/formulario", bytes.NewReader(body))
	r = injectFormulariosCtx(r, tenantID.String(), userID.String(), 1, "Admin")
	w := httptest.NewRecorder()

	h.CreateFormulario(w, r)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.Contains(t, w.Body.String(), `"VALIDATION_ERROR"`)
}

func TestCreateFormulario_ValidRequest_Returns201WithData(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()
	created := &formularios.Formulario{
		ID:        uuid.New(),
		EmpresaID: tenantID,
		Nombre:    "Control Higienico",
		Activo:    true,
	}
	svc := &mockFormularioService{
		createFormularioFn: func(_ context.Context, f *formularios.Formulario, _ uuid.UUID) (*formularios.Formulario, error) {
			require.Equal(t, tenantID, f.EmpresaID, "handler must propagate JWT tenant as EmpresaID")
			require.Equal(t, "Control Higienico", f.Nombre)
			return created, nil
		},
	}
	h := formshandler.NewFormularioHandler(svc)

	body, _ := json.Marshal(map[string]any{
		"empresa_id": tenantID,
		"nombre":     "Control Higienico",
		"activo":     true,
	})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/formularios/formulario", bytes.NewReader(body))
	r = injectFormulariosCtx(r, tenantID.String(), userID.String(), 1, "Admin")
	w := httptest.NewRecorder()

	h.CreateFormulario(w, r)

	require.Equal(t, http.StatusCreated, w.Code, "happy path → 201, body: %s", w.Body.String())
	var env map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
	require.Equal(t, true, env["success"])
	data, ok := env["data"].(map[string]any)
	require.True(t, ok, "expected data object, got %T", env["data"])
	assert.Equal(t, created.ID.String(), data["id"])
	assert.Equal(t, tenantID.String(), data["empresa_id"])
	assert.Equal(t, "Control Higienico", data["nombre"])
}

// ---------------------------------------------------------------------------
// GET /formulario/byempresa/{id} — GetFormulariosByEmpresa.
// ---------------------------------------------------------------------------

func TestGetFormulariosByEmpresa_NoJWT_Returns401(t *testing.T) {
	svc := &mockFormularioService{}
	h := formshandler.NewFormularioHandler(svc)
	router := formularioRouter(h)

	r := httptest.NewRequest(http.MethodGet, "/formulario/byempresa/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestGetFormulariosByEmpresa_InvalidUUID_Returns422(t *testing.T) {
	svc := &mockFormularioService{}
	h := formshandler.NewFormularioHandler(svc)
	router := formularioRouter(h)

	tenantID := uuid.New()
	userID := uuid.New()
	r := httptest.NewRequest(http.MethodGet, "/formulario/byempresa/not-a-uuid", nil)
	r = injectFormulariosCtx(r, tenantID.String(), userID.String(), 1, "Admin")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestGetFormulariosByEmpresa_ValidRequest_Returns200Paginated(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()
	empresaInPath := tenantID
	items := []*formularios.Formulario{
		{ID: uuid.New(), EmpresaID: tenantID, Nombre: "F1", Activo: true},
		{ID: uuid.New(), EmpresaID: tenantID, Nombre: "F2", Activo: true},
	}
	svc := &mockFormularioService{
		getFormulariosByEmpresaFn: func(_ context.Context, _ uuid.UUID, _ *bool, _, _ int) ([]*formularios.Formulario, int, error) {
			return items, 2, nil
		},
	}
	h := formshandler.NewFormularioHandler(svc)
	router := formularioRouter(h)

	r := httptest.NewRequest(http.MethodGet, "/formulario/byempresa/"+empresaInPath.String()+"?page=1&limit=10", nil)
	r = injectFormulariosCtx(r, tenantID.String(), userID.String(), 1, "Admin")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	require.Equal(t, http.StatusOK, w.Code, "happy path → 200, body: %s", w.Body.String())
	var env map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
	require.Equal(t, true, env["success"])
	data, ok := env["data"].([]any)
	require.True(t, ok, "expected data array, got %T", env["data"])
	assert.Len(t, data, 2)
	meta, ok := env["meta"].(map[string]any)
	require.True(t, ok, "expected meta object")
	assert.EqualValues(t, 1, meta["page"])
	assert.EqualValues(t, 10, meta["limit"])
	assert.EqualValues(t, 2, meta["total_records"])
}

func TestGetFormulariosByEmpresa_TenantMismatch_Returns403(t *testing.T) {
	// The path empresa_id must equal the JWT tenant — IDOR check.
	svc := &mockFormularioService{
		getFormulariosByEmpresaFn: func(_ context.Context, _ uuid.UUID, _ *bool, _, _ int) ([]*formularios.Formulario, int, error) {
			t.Fatal("service must not be called when tenant mismatches")
			return nil, 0, nil
		},
	}
	h := formshandler.NewFormularioHandler(svc)
	router := formularioRouter(h)

	jwtTenant := uuid.New()
	pathTenant := uuid.New() // different from JWT
	userID := uuid.New()
	r := httptest.NewRequest(http.MethodGet, "/formulario/byempresa/"+pathTenant.String(), nil)
	r = injectFormulariosCtx(r, jwtTenant.String(), userID.String(), 1, "Admin")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestGetFormulariosByEmpresa_ServiceErrForbidden_Returns403(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()
	svc := &mockFormularioService{
		getFormulariosByEmpresaFn: func(_ context.Context, _ uuid.UUID, _ *bool, _, _ int) ([]*formularios.Formulario, int, error) {
			return nil, 0, formularios.ErrForbidden
		},
	}
	h := formshandler.NewFormularioHandler(svc)
	router := formularioRouter(h)

	r := httptest.NewRequest(http.MethodGet, "/formulario/byempresa/"+tenantID.String(), nil)
	r = injectFormulariosCtx(r, tenantID.String(), userID.String(), 1, "Admin")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

// ---------------------------------------------------------------------------
// POST /pregunta — CreatePregunta.
// ---------------------------------------------------------------------------

func TestCreatePregunta_NoJWT_Returns401(t *testing.T) {
	svc := &mockFormularioService{}
	h := formshandler.NewFormularioHandler(svc)

	body, _ := json.Marshal(map[string]any{
		"formulario_id":  uuid.New(),
		"orden":          1,
		"texto_pregunta": "x",
		"tipo_pregunta":  1,
		"obligatoria":    true,
	})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/formularios/pregunta", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.CreatePregunta(w, r)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestCreatePregunta_InvalidTipo_Returns422(t *testing.T) {
	svc := &mockFormularioService{}
	h := formshandler.NewFormularioHandler(svc)

	tenantID := uuid.New()
	userID := uuid.New()
	body, _ := json.Marshal(map[string]any{
		"formulario_id":  uuid.New(),
		"orden":          1,
		"texto_pregunta": "x",
		"tipo_pregunta":  99, // outside the SQL CHECK set
		"obligatoria":    true,
	})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/formularios/pregunta", bytes.NewReader(body))
	r = injectFormulariosCtx(r, tenantID.String(), userID.String(), 1, "Admin")
	w := httptest.NewRecorder()

	h.CreatePregunta(w, r)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestCreatePregunta_OrdenZero_Returns422(t *testing.T) {
	svc := &mockFormularioService{}
	h := formshandler.NewFormularioHandler(svc)

	tenantID := uuid.New()
	userID := uuid.New()
	body, _ := json.Marshal(map[string]any{
		"formulario_id":  uuid.New(),
		"orden":          0,
		"texto_pregunta": "x",
		"tipo_pregunta":  1,
		"obligatoria":    true,
	})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/formularios/pregunta", bytes.NewReader(body))
	r = injectFormulariosCtx(r, tenantID.String(), userID.String(), 1, "Admin")
	w := httptest.NewRecorder()

	h.CreatePregunta(w, r)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code, "orden < 1 → 422")
}

func TestCreatePregunta_EmptyTexto_Returns422(t *testing.T) {
	svc := &mockFormularioService{}
	h := formshandler.NewFormularioHandler(svc)

	tenantID := uuid.New()
	userID := uuid.New()
	body, _ := json.Marshal(map[string]any{
		"formulario_id":  uuid.New(),
		"orden":          1,
		"texto_pregunta": "   ",
		"tipo_pregunta":  1,
		"obligatoria":    true,
	})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/formularios/pregunta", bytes.NewReader(body))
	r = injectFormulariosCtx(r, tenantID.String(), userID.String(), 1, "Admin")
	w := httptest.NewRecorder()

	h.CreatePregunta(w, r)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code, "whitespace texto → 422")
}

func TestCreatePregunta_ServiceErrNotFound_Returns404(t *testing.T) {
	// Foreign-tenant formulario produces ErrNotFound from the
	// service-layer IDOR check.
	tenantID := uuid.New()
	userID := uuid.New()
	svc := &mockFormularioService{
		addPreguntaFn: func(_ context.Context, _ *formularios.Pregunta, _ uuid.UUID) (*formularios.Pregunta, error) {
			return nil, formularios.ErrNotFound
		},
	}
	h := formshandler.NewFormularioHandler(svc)

	body, _ := json.Marshal(map[string]any{
		"formulario_id":  uuid.New(),
		"orden":          1,
		"texto_pregunta": "x",
		"tipo_pregunta":  1,
		"obligatoria":    true,
	})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/formularios/pregunta", bytes.NewReader(body))
	r = injectFormulariosCtx(r, tenantID.String(), userID.String(), 1, "Admin")
	w := httptest.NewRecorder()

	h.CreatePregunta(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), `"NOT_FOUND"`)
}

func TestCreatePregunta_ValidRequest_Returns201WithData(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()
	formID := uuid.New()
	created := &formularios.Pregunta{
		ID:            uuid.New(),
		FormularioID:  formID,
		Orden:         1,
		TipoPregunta:  formularios.TipoPreguntaTexto,
		TextoPregunta: "Limpieza",
		Obligatoria:   true,
	}
	svc := &mockFormularioService{
		addPreguntaFn: func(_ context.Context, p *formularios.Pregunta, _ uuid.UUID) (*formularios.Pregunta, error) {
			return created, nil
		},
	}
	h := formshandler.NewFormularioHandler(svc)

	body, _ := json.Marshal(map[string]any{
		"formulario_id":  formID,
		"orden":          1,
		"texto_pregunta": "Limpieza",
		"tipo_pregunta":  formularios.TipoPreguntaTexto,
		"obligatoria":    true,
	})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/formularios/pregunta", bytes.NewReader(body))
	r = injectFormulariosCtx(r, tenantID.String(), userID.String(), 1, "Admin")
	w := httptest.NewRecorder()

	h.CreatePregunta(w, r)

	require.Equal(t, http.StatusCreated, w.Code, "happy path → 201, body: %s", w.Body.String())
	var env map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
	require.Equal(t, true, env["success"])
	data, ok := env["data"].(map[string]any)
	require.True(t, ok, "expected data object, got %T", env["data"])
	assert.Equal(t, created.ID.String(), data["id"])
	assert.Equal(t, formID.String(), data["formulario_id"])
	assert.EqualValues(t, 1, data["orden"])
	assert.EqualValues(t, formularios.TipoPreguntaTexto, data["tipo_pregunta"])
}
