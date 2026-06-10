package asignacion_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"axis-flow-back/internal/asignacion"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAssignmentDefaultsToActiveCurrentAssignment(t *testing.T) {
	start := time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC)
	assignment, err := asignacion.NewAssignment(asignacion.NewAssignmentParams{
		CompanyID:   uuid.New(),
		EmployeeID:  uuid.New(),
		LocationID:  uuid.New(),
		ServiceID:   uuid.New(),
		ShiftID:     uuid.New(),
		StartDate:   start,
		GeneratedAt: time.Date(2026, 6, 8, 18, 40, 0, 0, time.UTC),
	})

	require.NoError(t, err)
	assert.Equal(t, asignacion.AssignmentStatusActive, assignment.Status)
	assert.True(t, assignment.IsCurrent)
	assert.Equal(t, start, assignment.StartDate)
}

func TestNewAssignmentRejectsInvalidDateWindow(t *testing.T) {
	start := time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 0, -1)

	_, err := asignacion.NewAssignment(asignacion.NewAssignmentParams{
		CompanyID:  uuid.New(),
		EmployeeID: uuid.New(),
		LocationID: uuid.New(),
		ServiceID:  uuid.New(),
		ShiftID:    uuid.New(),
		StartDate:  start,
		EndDate:    &end,
	})

	require.ErrorIs(t, err, asignacion.ErrInvalidAssignmentDateWindow)
}

func TestActivityEvidenceURLsReturnsOnlyPersistedEvidence(t *testing.T) {
	activity := asignacion.AssignedActivity{
		Evidence1: "https://cdn.example.com/one.jpg",
		Evidence3: "https://cdn.example.com/three.jpg",
	}

	assert.Equal(t, []string{
		"https://cdn.example.com/one.jpg",
		"https://cdn.example.com/three.jpg",
	}, activity.EvidenceURLs())
}

func TestModifyAssignmentInputDoesNotExposeServiceID(t *testing.T) {
	inputType := reflect.TypeOf(asignacion.ModifyAssignmentInput{})
	_, exists := inputType.FieldByName("ServiceID")

	assert.False(t, exists, "ModifyAssignmentInput must stay aligned with OpenAPI ModifyAssignmentRequest, which does not include servicio_id")
}

func TestAssignmentRepositoryPortContract(t *testing.T) {
	var _ asignacion.AssignmentRepository = (*assignmentRepositoryStub)(nil)
	var _ asignacion.AssignedActivityRepository = (*activityRepositoryStub)(nil)
	var _ asignacion.AssignedToolRepository = (*toolRepositoryStub)(nil)
	var _ asignacion.EmployeeEvaluationRepository = (*evaluationRepositoryStub)(nil)
	var _ asignacion.AssignmentService = (*assignmentServiceStub)(nil)
	var _ asignacion.AssignmentEventPublisher = (*eventPublisherStub)(nil)
}

func TestAsignacionModificadaEventCarriesAssignmentMutationPayload(t *testing.T) {
	assignmentID := uuid.New()
	employeeID := uuid.New()
	locationID := uuid.New()
	updatedAt := time.Date(2026, 6, 8, 18, 45, 0, 0, time.UTC)

	event := asignacion.AsignacionModificadaEvent{
		AssignmentID: assignmentID,
		EmployeeID:   employeeID,
		LocationID:   locationID,
		Changes: asignacion.AssignmentChangeSummary{
			AddedActivityIDs:   []uuid.UUID{uuid.New(), uuid.New()},
			RemovedActivityIDs: []uuid.UUID{uuid.New()},
			AddedToolIDs:       []uuid.UUID{uuid.New()},
			RemovedToolIDs:     []uuid.UUID{},
		},
		UpdatedAt: updatedAt,
	}

	assert.Equal(t, assignmentID, event.AssignmentID)
	assert.Equal(t, employeeID, event.EmployeeID)
	assert.Equal(t, locationID, event.LocationID)
	require.Len(t, event.Changes.AddedActivityIDs, 2)
	require.Len(t, event.Changes.RemovedActivityIDs, 1)
	assert.True(t, event.UpdatedAt.Equal(updatedAt))
	assert.Equal(t, "AsignacionModificada", event.Name())
}

