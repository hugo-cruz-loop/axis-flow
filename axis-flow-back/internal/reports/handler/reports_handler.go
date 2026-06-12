package handler

import (
	"encoding/json"
	"net/http"
	"regexp"

	"axis-flow-back/internal/reports/events"
	"axis-flow-back/internal/reports/service"
)

var reDate = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// ReportsHandler handles HTTP requests for the Reports module.
type ReportsHandler struct {
	reportsSvc service.ReportsService
	geoSvc     service.GeocodingService
	publisher  events.EventPublisher
}

// NewReportsHandler constructs a ReportsHandler.
func NewReportsHandler(
	reportsSvc service.ReportsService,
	geoSvc service.GeocodingService,
	publisher events.EventPublisher,
) *ReportsHandler {
	return &ReportsHandler{
		reportsSvc: reportsSvc,
		geoSvc:     geoSvc,
		publisher:  publisher,
	}
}

// GetEvidencias handles GET /api/v1/reports/evidencias.
// empresa_id is always sourced from the JWT claim, never from query params.
func (h *ReportsHandler) GetEvidencias(w http.ResponseWriter, r *http.Request) {
	empresaID, err := extractTenantID(r)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid tenant")
		return
	}
	role := extractRole(r)

	// Optional filters.
	clienteIDStr := r.URL.Query().Get("cliente_id")
	var clienteID *string
	if clienteIDStr != "" {
		if hasSuspiciousPattern(clienteIDStr, "cliente_id") || !isValidUUID(clienteIDStr) {
			respondError(w, http.StatusBadRequest, "INVALID_PARAM", "cliente_id must be a valid UUID")
			return
		}
		clienteID = &clienteIDStr
	}

	fechaStr := r.URL.Query().Get("fecha")
	var fecha *string
	if fechaStr != "" {
		if hasSuspiciousPattern(fechaStr, "fecha") || !reDate.MatchString(fechaStr) {
			respondError(w, http.StatusBadRequest, "INVALID_PARAM", "fecha must be YYYY-MM-DD")
			return
		}
		fecha = &fechaStr
	}

	page := queryInt(r, "page", 1)
	limit := queryInt(r, "limit", 10)

	items, total, err := h.reportsSvc.GetEvidencias(r.Context(), empresaID, role, clienteID, fecha, page, limit)
	if err != nil {
		status, code := mapReportsError(err)
		respondError(w, status, code, err.Error())
		return
	}

	respondPaginated(w, http.StatusOK, items, page, limit, total)
}

// GetAsistencias handles GET /api/v1/reports/asistencias.
func (h *ReportsHandler) GetAsistencias(w http.ResponseWriter, r *http.Request) {
	empresaID, err := extractTenantID(r)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid tenant")
		return
	}
	role := extractRole(r)

	employeeID := queryString(r, "employee_id")
	status := queryString(r, "status")
	dateFrom := queryString(r, "date_from")
	dateTo := queryString(r, "date_to")
	page := queryInt(r, "page", 1)
	limit := queryInt(r, "limit", 20)

	items, total, err := h.reportsSvc.GetAsistencias(r.Context(), empresaID, role, employeeID, status, dateFrom, dateTo, page, limit)
	if err != nil {
		status2, code := mapReportsError(err)
		respondError(w, status2, code, err.Error())
		return
	}

	respondPaginated(w, http.StatusOK, items, page, limit, total)
}

// GetGraficaEvidencia handles GET /api/v1/reports/graficaevidencia.
func (h *ReportsHandler) GetGraficaEvidencia(w http.ResponseWriter, r *http.Request) {
	empresaID, err := extractTenantID(r)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid tenant")
		return
	}

	clienteID := queryString(r, "cliente_id")

	stats, err := h.reportsSvc.GetGraficaEvidencia(r.Context(), empresaID, clienteID)
	if err != nil {
		status, code := mapReportsError(err)
		respondError(w, status, code, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, stats)
}

// GetCountIncidentes handles GET /api/v1/reports/countincidentes.
func (h *ReportsHandler) GetCountIncidentes(w http.ResponseWriter, r *http.Request) {
	empresaID, err := extractTenantID(r)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid tenant")
		return
	}

	counts, err := h.reportsSvc.GetCountIncidentes(r.Context(), empresaID)
	if err != nil {
		status, code := mapReportsError(err)
		respondError(w, status, code, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, counts)
}

// reverseGeocodeRequest is the body for POST /api/v1/reports/geocoding/reverse.
type reverseGeocodeRequest struct {
	Latitud  float64 `json:"latitud"`
	Longitud float64 `json:"longitud"`
}

// ReverseGeocode handles POST /api/v1/reports/geocoding/reverse.
func (h *ReportsHandler) ReverseGeocode(w http.ResponseWriter, r *http.Request) {
	_, err := extractTenantID(r)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid tenant")
		return
	}

	var req reverseGeocodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid request body")
		return
	}

	if req.Latitud < -90 || req.Latitud > 90 {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", "latitud must be between -90 and 90")
		return
	}
	if req.Longitud < -180 || req.Longitud > 180 {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", "longitud must be between -180 and 180")
		return
	}

	result, err := h.geoSvc.Reverse(r.Context(), req.Latitud, req.Longitud)
	if err != nil {
		status, code := mapReportsError(err)
		respondError(w, status, code, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"latitud":   result.Latitud,
		"longitud":  result.Longitud,
		"direccion": result.Direccion,
		"cached":    result.Cached,
	})
}
