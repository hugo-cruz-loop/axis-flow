package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"axis-flow-back/internal/empresas"
	"axis-flow-back/internal/empresas/handler"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── stubs ─────────────────────────────────────────────────────────────────────

type stubEmpresaService struct {
	empresa     *empresas.Empresa
	getErr      error
	updateErr   error
	deleteErr   error
	deleteCalls int
}

func (s *stubEmpresaService) GetByID(ctx context.Context, id int64) (*empresas.Empresa, error) {
	return s.empresa, s.getErr
}
func (s *stubEmpresaService) Update(ctx context.Context, e *empresas.Empresa) error {
	return s.updateErr
}
func (s *stubEmpresaService) Delete(ctx context.Context, id int64) error {
	s.deleteCalls++
	return s.deleteErr
}

type stubDatosFiscalesRepo struct {
	fiscal    *empresas.DatosFiscales
	findErr   error
	createErr error
	updateErr error
}

func (r *stubDatosFiscalesRepo) FindByEmpresaID(ctx context.Context, empresaID int64) (*empresas.DatosFiscales, error) {
	return r.fiscal, r.findErr
}
func (r *stubDatosFiscalesRepo) Create(ctx context.Context, d *empresas.DatosFiscales) error {
	return r.createErr
}
func (r *stubDatosFiscalesRepo) Update(ctx context.Context, d *empresas.DatosFiscales) error {
	return r.updateErr
}

type stubApoderadoRepo struct {
	list      []empresas.Apoderado
	createErr error
	updateErr error
	deleteErr error
}

func (r *stubApoderadoRepo) ListByEmpresaID(ctx context.Context, empresaID int64) ([]empresas.Apoderado, error) {
	return r.list, nil
}
func (r *stubApoderadoRepo) Create(ctx context.Context, a *empresas.Apoderado) error {
	return r.createErr
}
func (r *stubApoderadoRepo) Update(ctx context.Context, a *empresas.Apoderado) error {
	return r.updateErr
}
func (r *stubApoderadoRepo) Delete(ctx context.Context, id int64) error {
	return r.deleteErr
}

type stubServicioRepo struct {
	list      []empresas.Servicio
	createErr error
	updateErr error
	deleteErr error
}

func (r *stubServicioRepo) ListByEmpresaID(ctx context.Context, empresaID int64) ([]empresas.Servicio, error) {
	return r.list, nil
}
func (r *stubServicioRepo) Create(ctx context.Context, s *empresas.Servicio) error {
	return r.createErr
}
func (r *stubServicioRepo) Update(ctx context.Context, s *empresas.Servicio) error {
	return r.updateErr
}
func (r *stubServicioRepo) Delete(ctx context.Context, id int64) error {
	return r.deleteErr
}

// ── helpers ───────────────────────────────────────────────────────────────────

func routeRequest(method, url string, body any, h http.HandlerFunc, params map[string]string) *httptest.ResponseRecorder {
	var bodyReader *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(b)
	} else {
		bodyReader = bytes.NewReader(nil)
	}

	req := httptest.NewRequest(method, url, bodyReader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Inject chi URL params into context
	rc := chi.NewRouteContext()
	for k, v := range params {
		rc.URLParams.Add(k, v)
	}
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rc))

	w := httptest.NewRecorder()
	h(w, req)
	return w
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestGetEmpresa_WithValidID_Returns200(t *testing.T) {
	svc := &stubEmpresaService{
		empresa: &empresas.Empresa{ID: 1, Nombre: "Test SA", Status: empresas.EmpresaStatusActive},
	}
	h := handler.NewProtectedHandler(svc, &stubDatosFiscalesRepo{}, &stubApoderadoRepo{}, &stubServicioRepo{})

	w := routeRequest(http.MethodGet, "/api/v1/empresa/1", nil, h.GetEmpresa, map[string]string{"id": "1"})

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]any)
	assert.Equal(t, "Test SA", data["Nombre"])
}