func TestEvidenciaCargadaEventCarriesEvidenceUploadPayload(t *testing.T) {
	activityID := uuid.New()
	assignmentID := uuid.New()
	employeeID := uuid.New()
	uploadedAt := time.Date(2026, 6, 8, 18, 42, 0, 0, time.UTC)

	event := asignacion.EvidenciaCargadaEvent{
		ActivityID:   activityID,
		AssignmentID: assignmentID,
		EmployeeID:   employeeID,
		EvidenceURLs: []string{
			"https://s3.amazonaws.com/checkon-evidences/evidencia_1_abc.jpg",
			"https://s3.amazonaws.com/checkon-evidences/evidencia_2_def.png",
		},
		GPSCoordinates: asignacion.GPSCoordinate{Latitude: 19.4326, Longitude: -99.1332},
		UploadedAt:     uploadedAt,
	}

	assert.Equal(t, activityID, event.ActivityID)
	assert.Equal(t, assignmentID, event.AssignmentID)
	assert.Equal(t, employeeID, event.EmployeeID)
	require.Len(t, event.EvidenceURLs, 2)
	assert.InDelta(t, 19.4326, event.GPSCoordinates.Latitude, 1e-9)
	assert.InDelta(t, -99.1332, event.GPSCoordinates.Longitude, 1e-9)
	assert.True(t, event.UploadedAt.Equal(uploadedAt))
	assert.Equal(t, "EvidenciaCargada", event.Name())
}

func TestActivityStatusFromStringAcceptsAliasesAndRejectsUnknown(t *testing.T) {
	cases := []struct {
		raw      string
		expected asignacion.ActivityStatus
		wantErr  error
	}{
		{"PENDING", asignacion.ActivityStatusPending, nil},
		{"pending", asignacion.ActivityStatusPending, nil},
		{"COMPLETED", asignacion.ActivityStatusCompleted, nil},
		{"FAILED", asignacion.ActivityStatusCancelled, nil},
		{"CANCELLED", asignacion.ActivityStatusCancelled, nil},
		{"cancelled", asignacion.ActivityStatusCancelled, nil},
		{"WAT", 0, asignacion.ErrInvalidActivityStatusFilter},
	}
	for _, tc := range cases {
		got, err := asignacion.ActivityStatusFromString(tc.raw)
		if tc.wantErr != nil {
			require.ErrorIs(t, err, tc.wantErr, "raw=%s", tc.raw)
			continue
		}
		require.NoError(t, err, "raw=%s", tc.raw)
		assert.Equal(t, tc.expected, got, "raw=%s", tc.raw)
	}
}

func TestActivityStatusStringRoundTrips(t *testing.T) {
	for _, status := range []asignacion.ActivityStatus{asignacion.ActivityStatusPending, asignacion.ActivityStatusCompleted, asignacion.ActivityStatusCancelled} {
		assert.Equal(t, status, mustParseActivityStatus(t, status.String()))
	}
}

func TestToolDeliveryStatusFromStringAcceptsAliasesAndRejectsUnknown(t *testing.T) {
	cases := []struct {
		raw      string
		expected asignacion.ToolDeliveryStatus
		wantErr  error
	}{
		{"ASSIGNED", asignacion.ToolDeliveryStatusPending, nil},
		{"delivered", asignacion.ToolDeliveryStatusDelivered, nil},
		{"RETURNED", asignacion.ToolDeliveryStatusReturned, nil},
		{"BOGUS", 0, asignacion.ErrInvalidToolDeliveryStatusFilter},
	}
	for _, tc := range cases {
		got, err := asignacion.ToolDeliveryStatusFromString(tc.raw)
		if tc.wantErr != nil {
			require.ErrorIs(t, err, tc.wantErr, "raw=%s", tc.raw)
			continue
		}
		require.NoError(t, err, "raw=%s", tc.raw)
		assert.Equal(t, tc.expected, got, "raw=%s", tc.raw)
	}
}

