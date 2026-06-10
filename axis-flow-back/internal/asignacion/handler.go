package asignacion

import (
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	"axis-flow-back/internal/middleware"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const jsonContentType = "application/json"

// HTTPHandler exposes assignment application use cases over the OpenAPI contract.
type HTTPHandler struct {
	service AssignmentService
}

// NewHTTPHandler creates an assignment HTTP handler.
func NewHTTPHandler(service AssignmentService) *HTTPHandler {
	return &HTTPHandler{service: service}
}

// RegisterRoutes registers /api/v1/asignacion endpoints on the provided router.
func (h *HTTPHandler) RegisterRoutes(r chi.Router) {
	r.Post("/api/v1/asignacion", h.CreateAssignment)
	r.Patch("/api/v1/asignacion/{id}", h.ModifyAssignment)
	r.Get("/api/v1/asignacion/by-cliente/{id}", h.ListByClient)
	r.Get("/api/v1/asignacion/active-by-supervisor/{supervisor_id}", h.ListActiveBySupervisor)
	r.Get("/api/v1/asignacion/{id}/actividades", h.ListActivitiesByAssignment)
	r.Patch("/api/v1/asignacion/actividad/{actividad_id}/evidencia", h.UploadActivityEvidence)
	r.Patch("/api/v1/asignacion/actividad/batch-status", h.BatchUpdateActivityStatus)
	r.Get("/api/v1/asignacion/supervisor/{supervisor_id}/herramientas", h.ListToolsBySupervisor)
	r.Post("/api/v1/asignacion/evaluacion", h.SubmitEvaluation)
	r.Get("/api/v1/asignacion/evaluacion/historial/{empleado_id}", h.GetEvaluationHistory)
}

// RegisterSubroutes registers endpoints under an existing /api/v1/asignacion route.
func (h *HTTPHandler) RegisterSubroutes(r chi.Router) {
	r.Post("/", h.CreateAssignment)
	r.Patch("/{id}", h.ModifyAssignment)
	r.Get("/by-cliente/{id}", h.ListByClient)
	r.Get("/active-by-supervisor/{supervisor_id}", h.ListActiveBySupervisor)
	r.Get("/{id}/actividades", h.ListActivitiesByAssignment)
	r.Patch("/actividad/{actividad_id}/evidencia", h.UploadActivityEvidence)
	r.Patch("/actividad/batch-status", h.BatchUpdateActivityStatus)
	r.Get("/supervisor/{supervisor_id}/herramientas", h.ListToolsBySupervisor)
	r.Post("/evaluacion", h.SubmitEvaluation)
	r.Get("/evaluacion/historial/{empleado_id}", h.GetEvaluationHistory)
}

// CreateAssignment handles POST /api/v1/asignacion.
func (h *HTTPHandler) CreateAssignment(w http.ResponseWriter, r *http.Request) {
	if !roleAllowed(r, "ADMIN_CHECK_ON", "ADMINISTRADOR", "AdminCheckOn", "Administrador", "COORDINATOR", "COORDINADOR") {
		writeAssignmentError(w, "FORBIDDEN", "forbidden", http.StatusForbidden)
		return
	}

	var req createAssignmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAssignmentError(w, "VALIDATION_ERROR", "invalid request payload", http.StatusBadRequest)
		return
	}
	input, err := req.toInput()
	if err != nil {
		writeAssignmentError(w, "VALIDATION_ERROR", err.Error(), http.StatusBadRequest)
		return
	}

	assignment, err := h.service.CreateAssignment(r.Context(), input)
	if err != nil {
		handleAssignmentError(w, err)
		return
	}
	writeAssignmentJSON(w, http.StatusCreated, assignmentEnvelope{Data: newAssignmentDetail(assignment)})
}

// ModifyAssignment handles PATCH /api/v1/asignacion/{id}.
func (h *HTTPHandler) ModifyAssignment(w http.ResponseWriter, r *http.Request) {
	if !roleAllowed(r, "ADMIN_CHECK_ON", "ADMINISTRADOR", "AdminCheckOn", "Administrador", "COORDINATOR", "COORDINADOR") {
		writeAssignmentError(w, "FORBIDDEN", "forbidden", http.StatusForbidden)
		return
	}

	id, err := parseUUIDParam(r, "id")
	if err != nil {
		writeAssignmentError(w, "VALIDATION_ERROR", "invalid assignment id", http.StatusBadRequest)
		return
	}
	var req modifyAssignmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAssignmentError(w, "VALIDATION_ERROR", "invalid request payload", http.StatusBadRequest)
		return
	}
	input, err := req.toInput()
	if err != nil {
		writeAssignmentError(w, "VALIDATION_ERROR", err.Error(), http.StatusBadRequest)
		return
	}

	assignment, err := h.service.ModifyAssignment(r.Context(), id, input)
	if err != nil {
		handleAssignmentError(w, err)
		return
	}
	writeAssignmentJSON(w, http.StatusOK, assignmentEnvelope{Data: newAssignmentDetail(assignment)})
}

