// Package handler provides HTTP handlers for the empleados module.
package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	empleados "axis-flow-back/internal/empleados"
	"axis-flow-back/internal/empleados/service"
	empstorage "axis-flow-back/internal/empleados/storage"
	"axis-flow-back/internal/middleware"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const (
	maxDocumentoBytes  = 5 * 1024 * 1024
	maxAsistenciaBytes = 10 * 1024 * 1024
	defaultHoraEntrada = "00:00"
)

// EmpleadoServicer is the service port used by employee lifecycle handlers.
type EmpleadoServicer interface {
	CreateEmpleado(ctx context.Context, req service.CreateEmpleadoRequest) (*empleados.Empleado, error)
	GetEmpleado(ctx context.Context, numEmpleado, empresaID int64) (*empleados.Empleado, error)
	ListEmpleados(ctx context.Context, empresaID int64) ([]empleados.Empleado, error)
	GetEmpleadoByUser(ctx context.Context, userID uuid.UUID, empresaID int64) (*empleados.Empleado, error)
	UpdateEmpleado(ctx context.Context, numEmpleado, empresaID int64, req service.UpdateEmpleadoRequest) (*empleados.Empleado, error)
	DeleteEmpleado(ctx context.Context, numEmpleado, empresaID int64) error
}

// ExpedienteStore is the handler-level port for employee expediente subresources.
type ExpedienteStore interface {
	GetUbicacion(ctx context.Context, empleadoID, empresaID int64) (*empleados.Ubicacion, error)
	UpsertUbicacion(ctx context.Context, u *empleados.Ubicacion, empresaID int64) error
	GetAdicionales(ctx context.Context, empleadoID, empresaID int64) (*empleados.Adicionales, error)
	UpsertAdicionales(ctx context.Context, a *empleados.Adicionales, empresaID int64) error
	GetDocumentos(ctx context.Context, empleadoID, empresaID int64) (*empleados.Documentos, error)
	UpsertDocumentos(ctx context.Context, d *empleados.Documentos, empresaID int64) error
}

// AsistenciaStore is the handler-level port for attendance records.
type AsistenciaStore interface {
	Create(ctx context.Context, a *empleados.Asistencia, empresaID int64) error
	GetByEmpleado(ctx context.Context, empleadoID, empresaID int64) ([]empleados.Asistencia, error)
	GetByEmpresa(ctx context.Context, empresaID int64) ([]empleados.Asistencia, error)
}

// InasistenciaStore is the handler-level port for absence records.
type InasistenciaStore interface {
	Create(ctx context.Context, i *empleados.Inasistencia, empresaID int64) error
	GetByEmpleado(ctx context.Context, empleadoID, empresaID int64) ([]empleados.Inasistencia, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, empresaID int64, aprobado bool, observaciones *string, resolvedBy uuid.UUID) error
}

// DeviceStore is the handler-level port for authorized mobile devices.
type DeviceStore interface {
	Upsert(ctx context.Context, d *empleados.UserDevice, empresaID int64) error
	GetByEmpleado(ctx context.Context, empleadoID, empresaID int64) ([]empleados.UserDevice, error)
}

// FotologinStore is the handler-level port for employee biometric base photos.
type FotologinStore interface {
	ListFotologin(ctx context.Context, empleadoID, empresaID int64) ([]empleados.Fotologin, error)
	CreateFotologin(ctx context.Context, f *empleados.Fotologin, empresaID int64) error
}

// KPIStore is the handler-level port for documento KPI counts.
type KPIStore interface {
	KPIDocumentosComplete(ctx context.Context, empresaID int64) (int64, error)
	KPIDocumentosLack(ctx context.Context, empresaID int64) (int64, error)
	KPIDocumentosPendiente(ctx context.Context, empresaID int64) (int64, error)
}

