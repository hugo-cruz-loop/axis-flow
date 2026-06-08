package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"axis-flow-back/internal/clientes"
	"axis-flow-back/internal/clientes/handler"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── stub ──────────────────────────────────────────────────────────────────────

type stubSiteConfigService struct {
	addServicioFn    func(ctx context.Context, localidadID uuid.UUID, servicioID int64, empresaID int64) error
	removeServicioFn func(ctx context.Context, localidadID uuid.UUID, servicioID int64, empresaID int64) error
	listServiciosFn  func(ctx context.Context, localidadID uuid.UUID, empresaID int64) ([]clientes.ServiciosLocalidad, error)

	createHorarioFn func(ctx context.Context, h *clientes.Horario, empresaID int64) (*clientes.Horario, error)
	listHorariosFn  func(ctx context.Context, localidadID uuid.UUID, empresaID int64) ([]clientes.Horario, error)
	updateHorarioFn func(ctx context.Context, h *clientes.Horario, empresaID int64) (*clientes.Horario, error)
	deleteHorarioFn func(ctx context.Context, id uuid.UUID, empresaID int64) error

	createHerramientaFn func(ctx context.Context, h *clientes.Herramienta, empresaID int64) (*clientes.Herramienta, error)
	listHerramientasFn  func(ctx context.Context, localidadID uuid.UUID, empresaID int64) ([]clientes.Herramienta, error)
	updateHerramientaFn func(ctx context.Context, h *clientes.Herramienta, empresaID int64) (*clientes.Herramienta, error)
	deleteHerramientaFn func(ctx context.Context, id uuid.UUID, empresaID int64) error

	createActividadFn func(ctx context.Context, a *clientes.Actividad, empresaID int64) (*clientes.Actividad, error)
	listActividadesFn func(ctx context.Context, localidadID uuid.UUID, empresaID int64) ([]clientes.Actividad, error)
	updateActividadFn func(ctx context.Context, a *clientes.Actividad, empresaID int64) (*clientes.Actividad, error)
	deleteActividadFn func(ctx context.Context, id uuid.UUID, empresaID int64) error
}