// ListByClient handles GET /api/v1/asignacion/by-cliente/{id}.
func (h *HTTPHandler) ListByClient(w http.ResponseWriter, r *http.Request) {
	if !roleAllowed(r, "ADMIN_CHECK_ON", "ADMINISTRADOR", "AdminCheckOn", "Administrador", "COORDINATOR", "COORDINADOR", "CLIENT", "CLIENTE") {
		writeAssignmentError(w, "FORBIDDEN", "forbidden", http.StatusForbidden)
		return
	}
	clientID, err := parseUUIDParam(r, "id")
	if err != nil {
		writeAssignmentError(w, "VALIDATION_ERROR", "invalid client id", http.StatusBadRequest)
		return
	}
	filter, err := parseListFilter(r)
	if err != nil {
		writeAssignmentError(w, "VALIDATION_ERROR", err.Error(), http.StatusBadRequest)
		return
	}

	result, err := h.service.ListAssignmentsByClient(r.Context(), clientID, filter)
	if err != nil {
		handleAssignmentError(w, err)
		return
	}
	writeAssignmentJSON(w, http.StatusOK, assignmentListEnvelope{Data: newAssignmentDetails(result.Items), Pagination: result.Pagination})
}

// ListActiveBySupervisor handles GET /api/v1/asignacion/active-by-supervisor/{supervisor_id}.
func (h *HTTPHandler) ListActiveBySupervisor(w http.ResponseWriter, r *http.Request) {
	if !roleAllowed(r, "ADMIN_CHECK_ON", "ADMINISTRADOR", "AdminCheckOn", "Administrador", "SUPERVISOR") {
		writeAssignmentError(w, "FORBIDDEN", "forbidden", http.StatusForbidden)
		return
	}
	supervisorID, err := parseUUIDParam(r, "supervisor_id")
	if err != nil {
		writeAssignmentError(w, "VALIDATION_ERROR", "invalid supervisor id", http.StatusBadRequest)
		return
	}
	pagination, err := parsePagination(r)
	if err != nil {
		writeAssignmentError(w, "VALIDATION_ERROR", err.Error(), http.StatusBadRequest)
		return
	}

	result, err := h.service.ListActiveAssignmentsBySupervisor(r.Context(), supervisorID, pagination)
	if err != nil {
		handleAssignmentError(w, err)
		return
	}
	writeAssignmentJSON(w, http.StatusOK, assignmentListEnvelope{Data: newAssignmentDetails(result.Items), Pagination: result.Pagination})
}

type createAssignmentRequest struct {
	CompanyID  string `json:"empresa_id"`
	EmployeeID string `json:"empleado_id"`
	LocationID string `json:"localidad_id"`
	ServiceID  string `json:"servicio_id"`
	ShiftID    string `json:"turno_id"`
	StartDate  string `json:"fecha_inicio"`
	EndDate    string `json:"fecha_fin"`
	Status     *int   `json:"estatus"`
}

func (r createAssignmentRequest) toInput() (CreateAssignmentInput, error) {
	companyID, err := parseRequiredUUID(r.CompanyID, "empresa_id")
	if err != nil {
		return CreateAssignmentInput{}, err
	}
	employeeID, err := parseRequiredUUID(r.EmployeeID, "empleado_id")
	if err != nil {
		return CreateAssignmentInput{}, err
	}
	locationID, err := parseRequiredUUID(r.LocationID, "localidad_id")
	if err != nil {
		return CreateAssignmentInput{}, err
	}
	serviceID, err := parseRequiredUUID(r.ServiceID, "servicio_id")
	if err != nil {
		return CreateAssignmentInput{}, err
	}
	shiftID, err := parseRequiredUUID(r.ShiftID, "turno_id")
	if err != nil {
		return CreateAssignmentInput{}, err
	}
	startDate, err := parseRequiredDate(r.StartDate, "fecha_inicio")
	if err != nil {
		return CreateAssignmentInput{}, err
	}
	var endDate *time.Time
	if strings.TrimSpace(r.EndDate) != "" {
		parsed, err := time.Parse(DateLayout, r.EndDate)
		if err != nil {
			return CreateAssignmentInput{}, errors.New("fecha_fin must use YYYY-MM-DD")
		}
		endDate = &parsed
	}
	if r.Status != nil && *r.Status != int(AssignmentStatusActive) {
		return CreateAssignmentInput{}, errors.New("estatus must be 1 when creating assignments")
	}
	return CreateAssignmentInput{CompanyID: companyID, EmployeeID: employeeID, LocationID: locationID, ServiceID: serviceID, ShiftID: shiftID, StartDate: startDate, EndDate: endDate}, nil
}

type modifyAssignmentRequest struct {
	LocationID string `json:"localidad_id"`
	ShiftID    string `json:"turno_id"`
	EndDate    string `json:"fecha_fin"`
	Status     *int   `json:"estatus"`
}

