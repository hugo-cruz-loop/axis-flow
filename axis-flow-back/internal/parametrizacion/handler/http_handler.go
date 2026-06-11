package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"axis-flow-back/internal/middleware"
	"axis-flow-back/internal/parametrizacion"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type HTTPHandler struct {
	service parametrizacion.ParametrizacionService
}

func NewHTTPHandler(service parametrizacion.ParametrizacionService) *HTTPHandler {
	return &HTTPHandler{service: service}
}
func (h *HTTPHandler) RegisterRoutes(r chi.Router) {
	r.Get("/api/v1/parametrizacion/evaluacion-servicio", h.ListEvaluacionServicio)
	r.Post("/api/v1/parametrizacion/evaluacion-servicio", h.ConfigurarEvaluacionServicio)
	r.Get("/api/v1/parametrizacion/evaluacion-personal/filtrar", h.ListEvaluacionPersonal)
	r.Post("/api/v1/parametrizacion/evaluacion-personal", h.ConfigurarEvaluacionPersonal)
	r.Post("/api/v1/parametrizacion/dias-inactivos", h.AgregarDiaInactivo)
	r.Delete("/api/v1/parametrizacion/dias-inactivos/{id}", h.EliminarDiaInactivo)
	r.Get("/api/v1/parametrizacion/dias-inactivos/empresa/{empresa_id}", h.GetDiasInactivosEmpresa)
	r.Post("/api/v1/parametrizacion/dias-inactivos/umbral", h.ConfigurarUmbral)
	r.Get("/api/v1/parametrizacion/sistema", h.ListSistema)
	r.Patch("/api/v1/parametrizacion/sistema/{clave}", h.UpdateSistema)
}
func (h *HTTPHandler) RegisterSubroutes(r chi.Router) {
	r.Get("/evaluacion-servicio", h.ListEvaluacionServicio)
	r.Post("/evaluacion-servicio", h.ConfigurarEvaluacionServicio)
	r.Get("/evaluacion-personal/filtrar", h.ListEvaluacionPersonal)
	r.Post("/evaluacion-personal", h.ConfigurarEvaluacionPersonal)
	r.Post("/dias-inactivos", h.AgregarDiaInactivo)
	r.Delete("/dias-inactivos/{id}", h.EliminarDiaInactivo)
	r.Get("/dias-inactivos/empresa/{empresa_id}", h.GetDiasInactivosEmpresa)
	r.Post("/dias-inactivos/umbral", h.ConfigurarUmbral)
	r.Get("/sistema", h.ListSistema)
	r.Patch("/sistema/{clave}", h.UpdateSistema)
}
func (h *HTTPHandler) ListEvaluacionServicio(w http.ResponseWriter, r *http.Request) {
	if !allowRead(r) {
		writeErr(w, http.StatusForbidden, "FORBIDDEN", "forbidden")
		return
	}
	empresaID, ok := queryEmpresa(w, r)
	if !ok || !tenantOK(w, r, empresaID) {
		return
	}
	out, err := h.service.ListEvaluacionServicio(r.Context(), empresaID)
	if err != nil {
		mapErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, mapEvaluacionServicioList(out))
}
func (h *HTTPHandler) ConfigurarEvaluacionServicio(w http.ResponseWriter, r *http.Request) {
	if !allowWrite(r) {
		writeErr(w, http.StatusForbidden, "FORBIDDEN", "forbidden")
		return
	}
	var req serviceEvalReq
	if decode(w, r, &req) || !tenantOK(w, r, req.EmpresaID) {
		return
	}
	out, err := h.service.ConfigurarEvaluacionServicio(r.Context(), parametrizacion.EvaluacionServicio{EmpresaID: req.EmpresaID, ServicioID: req.ServicioID, PeriodicidadID: req.PeriodicidadID, Activa: req.activa()})
	if err != nil {
		mapErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, mapEvaluacionServicio(out))
}
func (h *HTTPHandler) ListEvaluacionPersonal(w http.ResponseWriter, r *http.Request) {
	if !allowRead(r) {
		writeErr(w, http.StatusForbidden, "FORBIDDEN", "forbidden")
		return
	}
	empresaID, ok := queryEmpresa(w, r)
	if !ok || !tenantOK(w, r, empresaID) {
		return
	}
	out, err := h.service.ListEvaluacionPersonal(r.Context(), empresaID)
	if err != nil {
		mapErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, mapEvaluacionPersonalList(out))
}
func (h *HTTPHandler) ConfigurarEvaluacionPersonal(w http.ResponseWriter, r *http.Request) {
	if !allowWrite(r) {
		writeErr(w, http.StatusForbidden, "FORBIDDEN", "forbidden")
		return
	}
	var req personalEvalReq
	if decode(w, r, &req) || !tenantOK(w, r, req.EmpresaID) {
		return
	}
	out, err := h.service.ConfigurarEvaluacionPersonal(r.Context(), parametrizacion.EvaluacionPersonal{EmpresaID: req.EmpresaID, PeriodicidadID: req.PeriodicidadID, Activa: req.activa()})
	if err != nil {
		mapErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, mapEvaluacionPersonal(out))
}
func (h *HTTPHandler) AgregarDiaInactivo(w http.ResponseWriter, r *http.Request) {
	if !allowWrite(r) {
		writeErr(w, http.StatusForbidden, "FORBIDDEN", "forbidden")
		return
	}
	var req inactiveDayReq
	if decode(w, r, &req) || !tenantOK(w, r, req.EmpresaID) {
		return
	}
	fecha, err := time.Parse("2006-01-02", req.Fecha)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "fecha must use YYYY-MM-DD")
		return
	}
	out, err := h.service.AgregarDiaInactivo(r.Context(), parametrizacion.DiaInactivo{EmpresaID: req.EmpresaID, Fecha: fecha, Descripcion: req.Descripcion})
	if err != nil {
		mapErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, mapDia(out))
}
func (h *HTTPHandler) EliminarDiaInactivo(w http.ResponseWriter, r *http.Request) {
	if !allowWrite(r) {
		writeErr(w, http.StatusForbidden, "FORBIDDEN", "forbidden")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid id")
		return
	}
	empresaID, ok := queryEmpresa(w, r)
	if !ok || !tenantOK(w, r, empresaID) {
		return
	}
	if err := h.service.EliminarDiaInactivo(r.Context(), id, empresaID); err != nil {
		mapErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Resource deleted successfully."})
}
func (h *HTTPHandler) GetDiasInactivosEmpresa(w http.ResponseWriter, r *http.Request) {
	if !allowRead(r) {
		writeErr(w, http.StatusForbidden, "FORBIDDEN", "forbidden")
		return
	}
	empresaID, err := strconv.ParseInt(chi.URLParam(r, "empresa_id"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid empresa_id")
		return
	}
	if !tenantOK(w, r, empresaID) {
		return
	}
	year := time.Now().Year()
	if raw := r.URL.Query().Get("year"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			year = parsed
		} else {
			writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid year")
			return
		}
	}
	out, err := h.service.GetDiasInactivosEmpresa(r.Context(), empresaID, year)
	if err != nil {
		mapErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, mapCompanyDays(out))
}
func (h *HTTPHandler) ConfigurarUmbral(w http.ResponseWriter, r *http.Request) {
	if !allowWrite(r) {
		writeErr(w, http.StatusForbidden, "FORBIDDEN", "forbidden")
		return
	}
	var req umbralReq
	if decode(w, r, &req) || !tenantOK(w, r, req.EmpresaID) {
		return
	}
	out, err := h.service.ConfigurarUmbralDiasInactivos(r.Context(), parametrizacion.DiasInactivosUmbral{EmpresaID: req.EmpresaID, UmbralDias: req.UmbralDias})
	if err != nil {
		mapErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, mapUmbral(out))
}
func (h *HTTPHandler) ListSistema(w http.ResponseWriter, r *http.Request) {
	if !allowWrite(r) {
		writeErr(w, http.StatusForbidden, "FORBIDDEN", "forbidden")
		return
	}
	out, err := h.service.ListSistemaParametros(r.Context())
	if err != nil {
		mapErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, mapSistemaList(out))
}
func (h *HTTPHandler) UpdateSistema(w http.ResponseWriter, r *http.Request) {
	if !allowWrite(r) {
		writeErr(w, http.StatusForbidden, "FORBIDDEN", "forbidden")
		return
	}
	var req systemReq
	if decode(w, r, &req) {
		return
	}
	actor := actorFromRequest(r)
	out, err := h.service.UpsertSistemaParametro(r.Context(), parametrizacion.SistemaParametro{ClaveParametro: chi.URLParam(r, "clave"), Valor: req.Valor}, actor)
	if err != nil {
		mapErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, mapSistema(out))
}

type serviceEvalReq struct {
	EmpresaID      int64 `json:"empresa_id"`
	ServicioID     int64 `json:"servicio_id"`
	PeriodicidadID int64 `json:"periodicidad_id"`
	Activa         *bool `json:"activa"`
}

func (r serviceEvalReq) activa() bool {
	if r.Activa == nil {
		return true
	}
	return *r.Activa
}

type personalEvalReq struct {
	EmpresaID      int64 `json:"empresa_id"`
	PeriodicidadID int64 `json:"periodicidad_id"`
	Activa         *bool `json:"activa"`
}

func (r personalEvalReq) activa() bool {
	if r.Activa == nil {
		return true
	}
	return *r.Activa
}

type inactiveDayReq struct {
	EmpresaID   int64  `json:"empresa_id"`
	Fecha       string `json:"fecha"`
	Descripcion string `json:"descripcion"`
}
type umbralReq struct {
	EmpresaID  int64 `json:"empresa_id"`
	UmbralDias int   `json:"umbral_dias"`
}
type systemReq struct {
	Valor string `json:"valor"`
}

func decode(w http.ResponseWriter, r *http.Request, dest any) bool {
	if err := json.NewDecoder(r.Body).Decode(dest); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request payload")
		return true
	}
	return false
}
func queryEmpresa(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.URL.Query().Get("empresa_id"), 10, 64)
	if err != nil || id <= 0 {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "empresa_id is required")
		return 0, false
	}
	return id, true
}
func tenantOK(w http.ResponseWriter, r *http.Request, empresaID int64) bool {
	tenant, err := tenantEmpresaID(r)
	if err != nil {
		writeErr(w, http.StatusForbidden, "FORBIDDEN", "forbidden")
		return false
	}
	if tenant != empresaID {
		writeErr(w, http.StatusForbidden, "FORBIDDEN", "tenant mismatch")
		return false
	}
	return true
}
func tenantEmpresaID(r *http.Request) (int64, error) {
	switch v := r.Context().Value(middleware.ContextKeyTenantID).(type) {
	case int64:
		return v, nil
	case string:
		return strconv.ParseInt(v, 10, 64)
	default:
		return 0, errors.New("missing tenant")
	}
}
func allowRead(r *http.Request) bool {
	return roleIn(r, "ADMIN_CHECK_ON", "ADMINISTRADOR", "AdminCheckOn", "Administrador", "CLIENT", "CLIENTE", "RH")
}
func allowWrite(r *http.Request) bool {
	return roleIn(r, "ADMIN_CHECK_ON", "ADMINISTRADOR", "AdminCheckOn", "Administrador")
}
func roleIn(r *http.Request, allowed ...string) bool {
	role, _ := middleware.RoleFromContext(r.Context())
	for _, a := range allowed {
		if role == a {
			return true
		}
	}
	return false
}
func actorFromRequest(r *http.Request) parametrizacion.Actor {
	var id uuid.UUID
	if raw, ok := r.Context().Value(middleware.ContextKeyUserID).(string); ok {
		id, _ = uuid.Parse(raw)
	}
	return parametrizacion.Actor{UserID: id, IPAddress: r.RemoteAddr, TraceID: middleware.TraceIDFromContext(r.Context())}
}
func mapErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, parametrizacion.ErrNotFound):
		writeErr(w, http.StatusNotFound, "NOT_FOUND", err.Error())
	case errors.Is(err, parametrizacion.ErrForbidden), errors.Is(err, parametrizacion.ErrTenantMismatch):
		writeErr(w, http.StatusForbidden, "FORBIDDEN", err.Error())
	case errors.Is(err, parametrizacion.ErrInvalidInput):
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
	default:
		writeErr(w, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
	}
}
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": data})
}
func writeErr(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": map[string]any{"code": code, "message": msg}})
}
func mapEvaluacionServicio(c *parametrizacion.EvaluacionServicio) map[string]any {
	return map[string]any{"id": c.ID, "empresa_id": c.EmpresaID, "servicio_id": c.ServicioID, "periodicidad_id": c.PeriodicidadID, "activa": c.Activa, "created_at": c.CreatedAt, "updated_at": c.UpdatedAt}
}
func mapEvaluacionServicioList(list []*parametrizacion.EvaluacionServicio) []map[string]any {
	out := make([]map[string]any, 0, len(list))
	for _, v := range list {
		out = append(out, mapEvaluacionServicio(v))
	}
	return out
}
func mapEvaluacionPersonal(c *parametrizacion.EvaluacionPersonal) map[string]any {
	return map[string]any{"id": c.ID, "empresa_id": c.EmpresaID, "periodicidad_id": c.PeriodicidadID, "activa": c.Activa, "created_at": c.CreatedAt, "updated_at": c.UpdatedAt}
}
func mapEvaluacionPersonalList(list []*parametrizacion.EvaluacionPersonal) []map[string]any {
	out := make([]map[string]any, 0, len(list))
	for _, v := range list {
		out = append(out, mapEvaluacionPersonal(v))
	}
	return out
}
func mapDia(d *parametrizacion.DiaInactivo) map[string]any {
	return map[string]any{"id": d.ID, "empresa_id": d.EmpresaID, "fecha": d.Fecha.Format("2006-01-02"), "descripcion": d.Descripcion, "created_at": d.CreatedAt, "updated_at": d.UpdatedAt}
}
func mapCompanyDays(d *parametrizacion.CompanyInactiveDays) map[string]any {
	days := make([]map[string]any, 0, len(d.DiasInactivos))
	for _, v := range d.DiasInactivos {
		days = append(days, mapDia(v))
	}
	return map[string]any{"empresa_id": d.EmpresaID, "umbral_dias": d.UmbralDias, "dias_inactivos": days}
}
func mapUmbral(u *parametrizacion.DiasInactivosUmbral) map[string]any {
	return map[string]any{"empresa_id": u.EmpresaID, "umbral_dias": u.UmbralDias, "updated_at": u.UpdatedAt}
}
func mapSistema(s *parametrizacion.SistemaParametro) map[string]any {
	return map[string]any{"clave_parametro": s.ClaveParametro, "valor": s.Valor, "descripcion": s.Descripcion, "created_at": s.CreatedAt, "updated_at": s.UpdatedAt}
}
func mapSistemaList(list []*parametrizacion.SistemaParametro) []map[string]any {
	out := make([]map[string]any, 0, len(list))
	for _, v := range list {
		out = append(out, mapSistema(v))
	}
	return out
}
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