func (s *stubSiteConfigService) AddServicio(ctx context.Context, id uuid.UUID, sid int64, eid int64) error {
	return s.addServicioFn(ctx, id, sid, eid)
}
func (s *stubSiteConfigService) RemoveServicio(ctx context.Context, id uuid.UUID, sid int64, eid int64) error {
	return s.removeServicioFn(ctx, id, sid, eid)
}
func (s *stubSiteConfigService) ListServicios(ctx context.Context, id uuid.UUID, eid int64) ([]clientes.ServiciosLocalidad, error) {
	return s.listServiciosFn(ctx, id, eid)
}
func (s *stubSiteConfigService) CreateHorario(ctx context.Context, h *clientes.Horario, eid int64) (*clientes.Horario, error) {
	return s.createHorarioFn(ctx, h, eid)
}
func (s *stubSiteConfigService) ListHorarios(ctx context.Context, id uuid.UUID, eid int64) ([]clientes.Horario, error) {
	return s.listHorariosFn(ctx, id, eid)
}
func (s *stubSiteConfigService) UpdateHorario(ctx context.Context, h *clientes.Horario, eid int64) (*clientes.Horario, error) {
	return s.updateHorarioFn(ctx, h, eid)
}
func (s *stubSiteConfigService) DeleteHorario(ctx context.Context, id uuid.UUID, eid int64) error {
	return s.deleteHorarioFn(ctx, id, eid)
}
func (s *stubSiteConfigService) CreateHerramienta(ctx context.Context, h *clientes.Herramienta, eid int64) (*clientes.Herramienta, error) {
	return s.createHerramientaFn(ctx, h, eid)
}
func (s *stubSiteConfigService) ListHerramientas(ctx context.Context, id uuid.UUID, eid int64) ([]clientes.Herramienta, error) {
	return s.listHerramientasFn(ctx, id, eid)
}
func (s *stubSiteConfigService) UpdateHerramienta(ctx context.Context, h *clientes.Herramienta, eid int64) (*clientes.Herramienta, error) {
	return s.updateHerramientaFn(ctx, h, eid)
}
func (s *stubSiteConfigService) DeleteHerramienta(ctx context.Context, id uuid.UUID, eid int64) error {
	return s.deleteHerramientaFn(ctx, id, eid)
}
func (s *stubSiteConfigService) CreateActividad(ctx context.Context, a *clientes.Actividad, eid int64) (*clientes.Actividad, error) {
	return s.createActividadFn(ctx, a, eid)
}
func (s *stubSiteConfigService) ListActividades(ctx context.Context, id uuid.UUID, eid int64) ([]clientes.Actividad, error) {
	return s.listActividadesFn(ctx, id, eid)
}
func (s *stubSiteConfigService) UpdateActividad(ctx context.Context, a *clientes.Actividad, eid int64) (*clientes.Actividad, error) {
	return s.updateActividadFn(ctx, a, eid)
}
func (s *stubSiteConfigService) DeleteActividad(ctx context.Context, id uuid.UUID, eid int64) error {
	return s.deleteActividadFn(ctx, id, eid)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func newSiteChiCtx(localidadID string, extraKey, extraVal string) context.Context {
	chiCtx := chi.NewRouteContext()
	chiCtx.URLParams.Add("localidad_id", localidadID)
	if extraKey != "" {
		chiCtx.URLParams.Add(extraKey, extraVal)
	}
	return context.WithValue(context.Background(), chi.RouteCtxKey, chiCtx)
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestAddServicio_Duplicate_409(t *testing.T) {
	localidadID := uuid.New()
	svc := &stubSiteConfigService{
		addServicioFn: func(_ context.Context, _ uuid.UUID, _ int64, _ int64) error {
			// Simulate a unique-constraint duplicate — map to a generic error
			// In practice the service would return a descriptive error; we use a known one
			return clientes.ErrFacturaExists // reusing a conflict sentinel for test purposes
		},
	}
	h := handler.NewSiteConfigHandler(svc)

	body, _ := json.Marshal(map[string]any{"servicio_id": 5})
	r := httptest.NewRequest(http.MethodPost, "/localidad/"+localidadID.String()+"/servicios?empresa_id=1",
		bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = r.WithContext(newSiteChiCtx(localidadID.String(), "", ""))

	w := httptest.NewRecorder()
	h.AddServicio(w, r)
	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestListHorarios_Happy_200(t *testing.T) {
	localidadID := uuid.New()
	svc := &stubSiteConfigService{
		listHorariosFn: func(_ context.Context, _ uuid.UUID, _ int64) ([]clientes.Horario, error) {
			return []clientes.Horario{
				{ID: uuid.New(), HoraEntrada: "08:00", HoraSalida: "17:00"},
			}, nil
		},
	}
	h := handler.NewSiteConfigHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/localidad/"+localidadID.String()+"/horario?empresa_id=1", nil)
	r = r.WithContext(newSiteChiCtx(localidadID.String(), "", ""))

	w := httptest.NewRecorder()
	h.ListHorarios(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	data, ok := resp["data"].([]any)
	require.True(t, ok)
	assert.Len(t, data, 1)
}

func TestDeleteActividad_Happy_204(t *testing.T) {
	localidadID := uuid.New()
	actividadID := uuid.New()
	svc := &stubSiteConfigService{
		deleteActividadFn: func(_ context.Context, _ uuid.UUID, _ int64) error {
			return nil
		},
	}
	h := handler.NewSiteConfigHandler(svc)

	url := "/localidad/" + localidadID.String() + "/actividades/" + actividadID.String() + "?empresa_id=1"
	r := httptest.NewRequest(http.MethodDelete, url, strings.NewReader(""))
	r = r.WithContext(newSiteChiCtx(localidadID.String(), "id", actividadID.String()))

	w := httptest.NewRecorder()
	h.DeleteActividad(w, r)
	assert.Equal(t, http.StatusNoContent, w.Code)
}