func (r modifyAssignmentRequest) toInput() (ModifyAssignmentInput, error) {
	var input ModifyAssignmentInput
	if strings.TrimSpace(r.LocationID) != "" {
		id, err := uuid.Parse(r.LocationID)
		if err != nil {
			return ModifyAssignmentInput{}, errors.New("localidad_id must be a valid UUID")
		}
		input.LocationID = &id
	}
	if strings.TrimSpace(r.ShiftID) != "" {
		id, err := uuid.Parse(r.ShiftID)
		if err != nil {
			return ModifyAssignmentInput{}, errors.New("turno_id must be a valid UUID")
		}
		input.ShiftID = &id
	}
	if strings.TrimSpace(r.EndDate) != "" {
		parsed, err := time.Parse(DateLayout, r.EndDate)
		if err != nil {
			return ModifyAssignmentInput{}, errors.New("fecha_fin must use YYYY-MM-DD")
		}
		input.EndDate = &parsed
	}
	if r.Status != nil {
		status := AssignmentStatus(*r.Status)
		if status != AssignmentStatusActive && status != AssignmentStatusInactive {
			return ModifyAssignmentInput{}, errors.New("estatus must be 1 or 2")
		}
		input.Status = &status
	}
	return input, nil
}

type assignmentEnvelope struct {
	Data assignmentDetail `json:"data"`
}

type assignmentListEnvelope struct {
	Data       []assignmentDetail `json:"data"`
	Pagination PaginationResult   `json:"pagination"`
}

type assignmentDetail struct {
	ID                       string `json:"id"`
	EmployeeID               string `json:"empleado_id"`
	LocationID               string `json:"localidad_id"`
	ServiceID                string `json:"servicio_id"`
	ShiftID                  string `json:"turno_id"`
	StartDate                string `json:"fecha_inicio"`
	EndDate                  string `json:"fecha_fin,omitempty"`
	Status                   int    `json:"estatus"`
	IsCurrent                bool   `json:"ultima_asignacion"`
	GeneratedActivitiesCount int    `json:"actividades_generadas_count,omitempty"`
	GeneratedToolsCount      int    `json:"herramientas_generadas_count,omitempty"`
	UpdatedActivitiesCount   int    `json:"actividades_actualizadas_count,omitempty"`
	UpdatedToolsCount        int    `json:"herramientas_actualizadas_count,omitempty"`
	CreatedAt                string `json:"created_at"`
	UpdatedAt                string `json:"updated_at"`
}

func newAssignmentDetails(assignments []Assignment) []assignmentDetail {
	details := make([]assignmentDetail, 0, len(assignments))
	for i := range assignments {
		details = append(details, newAssignmentDetail(&assignments[i]))
	}
	return details
}

func newAssignmentDetail(assignment *Assignment) assignmentDetail {
	detail := assignmentDetail{
		ID:                       assignment.ID.String(),
		EmployeeID:               assignment.EmployeeID.String(),
		LocationID:               assignment.LocationID.String(),
		ServiceID:                assignment.ServiceID.String(),
		ShiftID:                  assignment.ShiftID.String(),
		StartDate:                assignment.StartDate.Format(DateLayout),
		Status:                   int(assignment.Status),
		IsCurrent:                assignment.IsCurrent,
		GeneratedActivitiesCount: assignment.GeneratedActivitiesCount,
		GeneratedToolsCount:      assignment.GeneratedToolsCount,
		UpdatedActivitiesCount:   assignment.UpdatedActivitiesCount,
		UpdatedToolsCount:        assignment.UpdatedToolsCount,
		CreatedAt:                assignment.CreatedAt.Format(time.RFC3339),
		UpdatedAt:                assignment.UpdatedAt.Format(time.RFC3339),
	}
	if assignment.EndDate != nil {
		detail.EndDate = assignment.EndDate.Format(DateLayout)
	}
	return detail
}

func parseRequiredUUID(value string, field string) (uuid.UUID, error) {
	if strings.TrimSpace(value) == "" {
		return uuid.Nil, errors.New(field + " is required")
	}
	id, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil, errors.New(field + " must be a valid UUID")
	}
	return id, nil
}

func parseRequiredDate(value string, field string) (time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, errors.New(field + " is required")
	}
	parsed, err := time.Parse(DateLayout, value)
	if err != nil {
		return time.Time{}, errors.New(field + " must use YYYY-MM-DD")
	}
	return parsed, nil
}

func parseUUIDParam(r *http.Request, name string) (uuid.UUID, error) {
	return uuid.Parse(chi.URLParam(r, name))
}

func parseListFilter(r *http.Request) (AssignmentListFilter, error) {
	pagination, err := parsePagination(r)
	if err != nil {
		return AssignmentListFilter{}, err
	}
	var onlyCurrent *bool
	if raw := strings.TrimSpace(r.URL.Query().Get("activo")); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			return AssignmentListFilter{}, errors.New("activo must be a boolean")
		}
		onlyCurrent = &parsed
	}
	return AssignmentListFilter{Pagination: pagination, OnlyCurrent: onlyCurrent}, nil
}

