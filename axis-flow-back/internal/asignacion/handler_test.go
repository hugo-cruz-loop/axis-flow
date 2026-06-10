package asignacion_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"
	"time"

	"axis-flow-back/internal/asignacion"
	"axis-flow-back/internal/middleware"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAssignmentHandlerCreateContract(t *testing.T) {
	fixedID := uuid.MustParse("c7a85f64-5717-4562-b3fc-2c963f66afb0")
	createdAt := time.Date(2026, 6, 8, 18, 40, 0, 0, time.UTC)
	service := &assignmentHTTPServiceStub{
		created: &asignacion.Assignment{
			ID:         fixedID,
			EmployeeID: uuid.MustParse("3fa85f64-5717-4562-b3fc-2c963f66afa6"),
			LocationID: uuid.MustParse("3fa85f64-5717-4562-b3fc-2c963f66afa7"),
			ServiceID:  uuid.MustParse("3fa85f64-5717-4562-b3fc-2c963f66afa8"),
			ShiftID:    uuid.MustParse("3fa85f64-5717-4562-b3fc-2c963f66afa9"),
			StartDate:  mustDate(t, "2026-06-09"),
			EndDate:    datePtr(t, "2026-06-30"),
			Status:     asignacion.AssignmentStatusActive,
			IsCurrent:  true,
			CreatedAt:  createdAt,
			UpdatedAt:  createdAt,
		},
	}
	recorder := performAssignmentRequest(t, service, http.MethodPost, "/api/v1/asignacion", `{
		"empresa_id":"2fa85f64-5717-4562-b3fc-2c963f66afa5",
		"empleado_id":"3fa85f64-5717-4562-b3fc-2c963f66afa6",
		"localidad_id":"3fa85f64-5717-4562-b3fc-2c963f66afa7",
		"servicio_id":"3fa85f64-5717-4562-b3fc-2c963f66afa8",
		"turno_id":"3fa85f64-5717-4562-b3fc-2c963f66afa9",
		"fecha_inicio":"2026-06-09",
		"fecha_fin":"2026-06-30",
		"estatus":1
	}`, "ADMIN_CHECK_ON")

	require.Equal(t, http.StatusCreated, recorder.Code)
	assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))
	var body map[string]map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	assert.Equal(t, fixedID.String(), body["data"]["id"])
	assert.Equal(t, true, body["data"]["ultima_asignacion"])
	assert.Equal(t, float64(1), body["data"]["estatus"])
	assert.Equal(t, "2026-06-09", body["data"]["fecha_inicio"])
	assert.Equal(t, uuid.MustParse("2fa85f64-5717-4562-b3fc-2c963f66afa5"), service.createInput.CompanyID)
	assert.Equal(t, service.created.EmployeeID, service.createInput.EmployeeID)
}

func TestAssignmentHandlerCreateMapsEmpresaIDThroughApplicationService(t *testing.T) {
	companyID := uuid.MustParse("2fa85f64-5717-4562-b3fc-2c963f66afa5")
	employeeID := uuid.MustParse("3fa85f64-5717-4562-b3fc-2c963f66afa6")
	repository := &assignmentRepositorySpy{}
	service := asignacion.NewAssignmentApplicationService(repository, nil, fixedClock(time.Date(2026, 6, 8, 18, 40, 0, 0, time.UTC)))

	recorder := performAssignmentRequest(t, service, http.MethodPost, "/api/v1/asignacion", `{
		"empresa_id":"2fa85f64-5717-4562-b3fc-2c963f66afa5",
		"empleado_id":"3fa85f64-5717-4562-b3fc-2c963f66afa6",
		"localidad_id":"3fa85f64-5717-4562-b3fc-2c963f66afa7",
		"servicio_id":"3fa85f64-5717-4562-b3fc-2c963f66afa8",
		"turno_id":"3fa85f64-5717-4562-b3fc-2c963f66afa9",
		"fecha_inicio":"2026-06-09",
		"estatus":1
	}`, "ADMIN_CHECK_ON")

	require.Equal(t, http.StatusCreated, recorder.Code)
	assert.Equal(t, companyID, repository.created.CompanyID)
	assert.Equal(t, employeeID, repository.created.EmployeeID)
}

