package dashboardsdata_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"axis-flow-back/internal/dashboardsdata"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPHandler_ReturnsDummyJSON(t *testing.T) {
	cache := dashboardsdata.NewCacheClient(nil)
	handler := dashboardsdata.NewHTTPHandler(cache, nil)
	router := chi.NewRouter()
	noopAuth := func(next http.Handler) http.Handler { return next }
	dashboardsdata.RegisterRoutes(router, handler, noopAuth)

	tests := []struct {
		name           string
		method         string
		url            string
		expectedStatus int
		validateJSON   func(t *testing.T, body []byte)
	}{
		{
			name:           "GetEvaluacionesClientes_Success",
			method:         http.MethodGet,
			url:            "/api/v1/dashboards/admin-empresa/123/evaluaciones-clientes",
			expectedStatus: http.StatusOK,
			validateJSON: func(t *testing.T, body []byte) {
				var resp dashboardsdata.AdminEmpresaEvaluacionesResponse
				err := json.Unmarshal(body, &resp)
				require.NoError(t, err)
				assert.Equal(t, int64(123), resp.Data.EmpresaID)
				assert.Empty(t, resp.Data.Evaluaciones)
			},
		},
		{
			name:           "GetEvaluacionesClientes_InvalidID",
			method:         http.MethodGet,
			url:            "/api/v1/dashboards/admin-empresa/abc/evaluaciones-clientes",
			expectedStatus: http.StatusBadRequest,
			validateJSON: func(t *testing.T, body []byte) {
				var errResp map[string]any
				err := json.Unmarshal(body, &errResp)
				require.NoError(t, err)
				assert.Contains(t, errResp, "error")
			},
		},
		{
			name:           "GetEmpleadosCount_Success",
			method:         http.MethodGet,
			url:            "/api/v1/dashboards/admin-empresa/456/empleados/count",
			expectedStatus: http.StatusOK,
			validateJSON: func(t *testing.T, body []byte) {
				var resp dashboardsdata.AdminEmpresaEmpleadosCountResponse
				err := json.Unmarshal(body, &resp)
				require.NoError(t, err)
				assert.Equal(t, int64(456), resp.Data.EmpresaID)
				assert.Equal(t, 0, resp.Data.TotalEmpleados)
			},
		},
		{
			name:           "GetActividadesCount_Success",
			method:         http.MethodGet,
			url:            "/api/v1/dashboards/admin-empresa/789/actividades/count?periodo=week",
			expectedStatus: http.StatusOK,
			validateJSON: func(t *testing.T, body []byte) {
				var resp dashboardsdata.AdminEmpresaActividadesCountResponse
				err := json.Unmarshal(body, &resp)
				require.NoError(t, err)
				assert.Equal(t, int64(789), resp.Data.EmpresaID)
				assert.Equal(t, 0, resp.Data.TotalActividadesFinalizadas)
				assert.Equal(t, "week", resp.Data.Periodo)
			},
		},
		{
			name:           "GetAusenciasCount_Success",
			method:         http.MethodGet,
			url:            "/api/v1/dashboards/admin-empresa/789/ausencias/count?year=2026",
			expectedStatus: http.StatusOK,
			validateJSON: func(t *testing.T, body []byte) {
				var resp dashboardsdata.AdminEmpresaAusenciasCountResponse
				err := json.Unmarshal(body, &resp)
				require.NoError(t, err)
				assert.Equal(t, int64(789), resp.Data.EmpresaID)
				assert.Equal(t, 0, resp.Data.TotalAusencias)
				require.NotNil(t, resp.Data.Year)
				assert.Equal(t, 2026, *resp.Data.Year)
			},
		},
		{
			name:           "GetServiciosLocalidad_Success",
			method:         http.MethodGet,
			url:            "/api/v1/dashboards/cliente/550e8400-e29b-41d4-a716-446655440000/servicios-localidad",
			expectedStatus: http.StatusOK,
			validateJSON: func(t *testing.T, body []byte) {
				var resp dashboardsdata.ClienteServiciosLocalidadResponse
				err := json.Unmarshal(body, &resp)
				require.NoError(t, err)
				assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", resp.Data.ClientID.String())
				assert.Empty(t, resp.Data.Localidades)
			},
		},
		{
			name:           "GetServiciosLocalidad_InvalidUUID",
			method:         http.MethodGet,
			url:            "/api/v1/dashboards/cliente/invalid-uuid/servicios-localidad",
			expectedStatus: http.StatusBadRequest,
			validateJSON: func(t *testing.T, body []byte) {
				var errResp map[string]any
				err := json.Unmarshal(body, &errResp)
				require.NoError(t, err)
				assert.Contains(t, errResp, "error")
			},
		},
		{
			name:           "GetAtencionSeguimientoStatus_Success",
			method:         http.MethodGet,
			url:            "/api/v1/dashboards/cliente/550e8400-e29b-41d4-a716-446655440000/atencion-seguimiento/status",
			expectedStatus: http.StatusOK,
			validateJSON: func(t *testing.T, body []byte) {
				var resp dashboardsdata.ClienteAtencionSeguimientoStatusResponse
				err := json.Unmarshal(body, &resp)
				require.NoError(t, err)
				assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", resp.Data.ClientID.String())
				assert.Equal(t, 0, resp.Data.TotalTickets)
				assert.Equal(t, 0, resp.Data.Tickets.Pendiente)
				assert.Equal(t, 0, resp.Data.Tickets.EnProceso)
				assert.Equal(t, 0, resp.Data.Tickets.Finalizado)
			},
		},
		{
			name:           "GetBolsaTrabajoVacantesActivas_Success",
			method:         http.MethodGet,
			url:            "/api/v1/dashboards/rh/123/bolsa-trabajo/vacantes-activas",
			expectedStatus: http.StatusOK,
			validateJSON: func(t *testing.T, body []byte) {
				var resp dashboardsdata.RHTotalTrabajosActivosResponse
				err := json.Unmarshal(body, &resp)
				require.NoError(t, err)
				assert.Equal(t, int64(123), resp.Data.EmpresaID)
				assert.Equal(t, 0, resp.Data.TotalVacantesActivas)
				assert.Empty(t, resp.Data.Vacantes)
			},
		},
		{
			name:           "GetEmpleadosAbsentismo_Success",
			method:         http.MethodGet,
			url:            "/api/v1/dashboards/rh/123/empleados/absentismo",
			expectedStatus: http.StatusOK,
			validateJSON: func(t *testing.T, body []byte) {
				var resp dashboardsdata.RHEmpleadosAbsentismoResponse
				err := json.Unmarshal(body, &resp)
				require.NoError(t, err)
				assert.Equal(t, int64(123), resp.Data.EmpresaID)
				assert.Equal(t, 0.0, resp.Data.TasaAbsentismo)
				assert.Equal(t, 0, resp.Data.DiasLaborablesTotales)
				assert.Equal(t, 0, resp.Data.TotalInasistencias)
				assert.Equal(t, 0, resp.Data.EmpleadosAfectados)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.url, nil)
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)
			if tt.validateJSON != nil {
				tt.validateJSON(t, rr.Body.Bytes())
			}
		})
	}
}