func parsePagination(r *http.Request) (Pagination, error) {
	page := 1
	limit := 20
	if raw := strings.TrimSpace(r.URL.Query().Get("page")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			return Pagination{}, errors.New("page must be greater than zero")
		}
		page = parsed
	}
	if raw := firstNonEmpty(r.URL.Query().Get("pageSize"), r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 100 {
			return Pagination{}, errors.New("pageSize must be between 1 and 100")
		}
		limit = parsed
	}
	return Pagination{Limit: limit, Offset: (page - 1) * limit}, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func roleAllowed(r *http.Request, allowed ...string) bool {
	role, ok := middleware.RoleFromContext(r.Context())
	if !ok {
		return false
	}
	for _, item := range allowed {
		if role == item {
			return true
		}
	}
	return false
}

func handleAssignmentError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidAssignmentDateWindow),
		errors.Is(err, ErrMissingAssignmentRequiredField),
		errors.Is(err, ErrInvalidActivityStatusFilter),
		errors.Is(err, ErrInvalidToolDeliveryStatusFilter),
		errors.Is(err, ErrInvalidEvaluationRating),
		errors.Is(err, ErrBatchUpdateEmpty),
		errors.Is(err, ErrInvalidEvidenceFile):
		writeAssignmentError(w, "VALIDATION_ERROR", err.Error(), http.StatusBadRequest)
	case errors.Is(err, ErrAssignmentNotFound), errors.Is(err, ErrActivityNotFound):
		writeAssignmentError(w, "NOT_FOUND", err.Error(), http.StatusNotFound)
	case errors.Is(err, ErrAssignmentConflict):
		writeAssignmentError(w, "ASSIGNMENT_CONFLICT", "assignment conflict", http.StatusConflict)
	case errors.Is(err, ErrAssignmentSupervisorScopeNotImplemented):
		writeAssignmentError(w, "NOT_IMPLEMENTED", ErrAssignmentSupervisorScopeNotImplemented.Error(), http.StatusNotImplemented)
	default:
		writeAssignmentError(w, "INTERNAL_ERROR", "internal server error", http.StatusInternalServerError)
	}
}

func writeAssignmentJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", jsonContentType)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeAssignmentError(w http.ResponseWriter, code string, message string, status int) {
	writeAssignmentJSON(w, status, map[string]any{
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	})
}

// ListActivitiesByAssignment handles GET /api/v1/asignacion/{id}/actividades.
func (h *HTTPHandler) ListActivitiesByAssignment(w http.ResponseWriter, r *http.Request) {
	if !roleAllowed(r, "EMPLOYEE", "EMPLEADO", "SUPERVISOR", "ADMIN_CHECK_ON", "ADMINISTRADOR", "AdminCheckOn", "Administrador") {
		writeAssignmentError(w, "FORBIDDEN", "forbidden", http.StatusForbidden)
		return
	}
	assignmentID, err := parseUUIDParam(r, "id")
	if err != nil {
		writeAssignmentError(w, "VALIDATION_ERROR", "invalid assignment id", http.StatusBadRequest)
		return
	}
	rawStatus := strings.TrimSpace(r.URL.Query().Get("estatus"))
	var status *ActivityStatus
	if rawStatus != "" {
		parsed, err := ActivityStatusFromString(rawStatus)
		if err != nil {
			writeAssignmentError(w, "VALIDATION_ERROR", err.Error(), http.StatusBadRequest)
			return
		}
		status = &parsed
	}

	activities, err := h.service.ListActivitiesByAssignment(r.Context(), assignmentID, status)
	if err != nil {
		handleAssignmentError(w, err)
		return
	}
	writeAssignmentJSON(w, http.StatusOK, activitiesEnvelope{Data: newActivityDetails(activities)})
}