func TestValidateEvidenceFileEnforcesSpecRules(t *testing.T) {
	good := asignacion.EvidenceFile{Slot: 1, Filename: "evidencia_1.jpg", ContentType: "image/jpeg", SizeBytes: 1024}
	require.NoError(t, asignacion.ValidateEvidenceFile(good))

	badEmpty := good
	badEmpty.SizeBytes = 0
	require.ErrorIs(t, asignacion.ValidateEvidenceFile(badEmpty), asignacion.ErrInvalidEvidenceFile)

	badBig := good
	badBig.SizeBytes = asignacion.MaxEvidenceFileSize + 1
	require.ErrorIs(t, asignacion.ValidateEvidenceFile(badBig), asignacion.ErrInvalidEvidenceFile)

	badType := good
	badType.ContentType = "application/pdf"
	require.ErrorIs(t, asignacion.ValidateEvidenceFile(badType), asignacion.ErrInvalidEvidenceFile)
}

func TestBuildEvidenceStorageURLResolvesExpectedShape(t *testing.T) {
	activityID := uuid.MustParse("d8a85f64-5717-4562-b3fc-2c963f66afc0")
	assert.Equal(t,
		"https://s3.amazonaws.com/checkon-evidences/evidencia_1_d8a85f64-5717-4562-b3fc-2c963f66afc0.jpg",
		asignacion.BuildEvidenceStorageURL(activityID, 1, "evidencia_1.jpg"))
	assert.Equal(t,
		"https://s3.amazonaws.com/checkon-evidences/evidencia_2_d8a85f64-5717-4562-b3fc-2c963f66afc0.png",
		asignacion.BuildEvidenceStorageURL(activityID, 2, "photo.PNG"))
}

func TestValidateCoordinateRejectsOutOfRangeValues(t *testing.T) {
	require.NoError(t, asignacion.ValidateCoordinate(asignacion.GPSCoordinate{Latitude: 0, Longitude: 0}))
	require.NoError(t, asignacion.ValidateCoordinate(asignacion.GPSCoordinate{Latitude: -90, Longitude: 180}))
	require.Error(t, asignacion.ValidateCoordinate(asignacion.GPSCoordinate{Latitude: 91, Longitude: 0}))
	require.Error(t, asignacion.ValidateCoordinate(asignacion.GPSCoordinate{Latitude: 0, Longitude: 181}))
}

func mustParseActivityStatus(t *testing.T, raw string) asignacion.ActivityStatus {
	t.Helper()
	got, err := asignacion.ActivityStatusFromString(raw)
	require.NoError(t, err)
	return got
}

type assignmentRepositoryStub struct{}

func (assignmentRepositoryStub) Create(ctx context.Context, assignment *asignacion.Assignment) error {
	return nil
}
func (assignmentRepositoryStub) ReplaceCurrentForEmployee(ctx context.Context, assignment *asignacion.Assignment, deactivatedAt time.Time) error {
	return nil
}
func (assignmentRepositoryStub) FindByID(ctx context.Context, id uuid.UUID) (*asignacion.Assignment, error) {
	return nil, nil
}
func (assignmentRepositoryStub) FindCurrentByEmployee(ctx context.Context, employeeID uuid.UUID) (*asignacion.Assignment, error) {
	return nil, nil
}
func (assignmentRepositoryStub) ListByClient(ctx context.Context, companyID uuid.UUID, filter asignacion.AssignmentListFilter) ([]asignacion.Assignment, error) {
	return nil, nil
}
func (assignmentRepositoryStub) CountByClient(ctx context.Context, companyID uuid.UUID, filter asignacion.AssignmentListFilter) (int, error) {
	return 0, nil
}
func (assignmentRepositoryStub) ListActiveBySupervisor(ctx context.Context, supervisorID uuid.UUID, filter asignacion.Pagination) ([]asignacion.Assignment, error) {
	return nil, nil
}
func (assignmentRepositoryStub) CountActiveBySupervisor(ctx context.Context, supervisorID uuid.UUID) (int, error) {
	return 0, nil
}
func (assignmentRepositoryStub) DeactivateCurrentForEmployee(ctx context.Context, employeeID uuid.UUID, deactivatedAt time.Time) error {
	return nil
}
func (assignmentRepositoryStub) Update(ctx context.Context, assignment *asignacion.Assignment) error {
	return nil
}

type activityRepositoryStub struct{}