func TestGetEmpresa_WithInvalidID_Returns404(t *testing.T) {
	svc := &stubEmpresaService{getErr: empresas.ErrEmpresaNotFound}
	h := handler.NewProtectedHandler(svc, &stubDatosFiscalesRepo{}, &stubApoderadoRepo{}, &stubServicioRepo{})

	w := routeRequest(http.MethodGet, "/api/v1/empresa/99", nil, h.GetEmpresa, map[string]string{"id": "99"})

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPutEmpresa_WithValidBody_Returns200(t *testing.T) {
	svc := &stubEmpresaService{
		empresa: &empresas.Empresa{ID: 1, Nombre: "Old Name", Status: empresas.EmpresaStatusActive},
	}
	h := handler.NewProtectedHandler(svc, &stubDatosFiscalesRepo{}, &stubApoderadoRepo{}, &stubServicioRepo{})

	w := routeRequest(http.MethodPut, "/api/v1/empresa/1", map[string]any{
		"nombre":    "New Name",
		"direccion": "Nueva Dir",
		"telefono":  "5559999999",
	}, h.UpdateEmpresa, map[string]string{"id": "1"})

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDeleteEmpresa_Cascades_Returns200(t *testing.T) {
	svc := &stubEmpresaService{}
	h := handler.NewProtectedHandler(svc, &stubDatosFiscalesRepo{}, &stubApoderadoRepo{}, &stubServicioRepo{})

	w := routeRequest(http.MethodDelete, "/api/v1/empresa/1", nil, h.DeleteEmpresa, map[string]string{"id": "1"})

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 1, svc.deleteCalls)
}

func TestGetFiscal_Returns200(t *testing.T) {
	fiscal := &stubDatosFiscalesRepo{
		fiscal: &empresas.DatosFiscales{ID: 1, EmpresaID: 1, RazonSocial: "Test SA de CV"},
	}
	svc := &stubEmpresaService{}
	h := handler.NewProtectedHandler(svc, fiscal, &stubApoderadoRepo{}, &stubServicioRepo{})

	w := routeRequest(http.MethodGet, "/api/v1/empresa/1/fiscal", nil, h.GetFiscal, map[string]string{"id": "1"})

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestPostFiscal_Creates_Returns201(t *testing.T) {
	fiscal := &stubDatosFiscalesRepo{}
	svc := &stubEmpresaService{}
	h := handler.NewProtectedHandler(svc, fiscal, &stubApoderadoRepo{}, &stubServicioRepo{})

	w := routeRequest(http.MethodPost, "/api/v1/empresa/1/fiscal", map[string]any{
		"rfc":          "TEST123456",
		"razon_social": "Test SA de CV",
	}, h.CreateFiscal, map[string]string{"id": "1"})

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestListApoderados_Returns200(t *testing.T) {
	apoderados := &stubApoderadoRepo{
		list: []empresas.Apoderado{{ID: 1, Nombre: "Juan Perez"}},
	}
	svc := &stubEmpresaService{}
	h := handler.NewProtectedHandler(svc, &stubDatosFiscalesRepo{}, apoderados, &stubServicioRepo{})

	w := routeRequest(http.MethodGet, "/api/v1/empresa/1/apoderados", nil, h.ListApoderados, map[string]string{"id": "1"})

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestListServicios_Returns200(t *testing.T) {
	servicios := &stubServicioRepo{
		list: []empresas.Servicio{{ID: 1, Nombre: "Servicio A"}},
	}
	svc := &stubEmpresaService{}
	h := handler.NewProtectedHandler(svc, &stubDatosFiscalesRepo{}, &stubApoderadoRepo{}, servicios)

	w := routeRequest(http.MethodGet, "/api/v1/empresa/1/servicios", nil, h.ListServicios, map[string]string{"id": "1"})

	assert.Equal(t, http.StatusOK, w.Code)
}