// UploadActivityEvidence handles PATCH /api/v1/asignacion/actividad/{actividad_id}/evidencia.
func (h *HTTPHandler) UploadActivityEvidence(w http.ResponseWriter, r *http.Request) {
	if !roleAllowed(r, "EMPLOYEE", "EMPLEADO") {
		writeAssignmentError(w, "FORBIDDEN", "forbidden", http.StatusForbidden)
		return
	}
	activityID, err := parseUUIDParam(r, "actividad_id")
	if err != nil {
		writeAssignmentError(w, "VALIDATION_ERROR", "invalid activity id", http.StatusBadRequest)
		return
	}
	if err := r.ParseMultipartForm(6 << 20); err != nil {
		writeAssignmentError(w, "VALIDATION_ERROR", "invalid multipart payload", http.StatusBadRequest)
		return
	}

	files, openFiles, err := parseEvidenceFiles(r)
	if err != nil {
		writeAssignmentError(w, "VALIDATION_ERROR", err.Error(), http.StatusBadRequest)
		return
	}
	defer func() {
		for _, closer := range openFiles {
			_ = closer.Close()
		}
	}()
	lat, err := parseRequiredFloat(r, "latitud")
	if err != nil {
		lat, err = parseRequiredFloat(r, "lat")
		if err != nil {
			writeAssignmentError(w, "VALIDATION_ERROR", err.Error(), http.StatusBadRequest)
			return
		}
	}
	lon, err := parseRequiredFloat(r, "longitud")
	if err != nil {
		lon, err = parseRequiredFloat(r, "lon")
		if err != nil {
			writeAssignmentError(w, "VALIDATION_ERROR", err.Error(), http.StatusBadRequest)
			return
		}
	}
	employeeRaw := strings.TrimSpace(firstNonEmpty(r.FormValue("empleado_id"), r.FormValue("employee_id")))
	employeeID, err := parseRequiredUUID(employeeRaw, "empleado_id")
	if err != nil {
		writeAssignmentError(w, "VALIDATION_ERROR", err.Error(), http.StatusBadRequest)
		return
	}

	activity, err := h.service.UploadActivityEvidence(r.Context(), activityID, UploadActivityEvidenceInput{
		EmployeeID: employeeID,
		Latitude:   lat,
		Longitude:  lon,
		Comment:    strings.TrimSpace(r.FormValue("comentario")),
		Files:      files,
	})
	if err != nil {
		handleAssignmentError(w, err)
		return
	}
	writeAssignmentJSON(w, http.StatusOK, completedActivityEnvelope{Data: newCompletedActivity(activity)})
}

// BatchUpdateActivityStatus handles PATCH /api/v1/asignacion/actividad/batch-status.
func (h *HTTPHandler) BatchUpdateActivityStatus(w http.ResponseWriter, r *http.Request) {
	if !roleAllowed(r, "SUPERVISOR", "ADMIN_CHECK_ON", "ADMINISTRADOR", "AdminCheckOn", "Administrador") {
		writeAssignmentError(w, "FORBIDDEN", "forbidden", http.StatusForbidden)
		return
	}
	var req batchUpdateActivitiesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAssignmentError(w, "VALIDATION_ERROR", "invalid request payload", http.StatusBadRequest)
		return
	}
	updates, err := req.toUpdates()
	if err != nil {
		writeAssignmentError(w, "VALIDATION_ERROR", err.Error(), http.StatusBadRequest)
		return
	}
	updated, err := h.service.BatchUpdateActivityStatus(r.Context(), updates)
	if err != nil {
		handleAssignmentError(w, err)
		return
	}
	writeAssignmentJSON(w, http.StatusOK, batchUpdateActivitiesEnvelope{Data: newBatchUpdateResponse(updated)})
}

// ListToolsBySupervisor handles GET /api/v1/asignacion/supervisor/{supervisor_id}/herramientas.
func (h *HTTPHandler) ListToolsBySupervisor(w http.ResponseWriter, r *http.Request) {
	if !roleAllowed(r, "SUPERVISOR", "ADMIN_CHECK_ON", "ADMINISTRADOR", "AdminCheckOn", "Administrador") {
		writeAssignmentError(w, "FORBIDDEN", "forbidden", http.StatusForbidden)
		return
	}
	supervisorID, err := parseUUIDParam(r, "supervisor_id")
	if err != nil {
		writeAssignmentError(w, "VALIDATION_ERROR", "invalid supervisor id", http.StatusBadRequest)
		return
	}
	rawStatus := strings.TrimSpace(r.URL.Query().Get("estatus"))
	var status *ToolDeliveryStatus
	if rawStatus != "" {
		parsed, err := ToolDeliveryStatusFromString(rawStatus)
		if err != nil {
			writeAssignmentError(w, "VALIDATION_ERROR", err.Error(), http.StatusBadRequest)
			return
		}
		status = &parsed
	}
	tools, err := h.service.ListToolsBySupervisor(r.Context(), supervisorID, status)
	if err != nil {
		handleAssignmentError(w, err)
		return
	}
	writeAssignmentJSON(w, http.StatusOK, toolsEnvelope{Data: newToolDetails(tools)})
}

// SubmitEvaluation handles POST /api/v1/asignacion/evaluacion.
func (h *HTTPHandler) SubmitEvaluation(w http.ResponseWriter, r *http.Request) {
	if !roleAllowed(r, "SUPERVISOR", "ADMIN_CHECK_ON", "ADMINISTRADOR", "AdminCheckOn", "Administrador") {
		writeAssignmentError(w, "FORBIDDEN", "forbidden", http.StatusForbidden)
		return
	}
	var req submitEvaluationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAssignmentError(w, "VALIDATION_ERROR", "invalid request payload", http.StatusBadRequest)
		return
	}
	input, err := req.toInput(r)
	if err != nil {
		writeAssignmentError(w, "VALIDATION_ERROR", err.Error(), http.StatusBadRequest)
		return
	}
	evaluation, err := h.service.SubmitEvaluation(r.Context(), input)
	if err != nil {
		handleAssignmentError(w, err)
		return
	}
	writeAssignmentJSON(w, http.StatusCreated, evaluationEnvelope{Data: newEvaluationDetail(evaluation)})
}

