package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestRegisterEmpleadosRoutesMapsPR4HandlersAndWebSocket(t *testing.T) {
	recorder := &routeRecorder{}
	r := chi.NewRouter()

	registerEmpleadosRoutes(r, recorder.handlers(), http.HandlerFunc(recorder.websocket))

	cases := []struct {
		name   string
		method string
		path   string
		want   string
	}{
		{"create employee", http.MethodPost, "/api/v1/empleado", "create-empleado"},
		{"get employee", http.MethodGet, "/api/v1/empleado/105?empresa_id=12", "get-empleado"},
		{"update employee", http.MethodPut, "/api/v1/empleado/105?empresa_id=12", "update-empleado"},
		{"delete employee", http.MethodDelete, "/api/v1/empleado/105?empresa_id=12", "delete-empleado"},
		{"list employees", http.MethodGet, "/api/v1/empresa/12/empleados", "list-empleados"},
		{"employee by user", http.MethodGet, "/api/v1/empleado/by-user/8f37bc44-593b-48c2-a9b7-f58c73491a92?empresa_id=12", "get-empleado-by-user"},
		{"get ubicacion", http.MethodGet, "/api/v1/empleado/105/ubicacion?empresa_id=12", "get-ubicacion"},
		{"update ubicacion", http.MethodPut, "/api/v1/empleado/105/ubicacion?empresa_id=12", "update-ubicacion"},
		{"get adicionales", http.MethodGet, "/api/v1/empleado/105/adicionales?empresa_id=12", "get-adicionales"},
		{"update adicionales", http.MethodPut, "/api/v1/empleado/105/adicionales?empresa_id=12", "update-adicionales"},
		{"list documentos", http.MethodGet, "/api/v1/empleado/105/documentos?empresa_id=12", "list-documentos"},
		{"upload documentos", http.MethodPost, "/api/v1/empleado/105/documentos?empresa_id=12", "upload-documento"},
		{"list asistencias", http.MethodGet, "/api/v1/empleado/asistencias?empresa_id=12", "list-asistencias"},
		{"create asistencia", http.MethodPost, "/api/v1/empleado/asistencias", "create-asistencia"},
		{"list inasistencias", http.MethodGet, "/api/v1/empleado/inasistencia?empresa_id=12&empleado_id=105", "list-inasistencias"},
		{"create inasistencia", http.MethodPost, "/api/v1/empleado/inasistencia", "create-inasistencia"},
		{"resolve inasistencia", http.MethodPut, "/api/v1/empleado/inasistencia/8f37bc44-593b-48c2-a9b7-f58c73491a92?empresa_id=12", "resolve-inasistencia"},
		{"list devices", http.MethodGet, "/api/v1/empleado/devices?empresa_id=12&empleado_id=105", "list-devices"},
		{"register device", http.MethodPost, "/api/v1/empleado/devices", "register-device"},
		{"websocket", http.MethodGet, "/api/v1/ws/empleados/asistencias?empresa_id=12", "websocket"},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			res := httptest.NewRecorder()

			r.ServeHTTP(res, req)

			if res.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 body=%s", res.Code, res.Body.String())
			}
			if res.Body.String() != tt.want {
				t.Fatalf("body = %q, want %q", res.Body.String(), tt.want)
			}
		})
	}
}

type routeRecorder struct{}

func (r *routeRecorder) handlers() empleadosRouteHandlers {
	return empleadosRouteHandlers{
		CreateEmpleado:      r.write("create-empleado"),
		GetEmpleado:         r.write("get-empleado"),
		UpdateEmpleado:      r.write("update-empleado"),
		DeleteEmpleado:      r.write("delete-empleado"),
		ListEmpleados:       r.write("list-empleados"),
		GetEmpleadoByUser:   r.write("get-empleado-by-user"),
		GetUbicacion:        r.write("get-ubicacion"),
		UpdateUbicacion:     r.write("update-ubicacion"),
		GetAdicionales:      r.write("get-adicionales"),
		UpdateAdicionales:   r.write("update-adicionales"),
		ListDocumentos:      r.write("list-documentos"),
		UploadDocumento:     r.write("upload-documento"),
		ListAsistencias:     r.write("list-asistencias"),
		CreateAsistencia:    r.write("create-asistencia"),
		ListInasistencias:   r.write("list-inasistencias"),
		CreateInasistencia:  r.write("create-inasistencia"),
		ResolveInasistencia: r.write("resolve-inasistencia"),
		ListDevices:         r.write("list-devices"),
		RegisterDevice:      r.write("register-device"),
	}
}

func (r *routeRecorder) websocket(w http.ResponseWriter, _ *http.Request) {
	_, _ = w.Write([]byte("websocket"))
}
func (r *routeRecorder) write(value string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(value)) }
}