func (activityRepositoryStub) CreateBatch(ctx context.Context, activities []asignacion.AssignedActivity) error {
	return nil
}
func (activityRepositoryStub) ListByAssignment(ctx context.Context, assignmentID uuid.UUID, status *asignacion.ActivityStatus) ([]asignacion.AssignedActivity, error) {
	return nil, nil
}
func (activityRepositoryStub) UpdateEvidence(ctx context.Context, activityID uuid.UUID, evidence asignacion.ActivityEvidenceUpdate) (*asignacion.AssignedActivity, error) {
	return nil, nil
}
func (activityRepositoryStub) BatchUpdateStatus(ctx context.Context, updates []asignacion.ActivityStatusUpdate) ([]asignacion.AssignedActivity, error) {
	return nil, nil
}

type toolRepositoryStub struct{}

func (toolRepositoryStub) CreateBatch(ctx context.Context, tools []asignacion.AssignedTool) error {
	return nil
}
func (toolRepositoryStub) ListBySupervisor(ctx context.Context, supervisorID uuid.UUID, status *asignacion.ToolDeliveryStatus) ([]asignacion.AssignedTool, error) {
	return nil, nil
}

type evaluationRepositoryStub struct{}

func (evaluationRepositoryStub) Create(ctx context.Context, evaluation *asignacion.EmployeeEvaluation) error {
	return nil
}
func (evaluationRepositoryStub) ListByEmployee(ctx context.Context, employeeID uuid.UUID, filter asignacion.Pagination) ([]asignacion.EmployeeEvaluation, error) {
	return nil, nil
}

type assignmentServiceStub struct{}

func (assignmentServiceStub) CreateAssignment(ctx context.Context, input asignacion.CreateAssignmentInput) (*asignacion.Assignment, error) {
	return nil, nil
}
func (assignmentServiceStub) ModifyAssignment(ctx context.Context, id uuid.UUID, input asignacion.ModifyAssignmentInput) (*asignacion.Assignment, error) {
	return nil, nil
}
func (assignmentServiceStub) ListAssignmentsByClient(ctx context.Context, clientID uuid.UUID, filter asignacion.AssignmentListFilter) (asignacion.AssignmentListResult, error) {
	return asignacion.AssignmentListResult{}, nil
}
func (assignmentServiceStub) ListActiveAssignmentsBySupervisor(ctx context.Context, supervisorID uuid.UUID, pagination asignacion.Pagination) (asignacion.AssignmentListResult, error) {
	return asignacion.AssignmentListResult{}, nil
}
func (assignmentServiceStub) ListActivitiesByAssignment(ctx context.Context, assignmentID uuid.UUID, status *asignacion.ActivityStatus) ([]asignacion.AssignedActivity, error) {
	return nil, nil
}
func (assignmentServiceStub) UploadActivityEvidence(ctx context.Context, activityID uuid.UUID, input asignacion.UploadActivityEvidenceInput) (*asignacion.AssignedActivity, error) {
	return nil, nil
}
func (assignmentServiceStub) BatchUpdateActivityStatus(ctx context.Context, updates []asignacion.ActivityStatusUpdate) ([]asignacion.AssignedActivity, error) {
	return nil, nil
}
func (assignmentServiceStub) ListToolsBySupervisor(ctx context.Context, supervisorID uuid.UUID, status *asignacion.ToolDeliveryStatus) ([]asignacion.AssignedTool, error) {
	return nil, nil
}
func (assignmentServiceStub) SubmitEvaluation(ctx context.Context, input asignacion.SubmitEvaluationInput) (*asignacion.EmployeeEvaluation, error) {
	return nil, nil
}
func (assignmentServiceStub) GetEvaluationHistory(ctx context.Context, employeeID uuid.UUID, pagination asignacion.Pagination) (asignacion.EvaluationHistoryResult, error) {
	return asignacion.EvaluationHistoryResult{}, nil
}

type eventPublisherStub struct{}

func (eventPublisherStub) PublishAsignacionModificada(_ context.Context, _ asignacion.AsignacionModificadaEvent) error {
	return nil
}
func (eventPublisherStub) PublishEvidenciaCargada(_ context.Context, _ asignacion.EvidenciaCargadaEvent) error {
	return nil
}