// GetEvaluationHistory handles GET /api/v1/asignacion/evaluacion/historial/{empleado_id}.
func (h *HTTPHandler) GetEvaluationHistory(w http.ResponseWriter, r *http.Request) {
	if !roleAllowed(r, "SUPERVISOR", "EMPLOYEE", "EMPLEADO", "ADMIN_CHECK_ON", "ADMINISTRADOR", "AdminCheckOn", "Administrador") {
		writeAssignmentError(w, "FORBIDDEN", "forbidden", http.StatusForbidden)
		return
	}
	employeeID, err := parseUUIDParam(r, "empleado_id")
	if err != nil {
		writeAssignmentError(w, "VALIDATION_ERROR", "invalid employee id", http.StatusBadRequest)
		return
	}
	pagination, err := parsePagination(r)
	if err != nil {
		writeAssignmentError(w, "VALIDATION_ERROR", err.Error(), http.StatusBadRequest)
		return
	}
	result, err := h.service.GetEvaluationHistory(r.Context(), employeeID, pagination)
	if err != nil {
		handleAssignmentError(w, err)
		return
	}
	writeAssignmentJSON(w, http.StatusOK, evaluationHistoryEnvelope{Data: newEvaluationHistoryBody(result), Pagination: result.Pagination})
}

func parseEvidenceFiles(r *http.Request) ([]EvidenceFile, []multipart.File, error) {
	var files []EvidenceFile
	var openFiles []multipart.File
	for _, slot := range []int{1, 2, 3} {
		key := fmt.Sprintf("evidencia_%d", slot)
		headers := r.MultipartForm.File[key]
		if len(headers) == 0 {
			continue
		}
		for _, header := range headers {
			opened, openErr := header.Open()
			if openErr != nil {
				for _, c := range openFiles {
					_ = c.Close()
				}
				return nil, nil, fmt.Errorf("failed to open evidencia_%d: %w", slot, openErr)
			}
			files = append(files, EvidenceFile{
				Slot:        slot,
				Filename:    header.Filename,
				ContentType: header.Header.Get("Content-Type"),
				SizeBytes:   header.Size,
				Reader:      opened,
			})
			openFiles = append(openFiles, opened)
		}
	}
	return files, openFiles, nil
}

func parseRequiredFloat(r *http.Request, name string) (float64, error) {
	raw := strings.TrimSpace(r.FormValue(name))
	if raw == "" {
		return 0, errors.New(name + " is required")
	}
	parsed, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, errors.New(name + " must be a decimal number")
	}
	return parsed, nil
}

type batchUpdateActivitiesRequest struct {
	Actividades []batchUpdateActivitiesEntry `json:"actividades"`
}

type batchUpdateActivitiesEntry struct {
	ActividadID string `json:"actividad_id"`
	Estatus     string `json:"estatus"`
	Comentario  string `json:"comentario"`
}

func (r batchUpdateActivitiesRequest) toUpdates() ([]ActivityStatusUpdate, error) {
	if len(r.Actividades) == 0 {
		return nil, ErrBatchUpdateEmpty
	}
	if len(r.Actividades) > 100 {
		return nil, errors.New("actividades must contain at most 100 items")
	}
	updates := make([]ActivityStatusUpdate, 0, len(r.Actividades))
	for i, entry := range r.Actividades {
		activityID, err := parseRequiredUUID(entry.ActividadID, fmt.Sprintf("actividades[%d].actividad_id", i))
		if err != nil {
			return nil, err
		}
		status, err := ActivityStatusFromString(entry.Estatus)
		if err != nil {
			return nil, fmt.Errorf("actividades[%d].estatus: %w", i, err)
		}
		updates = append(updates, ActivityStatusUpdate{
			ActivityID: activityID,
			Status:     status,
			Comment:    entry.Comentario,
		})
	}
	return updates, nil
}

type submitEvaluationRequest struct {
	AsignacionID string                      `json:"asignacion_id"`
	EmpleadoID   string                      `json:"empleado_id"`
	Calificacion float64                     `json:"calificacion"`
	Comentarios  string                      `json:"comentarios"`
	Criterios    []submitEvaluationCriterion `json:"criterios"`
}

type submitEvaluationCriterion struct {
	Nombre string  `json:"nombre"`
	Puntos float64 `json:"puntos"`
}