// FileStorage is the storage port used by multipart upload handlers.
type FileStorage interface {
	Upload(ctx context.Context, bucket, key string, r io.Reader, size int64, contentType string) (string, error)
	Delete(ctx context.Context, bucket, key string) error
}

// Handler groups all empleados HTTP handlers without owning router wiring.
type Handler struct {
	empleados     EmpleadoServicer
	expedientes   ExpedienteStore
	asistencias   AsistenciaStore
	inasistencias InasistenciaStore
	devices       DeviceStore
	fotologin     FotologinStore
	kpi           KPIStore
	storage       FileStorage
	bucket        string
}

// NewHandler creates an empleados handler bundle.
func NewHandler(empleadosSvc EmpleadoServicer, expedientes ExpedienteStore, asistencias AsistenciaStore, inasistencias InasistenciaStore, devices DeviceStore, fotologin FotologinStore, kpi KPIStore, storage FileStorage, bucket string) *Handler {
	if bucket == "" {
		bucket = "empleados"
	}
	return &Handler{empleados: empleadosSvc, expedientes: expedientes, asistencias: asistencias, inasistencias: inasistencias, devices: devices, fotologin: fotologin, kpi: kpi, storage: storage, bucket: bucket}
}

// CreateEmpleado handles POST /empleado.
func (h *Handler) CreateEmpleado(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email           string `json:"email"`
		Nombre          string `json:"nombre"`
		ApellidoPaterno string `json:"apellido_paterno"`
		ApellidoMaterno string `json:"apellido_materno"`
		IDEmpleado      string `json:"id_empleado"`
		EmpresaID       int64  `json:"empresa_id"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest)
		return
	}

	createdBy, _ := userIDFromContext(r.Context())
	e, err := h.empleados.CreateEmpleado(r.Context(), service.CreateEmpleadoRequest{
		Email:           body.Email,
		Nombre:          body.Nombre,
		ApellidoPaterno: body.ApellidoPaterno,
		ApellidoMaterno: body.ApellidoMaterno,
		IDEmpleado:      body.IDEmpleado,
		EmpresaID:       body.EmpresaID,
		CreatedBy:       createdBy,
	})
	if err != nil {
		writeEmpleadoError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": e})
}

// GetEmpleado handles GET /empleado/{id}.
func (h *Handler) GetEmpleado(w http.ResponseWriter, r *http.Request) {
	empleadoID, empresaID, ok := parseEmpleadoEmpresa(w, r)
	if !ok {
		return
	}
	e, err := h.empleados.GetEmpleado(r.Context(), empleadoID, empresaID)
	if err != nil {
		writeEmpleadoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": e})
}

// UpdateEmpleado handles PUT /empleado/{id}.
func (h *Handler) UpdateEmpleado(w http.ResponseWriter, r *http.Request) {
	empleadoID, empresaID, ok := parseEmpleadoEmpresa(w, r)
	if !ok {
		return
	}
	var body struct {
		Nombre          *string `json:"nombre"`
		ApellidoPaterno *string `json:"apellido_paterno"`
		ApellidoMaterno *string `json:"apellido_materno"`
		Status          *int    `json:"status"`
		IDEmpleado      *string `json:"id_empleado"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest)
		return
	}
	e, err := h.empleados.UpdateEmpleado(r.Context(), empleadoID, empresaID, service.UpdateEmpleadoRequest{
		Nombre:          body.Nombre,
		ApellidoPaterno: body.ApellidoPaterno,
		ApellidoMaterno: body.ApellidoMaterno,
		Status:          body.Status,
	})
	if err != nil {
		writeEmpleadoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": e})
}

// DeleteEmpleado handles DELETE /empleado/{id}.
func (h *Handler) DeleteEmpleado(w http.ResponseWriter, r *http.Request) {
	empleadoID, empresaID, ok := parseEmpleadoEmpresa(w, r)
	if !ok {
		return
	}
	if err := h.empleados.DeleteEmpleado(r.Context(), empleadoID, empresaID); err != nil {
		writeEmpleadoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"message": "Empleado y usuario de acceso asociados fueron eliminados correctamente en cascada.", "id": empleadoID}})
}