func TestAssignmentHandlerCreateRejectsInactiveStatus(t *testing.T) {
	service := &assignmentHTTPServiceStub{created: &asignacion.Assignment{ID: uuid.New()}}
	recorder := performAssignmentRequest(t, service, http.MethodPost, "/api/v1/asignacion", `{
		"empresa_id":"2fa85f64-5717-4562-b3fc-2c963f66afa5",
		"empleado_id":"3fa85f64-5717-4562-b3fc-2c963f66afa6",
		"localidad_id":"3fa85f64-5717-4562-b3fc-2c963f66afa7",
		"servicio_id":"3fa85f64-5717-4562-b3fc-2c963f66afa8",
		"turno_id":"3fa85f64-5717-4562-b3fc-2c963f66afa9",
		"fecha_inicio":"2026-06-09",
		"estatus":2
	}`, "ADMIN_CHECK_ON")

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.False(t, service.createCalled)
	assert.JSONEq(t, `{"error":{"code":"VALIDATION_ERROR","message":"estatus must be 1 when creating assignments"}}`, recorder.Body.String())
}

func TestAssignmentHandlerCreateRejectsUnauthorizedRole(t *testing.T) {
	service := &assignmentHTTPServiceStub{}
	recorder := performAssignmentRequest(t, service, http.MethodPost, "/api/v1/asignacion", `{}`, "SUPERVISOR")

	assert.Equal(t, http.StatusForbidden, recorder.Code)
	assert.False(t, service.createCalled)
	assert.JSONEq(t, `{"error":{"code":"FORBIDDEN","message":"forbidden"}}`, recorder.Body.String())
}

func TestAssignmentHandlerModifyContract(t *testing.T) {
	assignmentID := uuid.MustParse("c7a85f64-5717-4562-b3fc-2c963f66afb0")
	updated := time.Date(2026, 6, 8, 18, 45, 0, 0, time.UTC)
	service := &assignmentHTTPServiceStub{
		modified: &asignacion.Assignment{
			ID:         assignmentID,
			EmployeeID: uuid.MustParse("3fa85f64-5717-4562-b3fc-2c963f66afa6"),
			LocationID: uuid.MustParse("3fa85f64-5717-4562-b3fc-2c963f66afa7"),
			ServiceID:  uuid.MustParse("3fa85f64-5717-4562-b3fc-2c963f66afa8"),
			ShiftID:    uuid.MustParse("3fa85f64-5717-4562-b3fc-2c963f66afa9"),
			StartDate:  mustDate(t, "2026-06-09"),
			EndDate:    datePtr(t, "2026-07-15"),
			Status:     asignacion.AssignmentStatusInactive,
			IsCurrent:  true,
			CreatedAt:  updated.Add(-5 * time.Minute),
			UpdatedAt:  updated,
		},
	}
	recorder := performAssignmentRequest(t, service, http.MethodPatch, "/api/v1/asignacion/"+assignmentID.String(), `{
		"localidad_id":"3fa85f64-5717-4562-b3fc-2c963f66afa7",
		"turno_id":"3fa85f64-5717-4562-b3fc-2c963f66afa9",
		"fecha_fin":"2026-07-15",
		"estatus":2
	}`, "COORDINATOR")

	require.Equal(t, http.StatusOK, recorder.Code)
	var body map[string]map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	assert.Equal(t, assignmentID.String(), body["data"]["id"])
	assert.Equal(t, float64(2), body["data"]["estatus"])
	assert.Equal(t, "2026-07-15", body["data"]["fecha_fin"])
	assert.Equal(t, assignmentID, service.modifyID)
	require.NotNil(t, service.modifyInput.Status)
	assert.Equal(t, asignacion.AssignmentStatusInactive, *service.modifyInput.Status)
}