func (r submitEvaluationRequest) toInput(httpRequest *http.Request) (SubmitEvaluationInput, error) {
	assignmentID, err := parseRequiredUUID(r.AsignacionID, "asignacion_id")
	if err != nil {
		return SubmitEvaluationInput{}, err
	}
	employeeID, err := parseRequiredUUID(r.EmpleadoID, "empleado_id")
	if err != nil {
		return SubmitEvaluationInput{}, err
	}
	evaluatorID, err := parseRequiredUUID(strings.TrimSpace(evaluatorIDFromContext(httpRequest)), "evaluador_id")
	if err != nil {
		return SubmitEvaluationInput{}, err
	}
	if r.Calificacion < 1.0 || r.Calificacion > 5.0 {
		return SubmitEvaluationInput{}, ErrInvalidEvaluationRating
	}
	criteria := make([]EvaluationCriterion, 0, len(r.Criterios))
	for i, c := range r.Criterios {
		if strings.TrimSpace(c.Nombre) == "" {
			return SubmitEvaluationInput{}, fmt.Errorf("criterios[%d].nombre is required", i)
		}
		criteria = append(criteria, EvaluationCriterion{Name: c.Nombre, Points: c.Puntos})
	}
	return SubmitEvaluationInput{
		AssignmentID: assignmentID,
		EmployeeID:   employeeID,
		EvaluatorID:  evaluatorID,
		Rating:       r.Calificacion,
		Comments:     strings.TrimSpace(r.Comentarios),
		Criteria:     criteria,
	}, nil
}

// evaluatorIDFromContext derives the authenticated supervisor/admin user ID that submitted the evaluation.
func evaluatorIDFromContext(r *http.Request) string {
	id, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		return ""
	}
	return id
}

type activitiesEnvelope struct {
	Data []activityDetail `json:"data"`
}

type activityDetail struct {
	ActividadID       string   `json:"actividad_id"`
	Nombre            string   `json:"nombre"`
	Descripcion       string   `json:"descripcion"`
	Estatus           string   `json:"estatus"`
	RequiereEvidencia bool     `json:"requiere_evidencia"`
	Evidencias        []string `json:"evidencias"`
	UbicacionCarga    *gpsBody `json:"ubicacion_carga,omitempty"`
	FechaLimite       string   `json:"fecha_limite,omitempty"`
	CompletadoAt      string   `json:"completed_at"`
}

type gpsBody struct {
	Latitud  float64 `json:"latitud"`
	Longitud float64 `json:"longitud"`
}

func newActivityDetails(activities []AssignedActivity) []activityDetail {
	out := make([]activityDetail, 0, len(activities))
	for i := range activities {
		out = append(out, newActivityDetail(&activities[i]))
	}
	return out
}

func newActivityDetail(activity *AssignedActivity) activityDetail {
	detail := activityDetail{
		ActividadID:       activity.ID.String(),
		Nombre:            activity.Description,
		Descripcion:       activity.Description,
		Estatus:           activity.Status.String(),
		RequiereEvidencia: true,
		Evidencias:        activity.EvidenceURLs(),
	}
	if activity.UploadLatitude != nil && activity.UploadLongitude != nil {
		detail.UbicacionCarga = &gpsBody{Latitud: *activity.UploadLatitude, Longitud: *activity.UploadLongitude}
	}
	if activity.ExecutionDate != nil {
		detail.CompletadoAt = activity.ExecutionDate.Format(time.RFC3339)
	}
	return detail
}

type completedActivityEnvelope struct {
	Data completedActivity `json:"data"`
}

type completedActivity struct {
	ActividadID    string   `json:"actividad_id"`
	Estatus        string   `json:"estatus"`
	Evidencias     []string `json:"evidencias"`
	UbicacionCarga gpsBody  `json:"ubicacion_carga"`
	Comentario     string   `json:"comentario,omitempty"`
	CompletadoAt   string   `json:"completed_at"`
}

func newCompletedActivity(activity *AssignedActivity) completedActivity {
	detail := completedActivity{
		ActividadID: activity.ID.String(),
		Estatus:     activity.Status.String(),
		Evidencias:  activity.EvidenceURLs(),
		Comentario:  activity.Comments,
	}
	if activity.UploadLatitude != nil {
		detail.UbicacionCarga.Latitud = *activity.UploadLatitude
	}
	if activity.UploadLongitude != nil {
		detail.UbicacionCarga.Longitud = *activity.UploadLongitude
	}
	if activity.ExecutionDate != nil {
		detail.CompletadoAt = activity.ExecutionDate.Format(time.RFC3339)
	}
	return detail
}

type batchUpdateActivitiesEnvelope struct {
	Data batchUpdateActivitiesBody `json:"data"`
}

type batchUpdateActivitiesBody struct {
	UpdatedCount int                              `json:"updated_count"`
	Actividades  []batchUpdateActivitiesEntryBody `json:"actividades"`
}

type batchUpdateActivitiesEntryBody struct {
	ActividadID string `json:"actividad_id"`
	Estatus     string `json:"estatus"`
	UpdatedAt   string `json:"updated_at"`
}