// ListEmpleados handles GET /empresa/{empresa_id}/empleados.
func (h *Handler) ListEmpleados(w http.ResponseWriter, r *http.Request) {
	empresaID, ok := parseEmpresaIDPath(w, r)
	if !ok {
		return
	}
	list, err := h.empleados.ListEmpleados(r.Context(), empresaID)
	if err != nil {
		writeEmpleadoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"items": list, "pagination": paginationFromQuery(r, len(list))}})
}

// GetEmpleadoByUser handles GET /empleado/by-user/{user_id}.
func (h *Handler) GetEmpleadoByUser(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(chi.URLParam(r, "user_id"))
	if err != nil {
		writeError(w, "BAD_REQUEST", "invalid user_id", http.StatusBadRequest)
		return
	}
	empresaID, ok := parseEmpresaIDQuery(w, r)
	if !ok {
		return
	}
	e, err := h.empleados.GetEmpleadoByUser(r.Context(), userID, empresaID)
	if err != nil {
		writeEmpleadoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": e})
}

// GetUbicacion handles GET /empleado/{id}/ubicacion.
func (h *Handler) GetUbicacion(w http.ResponseWriter, r *http.Request) {
	empleadoID, empresaID, ok := parseEmpleadoEmpresa(w, r)
	if !ok {
		return
	}
	u, err := h.expedientes.GetUbicacion(r.Context(), empleadoID, empresaID)
	if err != nil {
		writeEmpleadoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": u})
}

// UpdateUbicacion handles PUT /empleado/{id}/ubicacion.
func (h *Handler) UpdateUbicacion(w http.ResponseWriter, r *http.Request) {
	empleadoID, empresaID, ok := parseEmpleadoEmpresa(w, r)
	if !ok {
		return
	}
	var body empleados.Ubicacion
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest)
		return
	}
	body.EmpleadoID = empleadoID
	if err := h.expedientes.UpsertUbicacion(r.Context(), &body, empresaID); err != nil {
		writeEmpleadoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": body})
}

// GetAdicionales handles GET /empleado/{id}/adicionales.
func (h *Handler) GetAdicionales(w http.ResponseWriter, r *http.Request) {
	empleadoID, empresaID, ok := parseEmpleadoEmpresa(w, r)
	if !ok {
		return
	}
	a, err := h.expedientes.GetAdicionales(r.Context(), empleadoID, empresaID)
	if err != nil {
		writeEmpleadoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": a})
}

// UpdateAdicionales handles PUT /empleado/{id}/adicionales.
func (h *Handler) UpdateAdicionales(w http.ResponseWriter, r *http.Request) {
	empleadoID, empresaID, ok := parseEmpleadoEmpresa(w, r)
	if !ok {
		return
	}
	var body empleados.Adicionales
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest)
		return
	}
	body.EmpleadoID = empleadoID
	if err := h.expedientes.UpsertAdicionales(r.Context(), &body, empresaID); err != nil {
		writeEmpleadoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": body})
}

// ListDocumentos handles GET /empleado/{id}/documentos.
func (h *Handler) ListDocumentos(w http.ResponseWriter, r *http.Request) {
	empleadoID, empresaID, ok := parseEmpleadoEmpresa(w, r)
	if !ok {
		return
	}
	d, err := h.expedientes.GetDocumentos(r.Context(), empleadoID, empresaID)
	if err != nil {
		writeEmpleadoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": documentoResponses(d)})
}

// UploadDocumento handles POST /empleado/{id}/documentos.
func (h *Handler) UploadDocumento(w http.ResponseWriter, r *http.Request) {
	empleadoID, empresaID, ok := parseEmpleadoEmpresa(w, r)
	if !ok {
		return
	}
	if err := r.ParseMultipartForm(maxDocumentoBytes); err != nil {
		writeError(w, "BAD_REQUEST", "invalid multipart body", http.StatusBadRequest)
		return
	}
	tipoDocumento := strings.TrimSpace(r.FormValue("tipo_documento"))
	if !validDocumentoTipo(tipoDocumento) {
		writeError(w, "BAD_REQUEST", "invalid tipo_documento", http.StatusBadRequest)
		return
	}
	url, err := h.uploadMultipartFile(r, "archivo", empleadoID, "documentos", tipoDocumento, maxDocumentoBytes)
	if err != nil {
		writeEmpleadoError(w, err)
		return
	}
	d, err := h.expedientes.GetDocumentos(r.Context(), empleadoID, empresaID)
	if err != nil {
		if !errors.Is(err, empleados.ErrEmpleadoNotFound) {
			writeEmpleadoError(w, err)
			return
		}
		d = &empleados.Documentos{EmpleadoID: empleadoID, EstatusValidacion: empleados.ObservacionPendiente}
	}
	setDocumentoURL(d, tipoDocumento, url)
	if d.EstatusValidacion == 0 {
		d.EstatusValidacion = empleados.ObservacionPendiente
	}
	if err := h.expedientes.UpsertDocumentos(r.Context(), d, empresaID); err != nil {
		writeEmpleadoError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": documentoResponse{EmpleadoID: empleadoID, TipoDocumento: tipoDocumento, URL: url, EstatusValidacion: d.EstatusValidacion, UploadedAt: time.Now().UTC()}})
}

// ListAsistencias handles GET /empleado/asistencias.
func (h *Handler) ListAsistencias(w http.ResponseWriter, r *http.Request) {
	empresaID, ok := parseEmpresaIDQuery(w, r)
	if !ok {
		return
	}
	empleadoID := parseOptionalInt64(r.URL.Query().Get("empleado_id"))
	var (
		list []empleados.Asistencia
		err  error
	)
	if empleadoID > 0 {
		list, err = h.asistencias.GetByEmpleado(r.Context(), empleadoID, empresaID)
	} else {
		list, err = h.asistencias.GetByEmpresa(r.Context(), empresaID)
	}
	if err != nil {
		writeEmpleadoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": list})
}

// CreateAsistencia handles POST /empleado/asistencias.
func (h *Handler) CreateAsistencia(w http.ResponseWriter, r *http.Request) {
	empresaID, ok := parseEmpresaIDQuery(w, r)
	if !ok {
		return
	}
	empleadoID := parseOptionalInt64(r.URL.Query().Get("empleado_id"))
	if err := r.ParseMultipartForm(maxAsistenciaBytes); err != nil {
		writeError(w, "BAD_REQUEST", "invalid multipart body", http.StatusBadRequest)
		return
	}
	if empleadoID <= 0 {
		empleadoID = parseOptionalInt64(r.FormValue("empleado_id"))
	}
	if empleadoID <= 0 {
		writeError(w, "BAD_REQUEST", "empleado_id is required", http.StatusBadRequest)
		return
	}
	tipo := strings.TrimSpace(r.FormValue("tipo_registro"))
	if !validTipoRegistro(tipo) {
		writeError(w, "BAD_REQUEST", "invalid tipo_registro", http.StatusBadRequest)
		return
	}
	estatusRango := parseOptionalInt(r.FormValue("estatus_rango"))
	if estatusRango == 0 {
		estatusRango = empleados.RangoEnRango
	}
	fotoURL, err := h.uploadMultipartFile(r, "foto", empleadoID, "asistencias", tipo, maxAsistenciaBytes)
	if err != nil {
		writeEmpleadoError(w, err)
		return
	}
	a := &empleados.Asistencia{EmpleadoID: empleadoID, TipoRegistro: tipo, Geolocalizacion: r.FormValue("geolocalizacion"), EstatusRango: estatusRango, FotoEntradaURL: fotoURL, HoraEntrada: defaultHoraEntrada}
	if err := h.asistencias.Create(r.Context(), a, empresaID); err != nil {
		writeEmpleadoError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": a})
}

// ListInasistencias handles GET /empleado/inasistencia.
func (h *Handler) ListInasistencias(w http.ResponseWriter, r *http.Request) {
	empresaID, ok := parseEmpresaIDQuery(w, r)
	if !ok {
		return
	}
	empleadoID := parseOptionalInt64(r.URL.Query().Get("empleado_id"))
	if empleadoID <= 0 {
		writeError(w, "BAD_REQUEST", "empleado_id is required", http.StatusBadRequest)
		return
	}
	list, err := h.inasistencias.GetByEmpleado(r.Context(), empleadoID, empresaID)
	if err != nil {
		writeEmpleadoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": list})
}

// CreateInasistencia handles POST /empleado/inasistencia.
func (h *Handler) CreateInasistencia(w http.ResponseWriter, r *http.Request) {
	empresaID, ok := parseEmpresaIDQuery(w, r)
	if !ok {
		return
	}
	var body struct {
		EmpleadoID         int64   `json:"empleado_id"`
		TipoInasistenciaID int64   `json:"tipo_inasistencia_id"`
		TipoIncidencia     string  `json:"tipo_incidencia"`
		FechaInicio        string  `json:"fecha_inicio"`
		FechaFin           string  `json:"fecha_fin"`
		Motivo             *string `json:"motivo"`
		ComprobanteURL     *string `json:"comprobante_url"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest)
		return
	}
	fechaInicio, err := parseDate(body.FechaInicio)
	if err != nil {
		writeError(w, "BAD_REQUEST", "invalid fecha_inicio", http.StatusBadRequest)
		return
	}
	fechaFin, err := parseDate(body.FechaFin)
	if err != nil {
		writeError(w, "BAD_REQUEST", "invalid fecha_fin", http.StatusBadRequest)
		return
	}
	tipo := body.TipoIncidencia
	if tipo == "" && body.TipoInasistenciaID > 0 {
		tipo = strconv.FormatInt(body.TipoInasistenciaID, 10)
	}
	i := &empleados.Inasistencia{EmpleadoID: body.EmpleadoID, TipoIncidencia: tipo, FechaInicio: fechaInicio, FechaFin: fechaFin, JustificanteURL: body.ComprobanteURL, Observaciones: body.Motivo}
	if err := h.inasistencias.Create(r.Context(), i, empresaID); err != nil {
		writeEmpleadoError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": i})
}

// ResolveInasistencia handles PUT /empleado/inasistencia/{id}.
func (h *Handler) ResolveInasistencia(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, "BAD_REQUEST", "invalid id", http.StatusBadRequest)
		return
	}
	empresaID, ok := parseEmpresaIDQuery(w, r)
	if !ok {
		return
	}
	var body struct {
		Status        string  `json:"status"`
		Aprobado      *bool   `json:"aprobado"`
		Observaciones *string `json:"observaciones"`
		ResolvedBy    *string `json:"resolved_by"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest)
		return
	}
	aprobado := false
	if body.Aprobado != nil {
		aprobado = *body.Aprobado
	} else if strings.EqualFold(body.Status, "APROBADA") {
		aprobado = true
	} else if !strings.EqualFold(body.Status, "RECHAZADA") {
		writeError(w, "BAD_REQUEST", "invalid status", http.StatusBadRequest)
		return
	}
	resolvedBy, _ := userIDFromContext(r.Context())
	if resolvedBy == uuid.Nil && body.ResolvedBy != nil {
		resolvedBy, _ = uuid.Parse(*body.ResolvedBy)
	}
	if err := h.inasistencias.UpdateStatus(r.Context(), id, empresaID, aprobado, body.Observaciones, resolvedBy); err != nil {
		writeEmpleadoError(w, err)
		return
	}
	status := "RECHAZADA"
	if aprobado {
		status = "APROBADA"
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"id": id, "status": status, "observaciones": body.Observaciones, "resolved_by": resolvedBy, "resolved_at": time.Now().UTC()}})
}

// ListDevices handles GET /empleado/devices.
func (h *Handler) ListDevices(w http.ResponseWriter, r *http.Request) {
	empresaID, ok := parseEmpresaIDQuery(w, r)
	if !ok {
		return
	}
	empleadoID := parseOptionalInt64(r.URL.Query().Get("empleado_id"))
	if empleadoID <= 0 {
		writeError(w, "BAD_REQUEST", "empleado_id is required", http.StatusBadRequest)
		return
	}
	devices, err := h.devices.GetByEmpleado(r.Context(), empleadoID, empresaID)
	if err != nil {
		writeEmpleadoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": devices})
}

// RegisterDevice handles POST /empleado/devices.
func (h *Handler) RegisterDevice(w http.ResponseWriter, r *http.Request) {
	empresaID, ok := parseEmpresaIDQuery(w, r)
	if !ok {
		return
	}
	var body struct {
		EmpleadoID    int64  `json:"empleado_id"`
		TokenFirebase string `json:"token_firebase"`
		DeviceID      string `json:"device_id"`
		DeviceType    string `json:"device_type"`
		DeviceName    string `json:"device_name"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest)
		return
	}
	model := optionalString(body.DeviceName)
	osVersion := optionalString(body.DeviceType)
	d := &empleados.UserDevice{EmpleadoID: body.EmpleadoID, DeviceUUID: body.DeviceID, DeviceModel: model, OSVersion: osVersion, FCMToken: body.TokenFirebase}
	if err := h.devices.Upsert(r.Context(), d, empresaID); err != nil {
		writeEmpleadoError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": d})
}

// GetFotologin handles GET /empleado/{id}/fotologin — returns biometric base photos.
func (h *Handler) GetFotologin(w http.ResponseWriter, r *http.Request) {
	empleadoID, empresaID, ok := parseEmpleadoEmpresa(w, r)
	if !ok {
		return
	}
	list, err := h.fotologin.ListFotologin(r.Context(), empleadoID, empresaID)
	if err != nil {
		writeEmpleadoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": list})
}

// UploadFotologin handles POST /empleado/{id}/fotologin — stores a new base biometric photo.
func (h *Handler) UploadFotologin(w http.ResponseWriter, r *http.Request) {
	empleadoID, empresaID, ok := parseEmpleadoEmpresa(w, r)
	if !ok {
		return
	}
	if err := r.ParseMultipartForm(maxAsistenciaBytes); err != nil {
		writeError(w, "BAD_REQUEST", "invalid multipart body", http.StatusBadRequest)
		return
	}
	url, err := h.uploadMultipartFile(r, "foto", empleadoID, "fotologin", "base", maxAsistenciaBytes)
	if err != nil {
		writeEmpleadoError(w, err)
		return
	}
	f := &empleados.Fotologin{EmpleadoID: empleadoID, FotoBaseURL: url}
	if err := h.fotologin.CreateFotologin(r.Context(), f, empresaID); err != nil {
		writeEmpleadoError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": f})
}

// GetKPIComplete handles GET /empresa/{empresa_id}/empleados/documentos/complete.
func (h *Handler) GetKPIComplete(w http.ResponseWriter, r *http.Request) {
	empresaID, ok := parseEmpresaIDPath(w, r)
	if !ok {
		return
	}
	count, err := h.kpi.KPIDocumentosComplete(r.Context(), empresaID)
	if err != nil {
		writeEmpleadoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"count": count})
}

// GetKPILack handles GET /empresa/{empresa_id}/empleados/documentos/lack.
func (h *Handler) GetKPILack(w http.ResponseWriter, r *http.Request) {
	empresaID, ok := parseEmpresaIDPath(w, r)
	if !ok {
		return
	}
	count, err := h.kpi.KPIDocumentosLack(r.Context(), empresaID)
	if err != nil {
		writeEmpleadoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"count": count})
}

// GetKPIPendiente handles GET /empresa/{empresa_id}/empleados/documentos/pendiente.
func (h *Handler) GetKPIPendiente(w http.ResponseWriter, r *http.Request) {
	empresaID, ok := parseEmpresaIDPath(w, r)
	if !ok {
		return
	}
	count, err := h.kpi.KPIDocumentosPendiente(r.Context(), empresaID)
	if err != nil {
		writeEmpleadoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"count": count})
}

func (h *Handler) uploadMultipartFile(r *http.Request, field string, empleadoID int64, folder, label string, maxBytes int64) (string, error) {
	if h.storage == nil {
		return "", errors.New("file storage is not configured")
	}
	file, header, err := r.FormFile(field)
	if err != nil {
		return "", empleados.ErrInvalidMIMEType
	}
	defer file.Close()
	if err := empstorage.ValidateSize(header.Size, maxBytes); err != nil {
		return "", err
	}
	payload, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return "", fmt.Errorf("read upload: %w", err)
	}
	if err := empstorage.ValidateSize(int64(len(payload)), maxBytes); err != nil {
		return "", err
	}
	if err := empstorage.ValidateMIME(payload[:min(len(payload), 8)]); err != nil {
		return "", err
	}
	contentType := header.Header.Get("Content-Type")
	key := fmt.Sprintf("%d/%s/%s_%d%s", empleadoID, folder, sanitizeKeyPart(label), time.Now().UTC().UnixNano(), strings.ToLower(filepath.Ext(header.Filename)))
	return h.storage.Upload(r.Context(), h.bucket, key, bytesReader(payload), int64(len(payload)), contentType)
}

func decodeJSON(r *http.Request, dst any) error {
	return json.NewDecoder(r.Body).Decode(dst)
}

type problemDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, code, msg string, status int) {
	writeJSON(w, status, map[string]any{"error": problemDetail{Code: code, Message: msg}})
}

func writeEmpleadoError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, empleados.ErrEmpleadoNotFound), errors.Is(err, empleados.ErrDeviceNotFound):
		writeError(w, "NOT_FOUND", err.Error(), http.StatusNotFound)
	case errors.Is(err, empleados.ErrDuplicateCURP), errors.Is(err, empleados.ErrDuplicateNSS), errors.Is(err, empleados.ErrDuplicateIdEmpleado), errors.Is(err, empleados.ErrEmpleadoAlreadyExists):
		writeError(w, "CONFLICT", err.Error(), http.StatusConflict)
	case errors.Is(err, empleados.ErrTenantMismatch):
		writeError(w, "FORBIDDEN", err.Error(), http.StatusForbidden)
	case errors.Is(err, empleados.ErrInvalidMIMEType), errors.Is(err, empleados.ErrFileTooLarge), errors.Is(err, service.ErrInvalidEmpleadoRequest):
		writeError(w, "BAD_REQUEST", err.Error(), http.StatusBadRequest)
	default:
		writeError(w, "INTERNAL_ERROR", "internal server error", http.StatusInternalServerError)
	}
}

func parseEmpleadoEmpresa(w http.ResponseWriter, r *http.Request) (int64, int64, bool) {
	empleadoID, ok := parseIDPath(w, r, "id")
	if !ok {
		return 0, 0, false
	}
	empresaID, ok := parseEmpresaIDQuery(w, r)
	if !ok {
		return 0, 0, false
	}
	return empleadoID, empresaID, true
}

func parseIDPath(w http.ResponseWriter, r *http.Request, key string) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, key), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, "BAD_REQUEST", "invalid "+key, http.StatusBadRequest)
		return 0, false
	}
	return id, true
}

func parseEmpresaIDPath(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "empresa_id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, "BAD_REQUEST", "invalid empresa_id", http.StatusBadRequest)
		return 0, false
	}
	return id, true
}

func parseEmpresaIDQuery(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id := parseOptionalInt64(r.URL.Query().Get("empresa_id"))
	if id <= 0 {
		writeError(w, "BAD_REQUEST", "invalid empresa_id", http.StatusBadRequest)
		return 0, false
	}
	return id, true
}

func parseOptionalInt(raw string) int {
	v, _ := strconv.Atoi(strings.TrimSpace(raw))
	return v
}

func parseOptionalInt64(raw string) int64 {
	v, _ := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	return v
}

func parseDate(raw string) (time.Time, error) {
	return time.Parse("2006-01-02", raw)
}

func userIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	raw, ok := middleware.UserIDFromContext(ctx)
	if !ok || raw == "" {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(raw)
	return id, err == nil
}

func paginationFromQuery(r *http.Request, total int) map[string]int {
	page := parseOptionalInt(r.URL.Query().Get("page"))
	if page <= 0 {
		page = 1
	}
	limit := parseOptionalInt(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 10
	}
	totalPages := 0
	if total > 0 {
		totalPages = (total + limit - 1) / limit
	}
	return map[string]int{"page": page, "limit": limit, "total_items": total, "total_pages": totalPages}
}

func validTipoRegistro(tipo string) bool {
	switch tipo {
	case empleados.TipoEntradaLaboral, empleados.TipoSalidaLaboral, empleados.TipoEntradaComida, empleados.TipoSalidaComida:
		return true
	default:
		return false
	}
}

func validDocumentoTipo(tipo string) bool {
	switch tipo {
	case "ACTA_NACIMIENTO", "INE", "COMPROBANTE_DOMICILIO", "CURP_PDF", "NSS_PDF", "CONTRATO":
		return true
	default:
		return false
	}
}

func setDocumentoURL(d *empleados.Documentos, tipo, url string) {
	switch tipo {
	case "ACTA_NACIMIENTO":
		d.ActaURL = &url
	case "INE":
		d.INEURL = &url
	case "COMPROBANTE_DOMICILIO":
		d.ComprobanteDomicilioURL = &url
	case "CURP_PDF":
		d.CURPPdfURL = &url
	case "NSS_PDF":
		d.NSSPdfURL = &url
	case "CONTRATO":
		d.ContratoURL = &url
	}
}

type documentoResponse struct {
	EmpleadoID        int64     `json:"empleado_id"`
	TipoDocumento     string    `json:"tipo_documento"`
	URL               string    `json:"url"`
	EstatusValidacion int       `json:"estatus_validacion"`
	UploadedAt        time.Time `json:"uploaded_at"`
}

func documentoResponses(d *empleados.Documentos) []documentoResponse {
	if d == nil {
		return []documentoResponse{}
	}
	items := []documentoResponse{}
	appendIf := func(tipo string, url *string) {
		if url != nil && *url != "" {
			items = append(items, documentoResponse{EmpleadoID: d.EmpleadoID, TipoDocumento: tipo, URL: *url, EstatusValidacion: d.EstatusValidacion, UploadedAt: d.UpdatedAt})
		}
	}
	appendIf("ACTA_NACIMIENTO", d.ActaURL)
	appendIf("INE", d.INEURL)
	appendIf("COMPROBANTE_DOMICILIO", d.ComprobanteDomicilioURL)
	appendIf("CURP_PDF", d.CURPPdfURL)
	appendIf("NSS_PDF", d.NSSPdfURL)
	appendIf("CONTRATO", d.ContratoURL)
	return items
}

func optionalString(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return &s
}

func sanitizeKeyPart(s string) string {
	s = strings.TrimSpace(strings.ToUpper(s))
	s = strings.ReplaceAll(s, "/", "_")
	return strings.ReplaceAll(s, " ", "_")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func bytesReader(b []byte) io.Reader { return bytes.NewReader(b) }