func TestAssignmentHandlerListByClientContract(t *testing.T) {
	clientID := uuid.MustParse("9fa85f64-5717-4562-b3fc-2c963f66afa1")
	service := &assignmentHTTPServiceStub{
		listResult: asignacion.AssignmentListResult{
			Items: []asignacion.Assignment{{
				ID:         uuid.MustParse("c7a85f64-5717-4562-b3fc-2c963f66afb0"),
				EmployeeID: uuid.MustParse("3fa85f64-5717-4562-b3fc-2c963f66afa6"),
				LocationID: uuid.MustParse("3fa85f64-5717-4562-b3fc-2c963f66afa7"),
				ServiceID:  uuid.MustParse("3fa85f64-5717-4562-b3fc-2c963f66afa8"),
				ShiftID:    uuid.MustParse("3fa85f64-5717-4562-b3fc-2c963f66afa9"),
				StartDate:  mustDate(t, "2026-06-09"),
				Status:     asignacion.AssignmentStatusActive,
				IsCurrent:  true,
			}},
			Pagination: asignacion.PaginationResult{Page: 2, Limit: 5, TotalRecords: 6, TotalPages: 2},
		},
	}
	recorder := performAssignmentRequest(t, service, http.MethodGet, "/api/v1/asignacion/by-cliente/"+clientID.String()+"?activo=true&page=2&limit=5", "", "CLIENT")

	require.Equal(t, http.StatusOK, recorder.Code)
	var body struct {
		Data       []map[string]any `json:"data"`
		Pagination map[string]any   `json:"pagination"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Len(t, body.Data, 1)
	assert.Equal(t, float64(2), body.Pagination["page"])
	assert.Equal(t, float64(6), body.Pagination["total_records"])
	assert.Equal(t, clientID, service.listClientID)
	require.NotNil(t, service.listFilter.OnlyCurrent)
	assert.True(t, *service.listFilter.OnlyCurrent)
	assert.Equal(t, 5, service.listFilter.Limit)
	assert.Equal(t, 5, service.listFilter.Offset)
}

func TestAssignmentHandlerActiveBySupervisorContract(t *testing.T) {
	supervisorID := uuid.MustParse("8fa85f64-5717-4562-b3fc-2c963f66afa2")
	service := &assignmentHTTPServiceStub{
		listResult: asignacion.AssignmentListResult{
			Items:      []asignacion.Assignment{},
			Pagination: asignacion.PaginationResult{Page: 1, Limit: 10, TotalRecords: 0, TotalPages: 0},
		},
	}
	recorder := performAssignmentRequest(t, service, http.MethodGet, "/api/v1/asignacion/active-by-supervisor/"+supervisorID.String(), "", "SUPERVISOR")

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.JSONEq(t, `{"data":[],"pagination":{"page":1,"limit":10,"total_records":0,"total_pages":0}}`, recorder.Body.String())
	assert.Equal(t, supervisorID, service.supervisorID)
}

func TestAssignmentHandlerActiveBySupervisorReturnsNotImplementedWhenScopeRelationIsMissing(t *testing.T) {
	repository := &assignmentRepositorySpy{}
	service := asignacion.NewAssignmentApplicationService(repository, nil, fixedClock(time.Date(2026, 6, 8, 18, 40, 0, 0, time.UTC)))
	supervisorID := uuid.MustParse("8fa85f64-5717-4562-b3fc-2c963f66afa2")

	recorder := performAssignmentRequest(t, service, http.MethodGet, "/api/v1/asignacion/active-by-supervisor/"+supervisorID.String(), "", "SUPERVISOR")

	require.Equal(t, http.StatusNotImplemented, recorder.Code)
	assert.JSONEq(t, `{"error":{"code":"NOT_IMPLEMENTED","message":"active assignments by supervisor is not implemented until a supervisor scope relation exists"}}`, recorder.Body.String())
	assert.Equal(t, uuid.Nil, repository.listedSupervisorID)
}

func TestAssignmentHandlerMapsApplicationErrorsToContractEnvelope(t *testing.T) {
	service := &assignmentHTTPServiceStub{createErr: asignacion.ErrAssignmentConflict}
	recorder := performAssignmentRequest(t, service, http.MethodPost, "/api/v1/asignacion", `{
		"empresa_id":"2fa85f64-5717-4562-b3fc-2c963f66afa5",
		"empleado_id":"3fa85f64-5717-4562-b3fc-2c963f66afa6",
		"localidad_id":"3fa85f64-5717-4562-b3fc-2c963f66afa7",
		"servicio_id":"3fa85f64-5717-4562-b3fc-2c963f66afa8",
		"turno_id":"3fa85f64-5717-4562-b3fc-2c963f66afa9",
		"fecha_inicio":"2026-06-09"
	}`, "ADMIN_CHECK_ON")

	assert.Equal(t, http.StatusConflict, recorder.Code)
	assert.JSONEq(t, `{"error":{"code":"ASSIGNMENT_CONFLICT","message":"assignment conflict"}}`, recorder.Body.String())
}

func performAssignmentRequest(t *testing.T, service asignacion.AssignmentService, method, target, body, role string) *httptest.ResponseRecorder {
	t.Helper()
	router := chi.NewRouter()
	asignacion.NewHTTPHandler(service).RegisterRoutes(router)

	request := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	if role != "" {
		ctx := context.WithValue(request.Context(), middleware.ContextKeyRole, role)
		request = request.WithContext(context.WithValue(ctx, middleware.ContextKeyUserID, uuid.New().String()))
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

type assignmentHTTPServiceStub struct {
	createCalled bool
	createInput  asignacion.CreateAssignmentInput
	created      *asignacion.Assignment
	createErr    error

	modifyID    uuid.UUID
	modifyInput asignacion.ModifyAssignmentInput
	modified    *asignacion.Assignment
	modifyErr   error

	listClientID uuid.UUID
	listFilter   asignacion.AssignmentListFilter
	supervisorID uuid.UUID
	pagination   asignacion.Pagination
	listResult   asignacion.AssignmentListResult
	listErr      error

	activitiesAssignmentID uuid.UUID
	activitiesStatus       *asignacion.ActivityStatus
	activities             []asignacion.AssignedActivity
	activitiesErr          error

	evidenceActivityID uuid.UUID
	evidenceInput      asignacion.UploadActivityEvidenceInput
	evidence           *asignacion.AssignedActivity
	evidenceErr        error

	batchUpdates []asignacion.ActivityStatusUpdate
	batchUpdated []asignacion.AssignedActivity
	batchErr     error

	toolsSupervisorID uuid.UUID
	toolsStatus       *asignacion.ToolDeliveryStatus
	tools             []asignacion.AssignedTool
	toolsErr          error

	submitEvaluationInput asignacion.SubmitEvaluationInput
	submitEvaluation      *asignacion.EmployeeEvaluation
	submitEvaluationErr   error

	historyEmployeeID uuid.UUID
	historyPagination asignacion.Pagination
	historyResult     asignacion.EvaluationHistoryResult
	historyErr        error
}

func (s *assignmentHTTPServiceStub) CreateAssignment(_ context.Context, input asignacion.CreateAssignmentInput) (*asignacion.Assignment, error) {
	s.createCalled = true
	s.createInput = input
	if s.createErr != nil {
		return nil, s.createErr
	}
	if s.created == nil {
		return nil, errors.New("missing created assignment fixture")
	}
	return s.created, nil
}

func (s *assignmentHTTPServiceStub) ModifyAssignment(_ context.Context, id uuid.UUID, input asignacion.ModifyAssignmentInput) (*asignacion.Assignment, error) {
	s.modifyID = id
	s.modifyInput = input
	if s.modifyErr != nil {
		return nil, s.modifyErr
	}
	if s.modified == nil {
		return nil, errors.New("missing modified assignment fixture")
	}
	return s.modified, nil
}

func (s *assignmentHTTPServiceStub) ListAssignmentsByClient(_ context.Context, clientID uuid.UUID, filter asignacion.AssignmentListFilter) (asignacion.AssignmentListResult, error) {
	s.listClientID = clientID
	s.listFilter = filter
	return s.listResult, s.listErr
}

func (s *assignmentHTTPServiceStub) ListActiveAssignmentsBySupervisor(_ context.Context, supervisorID uuid.UUID, pagination asignacion.Pagination) (asignacion.AssignmentListResult, error) {
	s.supervisorID = supervisorID
	s.pagination = pagination
	return s.listResult, s.listErr
}

func (s *assignmentHTTPServiceStub) ListActivitiesByAssignment(_ context.Context, assignmentID uuid.UUID, status *asignacion.ActivityStatus) ([]asignacion.AssignedActivity, error) {
	s.activitiesAssignmentID = assignmentID
	s.activitiesStatus = status
	return s.activities, s.activitiesErr
}

func (s *assignmentHTTPServiceStub) UploadActivityEvidence(_ context.Context, activityID uuid.UUID, input asignacion.UploadActivityEvidenceInput) (*asignacion.AssignedActivity, error) {
	s.evidenceActivityID = activityID
	s.evidenceInput = input
	if s.evidenceErr != nil {
		return nil, s.evidenceErr
	}
	if s.evidence == nil {
		return nil, errors.New("missing evidence activity fixture")
	}
	return s.evidence, nil
}

func (s *assignmentHTTPServiceStub) BatchUpdateActivityStatus(_ context.Context, updates []asignacion.ActivityStatusUpdate) ([]asignacion.AssignedActivity, error) {
	s.batchUpdates = updates
	if s.batchErr != nil {
		return nil, s.batchErr
	}
	return s.batchUpdated, nil
}

func (s *assignmentHTTPServiceStub) ListToolsBySupervisor(_ context.Context, supervisorID uuid.UUID, status *asignacion.ToolDeliveryStatus) ([]asignacion.AssignedTool, error) {
	s.toolsSupervisorID = supervisorID
	s.toolsStatus = status
	return s.tools, s.toolsErr
}

func (s *assignmentHTTPServiceStub) SubmitEvaluation(_ context.Context, input asignacion.SubmitEvaluationInput) (*asignacion.EmployeeEvaluation, error) {
	s.submitEvaluationInput = input
	if s.submitEvaluationErr != nil {
		return nil, s.submitEvaluationErr
	}
	if s.submitEvaluation == nil {
		return nil, errors.New("missing evaluation fixture")
	}
	return s.submitEvaluation, nil
}

func (s *assignmentHTTPServiceStub) GetEvaluationHistory(_ context.Context, employeeID uuid.UUID, pagination asignacion.Pagination) (asignacion.EvaluationHistoryResult, error) {
	s.historyEmployeeID = employeeID
	s.historyPagination = pagination
	return s.historyResult, s.historyErr
}

func mustDate(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(asignacion.DateLayout, value)
	require.NoError(t, err)
	return parsed
}

func datePtr(t *testing.T, value string) *time.Time {
	t.Helper()
	parsed := mustDate(t, value)
	return &parsed
}

func TestAssignmentHandlerListActivitiesContract(t *testing.T) {
	assignmentID := uuid.MustParse("c7a85f64-5717-4562-b3fc-2c963f66afb0")
	activityID := uuid.MustParse("d8a85f64-5717-4562-b3fc-2c963f66afc0")
	completedAt := time.Date(2026, 6, 8, 18, 42, 0, 0, time.UTC)
	lat, lon := 19.4326, -99.1332
	service := &assignmentHTTPServiceStub{
		activities: []asignacion.AssignedActivity{
			{
				ID:              activityID,
				AssignmentID:    assignmentID,
				ActivityID:      activityID,
				Description:     "Limpieza de pasillo",
				Status:          asignacion.ActivityStatusCompleted,
				Evidence1:       "https://s3.amazonaws.com/checkon-evidences/evidencia_1_d8a85f64-5717-4562-b3fc-2c963f66afc0.jpg",
				UploadLatitude:  &lat,
				UploadLongitude: &lon,
				ExecutionDate:   &completedAt,
			},
		},
	}
	recorder := performAssignmentRequest(t, service, http.MethodGet, "/api/v1/asignacion/"+assignmentID.String()+"/actividades?estatus=COMPLETED", "", "SUPERVISOR")

	require.Equal(t, http.StatusOK, recorder.Code)
	var body struct {
		Data []map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Len(t, body.Data, 1)
	assert.Equal(t, activityID.String(), body.Data[0]["actividad_id"])
	assert.Equal(t, "COMPLETED", body.Data[0]["estatus"])
	assert.Equal(t, assignmentID, service.activitiesAssignmentID)
	require.NotNil(t, service.activitiesStatus)
	assert.Equal(t, asignacion.ActivityStatusCompleted, *service.activitiesStatus)
}

func TestAssignmentHandlerListActivitiesRejectsInvalidStatusFilter(t *testing.T) {
	service := &assignmentHTTPServiceStub{}
	recorder := performAssignmentRequest(t, service, http.MethodGet, "/api/v1/asignacion/"+uuid.New().String()+"/actividades?estatus=BOGUS", "", "EMPLOYEE")

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.False(t, service.activitiesAssignmentID != uuid.Nil, "service must not be called on invalid filter")
}

func TestAssignmentHandlerUploadEvidenceContract(t *testing.T) {
	activityID := uuid.MustParse("d8a85f64-5717-4562-b3fc-2c963f66afc0")
	completedAt := time.Date(2026, 6, 8, 18, 42, 0, 0, time.UTC)
	lat, lon := 19.4326, -99.1332
	service := &assignmentHTTPServiceStub{
		evidence: &asignacion.AssignedActivity{
			ID:              activityID,
			AssignmentID:    uuid.New(),
			ActivityID:      activityID,
			Status:          asignacion.ActivityStatusCompleted,
			Evidence1:       "https://s3.amazonaws.com/checkon-evidences/evidencia_1_d8a85f64-5717-4562-b3fc-2c963f66afc0.jpg",
			Comments:        "Pasillo limpio",
			UploadLatitude:  &lat,
			UploadLongitude: &lon,
			ExecutionDate:   &completedAt,
		},
	}
	body, contentType := buildMultipartEvidence(t, "empleado_id", uuid.New().String(), "latitud", "19.4326", "longitud", "-99.1332", "comentario", "Pasillo limpio")
	body, contentType = addMultipartFile(body, contentType, "evidencia_1", "evidencia_1.jpg", "image/jpeg", []byte("jpgdata"))
	recorder := performMultipartAssignmentRequest(t, service, http.MethodPatch, "/api/v1/asignacion/actividad/"+activityID.String()+"/evidencia", body, contentType, "EMPLOYEE")

	require.Equal(t, http.StatusOK, recorder.Code)
	var parsed struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &parsed))
	assert.Equal(t, "COMPLETED", parsed.Data["estatus"])
	assert.Equal(t, activityID, service.evidenceActivityID)
	require.Len(t, service.evidenceInput.Files, 1)
	assert.Equal(t, 1, service.evidenceInput.Files[0].Slot)
	assert.Equal(t, "image/jpeg", service.evidenceInput.Files[0].ContentType)
	assert.InDelta(t, 19.4326, service.evidenceInput.Latitude, 1e-9)
	assert.Equal(t, "Pasillo limpio", service.evidenceInput.Comment)
}

func TestAssignmentHandlerUploadEvidenceRejectsNonEmployeeRole(t *testing.T) {
	service := &assignmentHTTPServiceStub{}
	body, contentType := buildMultipartEvidence(t, "empleado_id", uuid.New().String(), "latitud", "0", "longitud", "0")
	recorder := performMultipartAssignmentRequest(t, service, http.MethodPatch, "/api/v1/asignacion/actividad/"+uuid.New().String()+"/evidencia", body, contentType, "SUPERVISOR")

	assert.Equal(t, http.StatusForbidden, recorder.Code)
	assert.Nil(t, service.evidence, "service must not be called when role is not Employee")
}

func TestAssignmentHandlerUploadEvidenceMapsNotFoundError(t *testing.T) {
	service := &assignmentHTTPServiceStub{evidenceErr: asignacion.ErrAssignmentNotFound}
	body, contentType := buildMultipartEvidence(t, "empleado_id", uuid.New().String(), "latitud", "0", "longitud", "0")
	recorder := performMultipartAssignmentRequest(t, service, http.MethodPatch, "/api/v1/asignacion/actividad/"+uuid.New().String()+"/evidencia", body, contentType, "EMPLOYEE")

	assert.Equal(t, http.StatusNotFound, recorder.Code)
}

func TestAssignmentHandlerBatchUpdateActivitiesContract(t *testing.T) {
	activityID1 := uuid.New()
	activityID2 := uuid.New()
	updatedAt := time.Date(2026, 6, 8, 18, 44, 0, 0, time.UTC)
	service := &assignmentHTTPServiceStub{
		batchUpdated: []asignacion.AssignedActivity{
			{ID: activityID1, Status: asignacion.ActivityStatusCompleted, UpdatedAt: updatedAt},
			{ID: activityID2, Status: asignacion.ActivityStatusCancelled, UpdatedAt: updatedAt},
		},
	}
	payload := fmt.Sprintf(`{"actividades":[{"actividad_id":"%s","estatus":"COMPLETED"},{"actividad_id":"%s","estatus":"FAILED","comentario":"sin insumo"}]}`, activityID1, activityID2)
	recorder := performAssignmentRequest(t, service, http.MethodPatch, "/api/v1/asignacion/actividad/batch-status", payload, "SUPERVISOR")

	require.Equal(t, http.StatusOK, recorder.Code)
	var body struct {
		Data struct {
			UpdatedCount int              `json:"updated_count"`
			Actividades  []map[string]any `json:"actividades"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	assert.Equal(t, 2, body.Data.UpdatedCount)
	require.Len(t, body.Data.Actividades, 2)
	assert.Equal(t, activityID1.String(), body.Data.Actividades[0]["actividad_id"])
	assert.Equal(t, "COMPLETED", body.Data.Actividades[0]["estatus"])
	assert.Equal(t, "FAILED", body.Data.Actividades[1]["estatus"])
}

func TestAssignmentHandlerBatchUpdateActivitiesRejectsForbiddenRole(t *testing.T) {
	service := &assignmentHTTPServiceStub{}
	recorder := performAssignmentRequest(t, service, http.MethodPatch, "/api/v1/asignacion/actividad/batch-status", `{"actividades":[]}`, "EMPLOYEE")

	assert.Equal(t, http.StatusForbidden, recorder.Code)
	assert.Nil(t, service.batchUpdated)
}

func TestAssignmentHandlerBatchUpdateActivitiesRejectsInvalidStatus(t *testing.T) {
	service := &assignmentHTTPServiceStub{}
	payload := fmt.Sprintf(`{"actividades":[{"actividad_id":"%s","estatus":"BOGUS"}]}`, uuid.New())
	recorder := performAssignmentRequest(t, service, http.MethodPatch, "/api/v1/asignacion/actividad/batch-status", payload, "SUPERVISOR")

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Nil(t, service.batchUpdated)
}

func TestAssignmentHandlerListToolsBySupervisorContract(t *testing.T) {
	supervisorID := uuid.New()
	asignacionID := uuid.New()
	toolID := uuid.New()
	herramientaAsignadaID := uuid.New()
	service := &assignmentHTTPServiceStub{
		tools: []asignacion.AssignedTool{
			{ID: herramientaAsignadaID, AssignmentID: asignacionID, ToolID: toolID, Name: "Pulidora Industrial", Quantity: 1, DeliveryStatus: asignacion.ToolDeliveryStatusDelivered},
		},
	}
	recorder := performAssignmentRequest(t, service, http.MethodGet, "/api/v1/asignacion/supervisor/"+supervisorID.String()+"/herramientas?estatus=DELIVERED", "", "SUPERVISOR")

	require.Equal(t, http.StatusOK, recorder.Code)
	var body struct {
		Data []map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Len(t, body.Data, 1)
	assert.Equal(t, herramientaAsignadaID.String(), body.Data[0]["herramienta_asignada_id"])
	assert.Equal(t, "Pulidora Industrial", body.Data[0]["herramienta"].(map[string]any)["nombre"])
	assert.Equal(t, "DELIVERED", body.Data[0]["estatus"])
	assert.Equal(t, supervisorID, service.toolsSupervisorID)
	require.NotNil(t, service.toolsStatus)
	assert.Equal(t, asignacion.ToolDeliveryStatusDelivered, *service.toolsStatus)
}

func TestAssignmentHandlerListToolsBySupervisorRejectsForbiddenRole(t *testing.T) {
	service := &assignmentHTTPServiceStub{}
	recorder := performAssignmentRequest(t, service, http.MethodGet, "/api/v1/asignacion/supervisor/"+uuid.New().String()+"/herramientas", "", "EMPLOYEE")

	assert.Equal(t, http.StatusForbidden, recorder.Code)
	assert.Nil(t, service.tools)
}

func TestAssignmentHandlerSubmitEvaluationContract(t *testing.T) {
	evaluationID := uuid.New()
	createdAt := time.Date(2026, 6, 8, 18, 46, 0, 0, time.UTC)
	service := &assignmentHTTPServiceStub{
		submitEvaluation: &asignacion.EmployeeEvaluation{
			ID:                  evaluationID,
			AssignmentID:        uuid.New(),
			EmployeeID:          uuid.New(),
			EvaluatorID:         uuid.New(),
			Rating:              5,
			Comments:            "Excelente",
			EvaluationDate:      createdAt,
			CreatedAt:           createdAt,
			ActivitiesCompliant: true,
		},
	}
	payload := fmt.Sprintf(`{"asignacion_id":"%s","empleado_id":"%s","calificacion":4.5,"comentarios":"Excelente actitud","criterios":[{"nombre":"Puntualidad","puntos":5.0}]}`,
		uuid.New(), uuid.New())
	recorder := performAssignmentRequest(t, service, http.MethodPost, "/api/v1/asignacion/evaluacion", payload, "SUPERVISOR")

	require.Equal(t, http.StatusCreated, recorder.Code)
	var body struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	assert.Equal(t, evaluationID.String(), body.Data["evaluacion_id"])
	assert.Equal(t, float64(5), body.Data["calificacion"])
	assert.InDelta(t, 4.5, service.submitEvaluationInput.Rating, 1e-9)
	require.Len(t, service.submitEvaluationInput.Criteria, 1)
	assert.Equal(t, "Puntualidad", service.submitEvaluationInput.Criteria[0].Name)
}

func TestAssignmentHandlerSubmitEvaluationRejectsForbiddenRole(t *testing.T) {
	service := &assignmentHTTPServiceStub{}
	payload := fmt.Sprintf(`{"asignacion_id":"%s","empleado_id":"%s","calificacion":3.0}`, uuid.New(), uuid.New())
	recorder := performAssignmentRequest(t, service, http.MethodPost, "/api/v1/asignacion/evaluacion", payload, "EMPLOYEE")

	assert.Equal(t, http.StatusForbidden, recorder.Code)
	assert.Nil(t, service.submitEvaluation)
}

func TestAssignmentHandlerSubmitEvaluationRejectsOutOfRangeRating(t *testing.T) {
	service := &assignmentHTTPServiceStub{}
	payload := fmt.Sprintf(`{"asignacion_id":"%s","empleado_id":"%s","calificacion":7.0}`, uuid.New(), uuid.New())
	recorder := performAssignmentRequest(t, service, http.MethodPost, "/api/v1/asignacion/evaluacion", payload, "SUPERVISOR")

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Nil(t, service.submitEvaluation, "service must not be called on validation failure")
}

func TestAssignmentHandlerGetEvaluationHistoryContract(t *testing.T) {
	employeeID := uuid.New()
	createdAt := time.Date(2026, 6, 8, 18, 46, 0, 0, time.UTC)
	service := &assignmentHTTPServiceStub{
		historyResult: asignacion.EvaluationHistoryResult{
			Items: []asignacion.EmployeeEvaluation{
				{ID: uuid.New(), EmployeeID: employeeID, EvaluatorID: uuid.New(), Rating: 5, EvaluationDate: createdAt},
				{ID: uuid.New(), EmployeeID: employeeID, EvaluatorID: uuid.New(), Rating: 4, EvaluationDate: createdAt.Add(-24 * time.Hour)},
			},
			AverageRating:   4.5,
			EvaluationCount: 2,
			Pagination:      asignacion.PaginationResult{Page: 1, Limit: 10, TotalRecords: 2, TotalPages: 1},
		},
	}
	recorder := performAssignmentRequest(t, service, http.MethodGet, "/api/v1/asignacion/evaluacion/historial/"+employeeID.String()+"?page=1&pageSize=10", "", "SUPERVISOR")

	require.Equal(t, http.StatusOK, recorder.Code)
	var body struct {
		Data       map[string]any `json:"data"`
		Pagination map[string]any `json:"pagination"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	assert.Equal(t, employeeID.String(), body.Data["empleado_id"])
	assert.Equal(t, float64(2), body.Data["evaluaciones_count"])
	assert.InDelta(t, 4.5, body.Data["calificacion_promedio"], 1e-9)
	require.NotNil(t, body.Pagination)
	assert.Equal(t, float64(1), body.Pagination["page"])
	assert.Equal(t, employeeID, service.historyEmployeeID)
}

func TestAssignmentHandlerGetEvaluationHistoryRejectsInvalidPagination(t *testing.T) {
	service := &assignmentHTTPServiceStub{}
	recorder := performAssignmentRequest(t, service, http.MethodGet, "/api/v1/asignacion/evaluacion/historial/"+uuid.New().String()+"?page=0", "", "SUPERVISOR")

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}

func buildMultipartEvidence(t *testing.T, fields ...string) (*bytes.Buffer, string) {
	t.Helper()
	require.True(t, len(fields)%2 == 0, "buildMultipartEvidence expects key/value pairs")
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for i := 0; i < len(fields); i += 2 {
		require.NoError(t, writer.WriteField(fields[i], fields[i+1]))
	}
	require.NoError(t, writer.Close())
	return body, writer.FormDataContentType()
}

func addMultipartFile(body *bytes.Buffer, contentType, fieldName, filename, fileContentType string, data []byte) (*bytes.Buffer, string) {
	boundary := extractBoundary(contentType)
	reader := multipart.NewReader(body, boundary)
	combined := &bytes.Buffer{}
	writer := multipart.NewWriter(combined)

	copyField := func() error {
		part, err := reader.NextPart()
		if err != nil {
			return err
		}
		header := make(textproto.MIMEHeader)
		if part.FileName() != "" {
			header.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q; filename=%q`, part.FormName(), part.FileName()))
			if ct := part.Header.Get("Content-Type"); ct != "" {
				header.Set("Content-Type", ct)
			}
		} else {
			header.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q`, part.FormName()))
		}
		out, perr := writer.CreatePart(header)
		if perr != nil {
			return perr
		}
		if _, cerr := io.Copy(out, part); cerr != nil {
			return cerr
		}
		_ = part.Close()
		return nil
	}
	for {
		if err := copyField(); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			panic(err)
		}
	}

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q; filename=%q`, fieldName, filename))
	h.Set("Content-Type", fileContentType)
	filePart, ferr := writer.CreatePart(h)
	if ferr != nil {
		panic(ferr)
	}
	if _, werr := filePart.Write(data); werr != nil {
		panic(werr)
	}
	if cerr := writer.Close(); cerr != nil {
		panic(cerr)
	}
	return combined, writer.FormDataContentType()
}

func extractBoundary(contentType string) string {
	const marker = "boundary="
	idx := strings.Index(contentType, marker)
	if idx == -1 {
		return ""
	}
	boundary := contentType[idx+len(marker):]
	if strings.HasPrefix(boundary, `"`) && strings.HasSuffix(boundary, `"`) {
		boundary = boundary[1 : len(boundary)-1]
	}
	if semi := strings.Index(boundary, ";"); semi != -1 {
		boundary = boundary[:semi]
	}
	return strings.TrimSpace(boundary)
}

func performMultipartAssignmentRequest(t *testing.T, service asignacion.AssignmentService, method, target string, body *bytes.Buffer, contentType, role string) *httptest.ResponseRecorder {
	t.Helper()
	router := chi.NewRouter()
	asignacion.NewHTTPHandler(service).RegisterRoutes(router)

	request := httptest.NewRequest(method, target, body)
	request.Header.Set("Content-Type", contentType)
	if role != "" {
		request = request.WithContext(context.WithValue(request.Context(), middleware.ContextKeyRole, role))
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}