func newBatchUpdateResponse(updated []AssignedActivity) batchUpdateActivitiesBody {
	body := batchUpdateActivitiesBody{UpdatedCount: len(updated), Actividades: make([]batchUpdateActivitiesEntryBody, 0, len(updated))}
	for i := range updated {
		body.Actividades = append(body.Actividades, batchUpdateActivitiesEntryBody{
			ActividadID: updated[i].ID.String(),
			Estatus:     updated[i].Status.String(),
			UpdatedAt:   updated[i].UpdatedAt.Format(time.RFC3339),
		})
	}
	return body
}

type toolsEnvelope struct {
	Data []toolDetail `json:"data"`
}

type toolDetail struct {
	HerramientaAsignadaID string    `json:"herramienta_asignada_id"`
	AsignacionID          string    `json:"asignacion_id"`
	Empleado              refDetail `json:"empleado"`
	Herramienta           refDetail `json:"herramienta"`
	Cantidad              int       `json:"cantidad"`
	Estatus               string    `json:"estatus"`
	FechaEntrega          string    `json:"fecha_entrega,omitempty"`
	FechaDevolucion       string    `json:"fecha_devolucion,omitempty"`
}

type refDetail struct {
	ID     string `json:"id"`
	Nombre string `json:"nombre"`
	Codigo string `json:"codigo,omitempty"`
}

func newToolDetails(tools []AssignedTool) []toolDetail {
	out := make([]toolDetail, 0, len(tools))
	for i := range tools {
		out = append(out, newToolDetail(&tools[i]))
	}
	return out
}

func newToolDetail(tool *AssignedTool) toolDetail {
	detail := toolDetail{
		HerramientaAsignadaID: tool.ID.String(),
		AsignacionID:          tool.AssignmentID.String(),
		Empleado:              refDetail{ID: tool.AssignmentID.String(), Nombre: ""},
		Herramienta:           refDetail{ID: tool.ToolID.String(), Nombre: tool.Name},
		Cantidad:              tool.Quantity,
		Estatus:               tool.DeliveryStatus.String(),
	}
	if tool.DeliveredAt != nil {
		detail.FechaEntrega = tool.DeliveredAt.Format(time.RFC3339)
	}
	if tool.ReturnedAt != nil {
		detail.FechaDevolucion = tool.ReturnedAt.Format(time.RFC3339)
	}
	return detail
}

type evaluationEnvelope struct {
	Data evaluationDetail `json:"data"`
}

type evaluationDetail struct {
	EvaluacionID    string `json:"evaluacion_id"`
	AsignacionID    string `json:"asignacion_id"`
	EmpleadoID      string `json:"empleado_id"`
	EvaluadorID     string `json:"evaluador_id"`
	Calificacion    int    `json:"calificacion"`
	Comentarios     string `json:"comentarios,omitempty"`
	FechaEvaluacion string `json:"fecha_evaluacion"`
}

func newEvaluationDetail(evaluation *EmployeeEvaluation) evaluationDetail {
	return evaluationDetail{
		EvaluacionID:    evaluation.ID.String(),
		AsignacionID:    evaluation.AssignmentID.String(),
		EmpleadoID:      evaluation.EmployeeID.String(),
		EvaluadorID:     evaluation.EvaluatorID.String(),
		Calificacion:    evaluation.Rating,
		Comentarios:     evaluation.Comments,
		FechaEvaluacion: evaluation.EvaluationDate.Format(time.RFC3339),
	}
}

type evaluationHistoryEnvelope struct {
	Data       evaluationHistoryBody `json:"data"`
	Pagination PaginationResult      `json:"pagination"`
}

type evaluationHistoryBody struct {
	EmpleadoID           string                   `json:"empleado_id"`
	CalificacionPromedio float64                  `json:"calificacion_promedio"`
	EvaluacionesCount    int                      `json:"evaluaciones_count"`
	Historial            []evaluationHistoryEntry `json:"historial"`
}

type evaluationHistoryEntry struct {
	EvaluacionID    string    `json:"evaluacion_id"`
	Calificacion    int       `json:"calificacion"`
	Comentarios     string    `json:"comentarios,omitempty"`
	FechaEvaluacion string    `json:"fecha_evaluacion"`
	Evaluador       refDetail `json:"evaluador"`
}

func newEvaluationHistoryBody(result EvaluationHistoryResult) evaluationHistoryBody {
	body := evaluationHistoryBody{
		CalificacionPromedio: result.AverageRating,
		EvaluacionesCount:    result.EvaluationCount,
		Historial:            make([]evaluationHistoryEntry, 0, len(result.Items)),
	}
	for i := range result.Items {
		entry := evaluationHistoryEntry{
			EvaluacionID:    result.Items[i].ID.String(),
			Calificacion:    result.Items[i].Rating,
			Comentarios:     result.Items[i].Comments,
			FechaEvaluacion: result.Items[i].EvaluationDate.Format(time.RFC3339),
			Evaluador:       refDetail{ID: result.Items[i].EvaluatorID.String()},
		}
		if i == 0 && len(result.Items) > 0 {
			body.EmpleadoID = result.Items[i].EmployeeID.String()
		}
		body.Historial = append(body.Historial, entry)
	}
	return body
}
