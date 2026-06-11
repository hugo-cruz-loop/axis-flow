package main

import (
	"context"
	"net/http"
	"os"
	"path/filepath"

	"axis-flow-back/internal/config"
	empgateway "axis-flow-back/internal/empleados/gateway"
	empleadoshandler "axis-flow-back/internal/empleados/handler"
	empleadosrepo "axis-flow-back/internal/empleados/repository"
	empleadossvc "axis-flow-back/internal/empleados/service"
	empstorage "axis-flow-back/internal/empleados/storage"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type empleadosRouteHandlers struct {
	CreateEmpleado      http.HandlerFunc
	GetEmpleado         http.HandlerFunc
	UpdateEmpleado      http.HandlerFunc
	DeleteEmpleado      http.HandlerFunc
	ListEmpleados       http.HandlerFunc
	GetEmpleadoByUser   http.HandlerFunc
	GetUbicacion        http.HandlerFunc
	UpdateUbicacion     http.HandlerFunc
	GetAdicionales      http.HandlerFunc
	UpdateAdicionales   http.HandlerFunc
	ListDocumentos      http.HandlerFunc
	UploadDocumento     http.HandlerFunc
	ListAsistencias     http.HandlerFunc
	CreateAsistencia    http.HandlerFunc
	ListInasistencias   http.HandlerFunc
	CreateInasistencia  http.HandlerFunc
	ResolveInasistencia http.HandlerFunc
	ListDevices         http.HandlerFunc
	RegisterDevice      http.HandlerFunc
	GetFotologin        http.HandlerFunc
	UploadFotologin     http.HandlerFunc
	GetKPIComplete      http.HandlerFunc
	GetKPILack          http.HandlerFunc
	GetKPIPendiente     http.HandlerFunc
}

func newEmpleadosModule(dbPool *pgxpool.Pool, redisClient *redis.Client, cfg config.Config) (empleadosRouteHandlers, *empgateway.VerificationGateway) {
	empleadoRepo := empleadosrepo.NewPgxEmpleadoRepository(dbPool)
	expedienteRepo := empleadosrepo.NewPgxExpedienteRepository(dbPool)
	asistenciaRepo := empleadosrepo.NewPgxAsistenciaRepository(dbPool)
	inasistenciaRepo := empleadosrepo.NewPgxInasistenciaRepository(dbPool)
	deviceRepo := empleadosrepo.NewPgxDeviceRepository(dbPool, redisClient)

	empleadoSvc := empleadossvc.NewEmpleadoService(empleadoRepo, zeroTenantResolver{})
	storage := &empstorage.LocalStorage{BaseDir: filepath.Join(os.TempDir(), "axis-flow-back", "empleados")}
	// expedienteRepo satisfies both FotologinStore and KPIStore in addition to ExpedienteStore.
	handler := empleadoshandler.NewHandler(empleadoSvc, expedienteRepo, asistenciaRepo, inasistenciaRepo, deviceRepo, expedienteRepo, expedienteRepo, storage, cfg.Storage.MinIOBucket)
	handler.RedisClient = redisClient

	return empleadosRoutesFromHandler(handler), empgateway.NewVerificationGateway()
}

func empleadosRoutesFromHandler(h *empleadoshandler.Handler) empleadosRouteHandlers {
	return empleadosRouteHandlers{
		CreateEmpleado:      h.CreateEmpleado,
		GetEmpleado:         h.GetEmpleado,
		UpdateEmpleado:      h.UpdateEmpleado,
		DeleteEmpleado:      h.DeleteEmpleado,
		ListEmpleados:       h.ListEmpleados,
		GetEmpleadoByUser:   h.GetEmpleadoByUser,
		GetUbicacion:        h.GetUbicacion,
		UpdateUbicacion:     h.UpdateUbicacion,
		GetAdicionales:      h.GetAdicionales,
		UpdateAdicionales:   h.UpdateAdicionales,
		ListDocumentos:      h.ListDocumentos,
		UploadDocumento:     h.UploadDocumento,
		ListAsistencias:     h.ListAsistencias,
		CreateAsistencia:    h.CreateAsistencia,
		ListInasistencias:   h.ListInasistencias,
		CreateInasistencia:  h.CreateInasistencia,
		ResolveInasistencia: h.ResolveInasistencia,
		ListDevices:         h.ListDevices,
		RegisterDevice:      h.RegisterDevice,
		GetFotologin:        h.GetFotologin,
		UploadFotologin:     h.UploadFotologin,
		GetKPIComplete:      h.GetKPIComplete,
		GetKPILack:          h.GetKPILack,
		GetKPIPendiente:     h.GetKPIPendiente,
	}
}

func registerEmpleadosRoutes(r chi.Router, h empleadosRouteHandlers, ws http.Handler) {
	r.Post("/api/v1/empleado", h.CreateEmpleado)
	r.Get("/api/v1/empleado/{id}", h.GetEmpleado)
	r.Put("/api/v1/empleado/{id}", h.UpdateEmpleado)
	r.Delete("/api/v1/empleado/{id}", h.DeleteEmpleado)
	r.Get("/api/v1/empresa/{empresa_id}/empleados", h.ListEmpleados)
	r.Get("/api/v1/empleado/by-user/{user_id}", h.GetEmpleadoByUser)

	r.Get("/api/v1/empleado/{id}/ubicacion", h.GetUbicacion)
	r.Put("/api/v1/empleado/{id}/ubicacion", h.UpdateUbicacion)
	r.Get("/api/v1/empleado/{id}/adicionales", h.GetAdicionales)
	r.Put("/api/v1/empleado/{id}/adicionales", h.UpdateAdicionales)
	r.Get("/api/v1/empleado/{id}/documentos", h.ListDocumentos)
	r.Post("/api/v1/empleado/{id}/documentos", h.UploadDocumento)

	r.Get("/api/v1/empleado/asistencias", h.ListAsistencias)
	r.Post("/api/v1/empleado/asistencias", h.CreateAsistencia)
	r.Get("/api/v1/empleado/inasistencia", h.ListInasistencias)
	r.Post("/api/v1/empleado/inasistencia", h.CreateInasistencia)
	r.Put("/api/v1/empleado/inasistencia/{id}", h.ResolveInasistencia)

	r.Get("/api/v1/empleado/devices", h.ListDevices)
	r.Post("/api/v1/empleado/devices", h.RegisterDevice)

	r.Get("/api/v1/empleado/{id}/fotologin", h.GetFotologin)
	r.Post("/api/v1/empleado/{id}/fotologin", h.UploadFotologin)

	r.Get("/api/v1/empresa/{empresa_id}/empleados/documentos/complete", h.GetKPIComplete)
	r.Get("/api/v1/empresa/{empresa_id}/empleados/documentos/lack", h.GetKPILack)
	r.Get("/api/v1/empresa/{empresa_id}/empleados/documentos/pendiente", h.GetKPIPendiente)

	if ws != nil {
		r.Get("/api/v1/ws/empleados/asistencias", ws.ServeHTTP)
	}
}

// zeroTenantResolver satisfies the TenantResolver port by returning uuid.Nil.
// users.identity_users.tenant_id was made nullable in V3__fix_schema_gaps.sql, so the zero UUID
// (00000000-0000-0000-0000-000000000000) written by pgx is accepted by the database.
// Replace this with a real DB-backed resolver once the empresa→tenant mapping table exists.
type zeroTenantResolver struct{}

func (zeroTenantResolver) ResolveTenantID(_ context.Context, _ int64) (uuid.UUID, error) {
	return uuid.Nil, nil
}
