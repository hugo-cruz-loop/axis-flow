package asignacion_test

import (
	"context"
	"testing"
	"time"

	"axis-flow-back/internal/asignacion"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAssignmentApplicationServiceCreateReplacesPreviousAtomicallyAndInvalidatesCache(t *testing.T) {
	repository := &assignmentRepositorySpy{}
	cache := &assignmentCacheSpy{}
	clock := fixedClock(time.Date(2026, 6, 8, 18, 40, 0, 0, time.UTC))
	service := asignacion.NewAssignmentApplicationService(repository, cache, clock)
	employeeID := uuid.New()

	assignment, err := service.CreateAssignment(context.Background(), asignacion.CreateAssignmentInput{
		CompanyID:  uuid.New(),
		EmployeeID: employeeID,
		LocationID: uuid.New(),
		ServiceID:  uuid.New(),
		ShiftID:    uuid.New(),
		StartDate:  mustDate(t, "2026-06-09"),
	})

	require.NoError(t, err)
	require.NotNil(t, assignment)
	assert.True(t, repository.replaceCalled)
	assert.Equal(t, employeeID, repository.replacedEmployeeID)
	assert.True(t, repository.deactivatedAt.Equal(clock()))
	assert.Equal(t, assignment.ID, repository.created.ID)
	assert.True(t, assignment.IsCurrent)
	assert.Equal(t, []uuid.UUID{employeeID}, cache.invalidatedEmployees)
}

func TestAssignmentApplicationServiceCreateUsesAtomicRepositoryReplacement(t *testing.T) {
	repository := &assignmentRepositorySpy{}
	service := asignacion.NewAssignmentApplicationService(repository, nil, fixedClock(time.Date(2026, 6, 8, 18, 40, 0, 0, time.UTC)))
	employeeID := uuid.New()

	assignment, err := service.CreateAssignment(context.Background(), asignacion.CreateAssignmentInput{
		CompanyID:  uuid.New(),
		EmployeeID: employeeID,
		LocationID: uuid.New(),
		ServiceID:  uuid.New(),
		ShiftID:    uuid.New(),
		StartDate:  mustDate(t, "2026-06-09"),
	})

	require.NoError(t, err)
	require.NotNil(t, assignment)
	assert.True(t, repository.replaceCalled)
	assert.Equal(t, employeeID, repository.replacedEmployeeID)
	assert.Equal(t, assignment.ID, repository.created.ID)
	assert.False(t, repository.deactivateCalled, "service must not fake atomicity with a separate deactivate call")
	assert.False(t, repository.createCalled, "service must not fake atomicity with a separate create call")
}

func TestAssignmentApplicationServiceCreateRejectsInvalidDateWindowBeforeWriting(t *testing.T) {
	repository := &assignmentRepositorySpy{}
	cache := &assignmentCacheSpy{}
	service := asignacion.NewAssignmentApplicationService(repository, cache, fixedClock(time.Now().UTC()))
	start := mustDate(t, "2026-06-09")
	end := mustDate(t, "2026-06-08")

	_, err := service.CreateAssignment(context.Background(), asignacion.CreateAssignmentInput{
		CompanyID:  uuid.New(),
		EmployeeID: uuid.New(),
		LocationID: uuid.New(),
		ServiceID:  uuid.New(),
		ShiftID:    uuid.New(),
		StartDate:  start,
		EndDate:    &end,
	})

	require.ErrorIs(t, err, asignacion.ErrInvalidAssignmentDateWindow)
	assert.False(t, repository.deactivateCalled)
	assert.False(t, repository.createCalled)
	assert.Empty(t, cache.invalidatedEmployees)
}

func TestAssignmentApplicationServiceModifyUpdatesExistingAndInvalidatesCache(t *testing.T) {
	assignmentID := uuid.New()
	employeeID := uuid.New()
	locationID := uuid.New()
	shiftID := uuid.New()
	inactive := asignacion.AssignmentStatusInactive
	repository := &assignmentRepositorySpy{
		found: &asignacion.Assignment{
			ID:         assignmentID,
			CompanyID:  uuid.New(),
			EmployeeID: employeeID,
			LocationID: uuid.New(),
			ServiceID:  uuid.New(),
			ShiftID:    uuid.New(),
			StartDate:  mustDate(t, "2026-06-09"),
			Status:     asignacion.AssignmentStatusActive,
			IsCurrent:  true,
		},
	}
	cache := &assignmentCacheSpy{}
	publisher := &eventPublisherSpy{}
	clock := fixedClock(time.Date(2026, 6, 8, 18, 45, 0, 0, time.UTC))
	service := asignacion.NewAssignmentApplicationServiceWithDependencies(
		repository, cache, &activityRepositorySpy{}, &toolRepositorySpy{}, &evaluationRepositorySpy{},
		nil, publisher, clock,
	)
	end := mustDate(t, "2026-07-15")

	updated, err := service.ModifyAssignment(context.Background(), assignmentID, asignacion.ModifyAssignmentInput{
		LocationID: &locationID,
		ShiftID:    &shiftID,
		EndDate:    &end,
		Status:     &inactive,
	})

	require.NoError(t, err)
	assert.Equal(t, locationID, updated.LocationID)
	assert.Equal(t, shiftID, updated.ShiftID)
	assert.Equal(t, inactive, updated.Status)
	assert.True(t, updated.UpdatedAt.Equal(clock()))
	assert.Equal(t, updated.ID, repository.updated.ID)
	assert.Equal(t, []uuid.UUID{employeeID}, cache.invalidatedEmployees)
	require.Len(t, publisher.asignacionModificadaEvents, 1, "AsignacionModificada must be published after successful modify")
	assert.Equal(t, assignmentID, publisher.asignacionModificadaEvents[0].AssignmentID)
	assert.Equal(t, employeeID, publisher.asignacionModificadaEvents[0].EmployeeID)
	assert.Equal(t, locationID, publisher.asignacionModificadaEvents[0].LocationID)
	assert.True(t, publisher.asignacionModificadaEvents[0].UpdatedAt.Equal(clock()))
}

func TestAssignmentApplicationServiceModifyDoesNotPublishWhenAssignmentDoesNotExist(t *testing.T) {
	assignmentID := uuid.New()
	repository := &assignmentRepositorySpy{}
	publisher := &eventPublisherSpy{}
	service := asignacion.NewAssignmentApplicationServiceWithDependencies(
		repository, &assignmentCacheSpy{}, &activityRepositorySpy{}, &toolRepositorySpy{}, &evaluationRepositorySpy{},
		nil, publisher, fixedClock(time.Now().UTC()),
	)

	_, err := service.ModifyAssignment(context.Background(), assignmentID, asignacion.ModifyAssignmentInput{
		LocationID: ptrUUID(uuid.New()),
	})

	require.ErrorIs(t, err, asignacion.ErrAssignmentNotFound)
	assert.Empty(t, publisher.asignacionModificadaEvents, "publisher must not be called when modify fails")
}

func ptrUUID(id uuid.UUID) *uuid.UUID { return &id }

func TestAssignmentApplicationServiceActiveBySupervisorReturnsNotImplementedWithoutScopeRelation(t *testing.T) {
	repository := &assignmentRepositorySpy{}
	service := asignacion.NewAssignmentApplicationService(repository, nil, fixedClock(time.Now().UTC()))
	supervisorID := uuid.New()

	_, err := service.ListActiveAssignmentsBySupervisor(context.Background(), supervisorID, asignacion.Pagination{Limit: 10})

	require.ErrorIs(t, err, asignacion.ErrAssignmentSupervisorScopeNotImplemented)
	assert.Equal(t, uuid.Nil, repository.listedSupervisorID)
}

func TestAssignmentApplicationServiceListCalculatesPagination(t *testing.T) {
	clientID := uuid.New()
	repository := &assignmentRepositorySpy{
		listedByClient: []asignacion.Assignment{{ID: uuid.New()}},
		clientCount:    11,
	}
	service := asignacion.NewAssignmentApplicationService(repository, &assignmentCacheSpy{}, fixedClock(time.Now().UTC()))

	result, err := service.ListAssignmentsByClient(context.Background(), clientID, asignacion.AssignmentListFilter{
		Pagination: asignacion.Pagination{Limit: 5, Offset: 10},
	})

	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	assert.Equal(t, asignacion.PaginationResult{Page: 3, Limit: 5, TotalRecords: 11, TotalPages: 3}, result.Pagination)
	assert.Equal(t, clientID, repository.listedClientID)
}

type assignmentRepositorySpy struct {
	deactivateCalled      bool
	replaceCalled         bool
	replacedEmployeeID    uuid.UUID
	deactivatedEmployeeID uuid.UUID
	deactivatedAt         time.Time
	createCalled          bool
	created               asignacion.Assignment
	found                 *asignacion.Assignment
	updated               asignacion.Assignment
	listedClientID        uuid.UUID
	listedByClient        []asignacion.Assignment
	clientCount           int
	listedSupervisorID    uuid.UUID
	listedBySupervisor    []asignacion.Assignment
	supervisorCount       int
}

func (r *assignmentRepositorySpy) ReplaceCurrentForEmployee(_ context.Context, assignment *asignacion.Assignment, deactivatedAt time.Time) error {
	r.replaceCalled = true
	r.replacedEmployeeID = assignment.EmployeeID
	r.deactivatedAt = deactivatedAt
	r.created = *assignment
	return nil
}
func (r *assignmentRepositorySpy) Create(_ context.Context, assignment *asignacion.Assignment) error {
	r.createCalled = true
	r.created = *assignment
	return nil
}
func (r *assignmentRepositorySpy) FindByID(_ context.Context, id uuid.UUID) (*asignacion.Assignment, error) {
	if r.found == nil {
		return nil, asignacion.ErrAssignmentNotFound
	}
	copy := *r.found
	return &copy, nil
}
func (r *assignmentRepositorySpy) FindCurrentByEmployee(_ context.Context, employeeID uuid.UUID) (*asignacion.Assignment, error) {
	return nil, asignacion.ErrAssignmentNotFound
}
func (r *assignmentRepositorySpy) ListByClient(_ context.Context, clientID uuid.UUID, filter asignacion.AssignmentListFilter) ([]asignacion.Assignment, error) {
	r.listedClientID = clientID
	return r.listedByClient, nil
}
func (r *assignmentRepositorySpy) CountByClient(_ context.Context, clientID uuid.UUID, filter asignacion.AssignmentListFilter) (int, error) {
	r.listedClientID = clientID
	return r.clientCount, nil
}
func (r *assignmentRepositorySpy) ListActiveBySupervisor(_ context.Context, supervisorID uuid.UUID, filter asignacion.Pagination) ([]asignacion.Assignment, error) {
	r.listedSupervisorID = supervisorID
	return r.listedBySupervisor, nil
}
func (r *assignmentRepositorySpy) CountActiveBySupervisor(_ context.Context, supervisorID uuid.UUID) (int, error) {
	r.listedSupervisorID = supervisorID
	return r.supervisorCount, nil
}
func (r *assignmentRepositorySpy) DeactivateCurrentForEmployee(_ context.Context, employeeID uuid.UUID, deactivatedAt time.Time) error {
	r.deactivateCalled = true
	r.deactivatedEmployeeID = employeeID
	r.deactivatedAt = deactivatedAt
	return nil
}
func (r *assignmentRepositorySpy) Update(_ context.Context, assignment *asignacion.Assignment) error {
	r.updated = *assignment
	return nil
}

type assignmentCacheSpy struct {
	invalidatedEmployees []uuid.UUID
}

func (c *assignmentCacheSpy) InvalidateEmployeeAssignments(_ context.Context, employeeID uuid.UUID) error {
	c.invalidatedEmployees = append(c.invalidatedEmployees, employeeID)
	return nil
}

type activityRepositorySpy struct {
	listedAssignmentID uuid.UUID
	listedStatus       *asignacion.ActivityStatus
	listed             []asignacion.AssignedActivity
	listErr            error

	evidenceActivityID uuid.UUID
	evidenceUpdate     asignacion.ActivityEvidenceUpdate
	updatedEvidence    *asignacion.AssignedActivity
	evidenceErr        error

	batchUpdates []asignacion.ActivityStatusUpdate
	batchResult  []asignacion.AssignedActivity
	batchErr     error
}

func (r *activityRepositorySpy) CreateBatch(_ context.Context, _ []asignacion.AssignedActivity) error {
	return nil
}
func (r *activityRepositorySpy) ListByAssignment(_ context.Context, assignmentID uuid.UUID, status *asignacion.ActivityStatus) ([]asignacion.AssignedActivity, error) {
	r.listedAssignmentID = assignmentID
	r.listedStatus = status
	if r.listErr != nil {
		return nil, r.listErr
	}
	return r.listed, nil
}
func (r *activityRepositorySpy) UpdateEvidence(_ context.Context, activityID uuid.UUID, evidence asignacion.ActivityEvidenceUpdate) (*asignacion.AssignedActivity, error) {
	r.evidenceActivityID = activityID
	r.evidenceUpdate = evidence
	if r.evidenceErr != nil {
		return nil, r.evidenceErr
	}
	if r.updatedEvidence == nil {
		return &asignacion.AssignedActivity{ID: activityID, AssignmentID: evidenceActivityAssignmentID(activityID), Status: asignacion.ActivityStatusCompleted, ExecutionDate: &evidence.CompletedAt}, nil
	}
	return r.updatedEvidence, nil
}
func (r *activityRepositorySpy) BatchUpdateStatus(_ context.Context, updates []asignacion.ActivityStatusUpdate) ([]asignacion.AssignedActivity, error) {
	r.batchUpdates = updates
	if r.batchErr != nil {
		return nil, r.batchErr
	}
	if r.batchResult != nil {
		return r.batchResult, nil
	}
	out := make([]asignacion.AssignedActivity, len(updates))
	for i, u := range updates {
		out[i] = asignacion.AssignedActivity{ID: u.ActivityID, Status: u.Status, UpdatedAt: u.UpdatedAt}
	}
	return out, nil
}

type toolRepositorySpy struct {
	listedSupervisorID uuid.UUID
	listedStatus       *asignacion.ToolDeliveryStatus
	listed             []asignacion.AssignedTool
	listErr            error
}

func (r *toolRepositorySpy) CreateBatch(_ context.Context, _ []asignacion.AssignedTool) error {
	return nil
}
func (r *toolRepositorySpy) ListBySupervisor(_ context.Context, supervisorID uuid.UUID, status *asignacion.ToolDeliveryStatus) ([]asignacion.AssignedTool, error) {
	r.listedSupervisorID = supervisorID
	r.listedStatus = status
	if r.listErr != nil {
		return nil, r.listErr
	}
	return r.listed, nil
}

type evaluationRepositorySpy struct {
	created    *asignacion.EmployeeEvaluation
	createErr  error
	listed     []asignacion.EmployeeEvaluation
	listedErr  error
	listFilter asignacion.Pagination
	listByID   uuid.UUID
}

func (r *evaluationRepositorySpy) Create(_ context.Context, evaluation *asignacion.EmployeeEvaluation) error {
	r.created = evaluation
	return r.createErr
}
func (r *evaluationRepositorySpy) ListByEmployee(_ context.Context, employeeID uuid.UUID, filter asignacion.Pagination) ([]asignacion.EmployeeEvaluation, error) {
	r.listByID = employeeID
	r.listFilter = filter
	if r.listedErr != nil {
		return nil, r.listedErr
	}
	return r.listed, nil
}

type eventPublisherSpy struct {
	asignacionModificadaEvents []asignacion.AsignacionModificadaEvent
	evidenciaCargadaEvents     []asignacion.EvidenciaCargadaEvent
	modErr                     error
	evErr                      error
}

func (p *eventPublisherSpy) PublishAsignacionModificada(_ context.Context, event asignacion.AsignacionModificadaEvent) error {
	p.asignacionModificadaEvents = append(p.asignacionModificadaEvents, event)
	return p.modErr
}

func (p *eventPublisherSpy) PublishEvidenciaCargada(_ context.Context, event asignacion.EvidenciaCargadaEvent) error {
	p.evidenciaCargadaEvents = append(p.evidenciaCargadaEvents, event)
	return p.evErr
}

func evidenceActivityAssignmentID(_ uuid.UUID) uuid.UUID {
	return uuid.MustParse("aaaaaaaa-aaaa-4aaa-aaaa-aaaaaaaaaaaa")
}

func fixedClock(now time.Time) func() time.Time {
	return func() time.Time { return now }
}

func TestAssignmentApplicationServiceUploadActivityEvidenceMarksCompletedAndPublishesEvent(t *testing.T) {
	activityID := uuid.MustParse("d8a85f64-5717-4562-b3fc-2c963f66afc0")
	employeeID := uuid.New()
	assignmentID := uuid.New()
	clock := time.Date(2026, 6, 8, 18, 42, 0, 0, time.UTC)

	activityRepo := &activityRepositorySpy{
		updatedEvidence: &asignacion.AssignedActivity{
			ID:           activityID,
			AssignmentID: assignmentID,
			Status:       asignacion.ActivityStatusCompleted,
		},
	}
	cache := &assignmentCacheSpy{}
	publisher := &eventPublisherSpy{}
	service := asignacion.NewAssignmentApplicationServiceWithDependencies(
		&assignmentRepositorySpy{},
		cache,
		activityRepo,
		&toolRepositorySpy{},
		&evaluationRepositorySpy{},
		nil,
		publisher,
		fixedClock(clock),
	)

	activity, err := service.UploadActivityEvidence(context.Background(), activityID, asignacion.UploadActivityEvidenceInput{
		EmployeeID: employeeID,
		Latitude:   19.4326,
		Longitude:  -99.1332,
		Comment:    "Pasillo limpio",
		Files: []asignacion.EvidenceFile{
			{Slot: 1, Filename: "evidencia_1.jpg", ContentType: "image/jpeg", SizeBytes: 1024},
			{Slot: 2, Filename: "evidencia_2.png", ContentType: "image/png", SizeBytes: 2048},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, activity)
	assert.Equal(t, asignacion.ActivityStatusCompleted, activity.Status)
	require.Len(t, activityRepo.evidenceUpdate.EvidenceURLs, 2)
	assert.Equal(t, "https://s3.amazonaws.com/checkon-evidences/evidencia_1_d8a85f64-5717-4562-b3fc-2c963f66afc0.jpg", activityRepo.evidenceUpdate.EvidenceURLs[0])
	assert.Equal(t, "https://s3.amazonaws.com/checkon-evidences/evidencia_2_d8a85f64-5717-4562-b3fc-2c963f66afc0.png", activityRepo.evidenceUpdate.EvidenceURLs[1])
	assert.InDelta(t, 19.4326, activityRepo.evidenceUpdate.Latitude, 1e-9)
	assert.Equal(t, "Pasillo limpio", activityRepo.evidenceUpdate.Comment)
	assert.True(t, activityRepo.evidenceUpdate.CompletedAt.Equal(clock))

	require.Len(t, publisher.evidenciaCargadaEvents, 1)
	got := publisher.evidenciaCargadaEvents[0]
	assert.Equal(t, activityID, got.ActivityID)
	assert.Equal(t, assignmentID, got.AssignmentID)
	assert.Equal(t, employeeID, got.EmployeeID)
	assert.Equal(t, activityRepo.evidenceUpdate.EvidenceURLs, got.EvidenceURLs)
	assert.InDelta(t, 19.4326, got.GPSCoordinates.Latitude, 1e-9)
	assert.True(t, got.UploadedAt.Equal(clock))

	assert.Equal(t, []uuid.UUID{employeeID}, cache.invalidatedEmployees)
}

func TestAssignmentApplicationServiceUploadActivityEvidenceRejectsEmptyInput(t *testing.T) {
	activityID := uuid.New()
	employeeID := uuid.New()
	activityRepo := &activityRepositorySpy{}
	cache := &assignmentCacheSpy{}
	publisher := &eventPublisherSpy{}
	service := asignacion.NewAssignmentApplicationServiceWithDependencies(
		&assignmentRepositorySpy{}, cache, activityRepo,
		&toolRepositorySpy{}, &evaluationRepositorySpy{}, nil, publisher,
		fixedClock(time.Date(2026, 6, 8, 18, 42, 0, 0, time.UTC)),
	)

	_, err := service.UploadActivityEvidence(context.Background(), activityID, asignacion.UploadActivityEvidenceInput{
		EmployeeID: employeeID,
		Latitude:   19.4326,
		Longitude:  -99.1332,
		Files:      nil,
	})

	require.ErrorIs(t, err, asignacion.ErrInvalidEvidenceFile)
	assert.Nil(t, activityRepo.evidenceUpdate.EvidenceURLs, "repo must not be touched on validation failure")
	assert.Empty(t, publisher.evidenciaCargadaEvents, "publisher must not be called on validation failure")
	assert.Empty(t, cache.invalidatedEmployees, "cache must not be invalidated on validation failure")
}

func TestAssignmentApplicationServiceListActivitiesByAssignmentForwardsStatusFilter(t *testing.T) {
	assignmentID := uuid.New()
	pending := asignacion.ActivityStatusPending
	listed := []asignacion.AssignedActivity{
		{ID: uuid.New(), AssignmentID: assignmentID, Status: asignacion.ActivityStatusPending},
		{ID: uuid.New(), AssignmentID: assignmentID, Status: asignacion.ActivityStatusCompleted},
	}
	activityRepo := &activityRepositorySpy{listed: listed}
	service := asignacion.NewAssignmentApplicationServiceWithDependencies(
		&assignmentRepositorySpy{}, &assignmentCacheSpy{}, activityRepo,
		&toolRepositorySpy{}, &evaluationRepositorySpy{}, nil, &eventPublisherSpy{},
		fixedClock(time.Now().UTC()),
	)

	activities, err := service.ListActivitiesByAssignment(context.Background(), assignmentID, &pending)

	require.NoError(t, err)
	require.Len(t, activities, 2)
	assert.Equal(t, assignmentID, activityRepo.listedAssignmentID)
	require.NotNil(t, activityRepo.listedStatus)
	assert.Equal(t, pending, *activityRepo.listedStatus)
}

func TestAssignmentApplicationServiceListActivitiesByAssignmentReturnsEmptyListForUnknownAssignment(t *testing.T) {
	assignmentID := uuid.New()
	activityRepo := &activityRepositorySpy{listed: nil}
	service := asignacion.NewAssignmentApplicationServiceWithDependencies(
		&assignmentRepositorySpy{}, &assignmentCacheSpy{}, activityRepo,
		&toolRepositorySpy{}, &evaluationRepositorySpy{}, nil, &eventPublisherSpy{},
		fixedClock(time.Now().UTC()),
	)

	activities, err := service.ListActivitiesByAssignment(context.Background(), assignmentID, nil)

	require.NoError(t, err)
	assert.Empty(t, activities)
	assert.Equal(t, assignmentID, activityRepo.listedAssignmentID)
	assert.Nil(t, activityRepo.listedStatus)
}

func TestAssignmentApplicationServiceBatchUpdateActivityStatusStampsUpdatedAtAndDoesNotPublish(t *testing.T) {
	activityID1 := uuid.New()
	activityID2 := uuid.New()
	clock := time.Date(2026, 6, 8, 18, 44, 0, 0, time.UTC)
	activityRepo := &activityRepositorySpy{}
	publisher := &eventPublisherSpy{}
	service := asignacion.NewAssignmentApplicationServiceWithDependencies(
		&assignmentRepositorySpy{}, &assignmentCacheSpy{}, activityRepo,
		&toolRepositorySpy{}, &evaluationRepositorySpy{}, nil, publisher,
		fixedClock(clock),
	)

	updated, err := service.BatchUpdateActivityStatus(context.Background(), []asignacion.ActivityStatusUpdate{
		{ActivityID: activityID1, Status: asignacion.ActivityStatusCompleted},
		{ActivityID: activityID2, Status: asignacion.ActivityStatusCancelled, Comment: "no se pudo"},
	})

	require.NoError(t, err)
	require.Len(t, updated, 2)
	assert.Equal(t, activityID1, updated[0].ID)
	assert.Equal(t, asignacion.ActivityStatusCompleted, updated[0].Status)
	assert.Equal(t, activityID2, updated[1].ID)
	assert.Equal(t, asignacion.ActivityStatusCancelled, updated[1].Status)
	require.Len(t, activityRepo.batchUpdates, 2)
	assert.True(t, activityRepo.batchUpdates[0].UpdatedAt.Equal(clock))
	assert.True(t, activityRepo.batchUpdates[1].UpdatedAt.Equal(clock))
	assert.Equal(t, "no se pudo", activityRepo.batchUpdates[1].Comment)
	assert.Empty(t, publisher.asignacionModificadaEvents, "batch status is not an assignment modification event")
	assert.Empty(t, publisher.evidenciaCargadaEvents)
}

func TestAssignmentApplicationServiceBatchUpdateActivityStatusRejectsEmptyBatch(t *testing.T) {
	activityRepo := &activityRepositorySpy{}
	publisher := &eventPublisherSpy{}
	service := asignacion.NewAssignmentApplicationServiceWithDependencies(
		&assignmentRepositorySpy{}, &assignmentCacheSpy{}, activityRepo,
		&toolRepositorySpy{}, &evaluationRepositorySpy{}, nil, publisher,
		fixedClock(time.Now().UTC()),
	)

	_, err := service.BatchUpdateActivityStatus(context.Background(), nil)

	require.ErrorIs(t, err, asignacion.ErrBatchUpdateEmpty)
	assert.Empty(t, activityRepo.batchUpdates, "repo must not be touched when batch is empty")
	assert.Empty(t, publisher.asignacionModificadaEvents)
	assert.Empty(t, publisher.evidenciaCargadaEvents)
}

func TestAssignmentApplicationServiceListToolsBySupervisorForwardsStatusFilter(t *testing.T) {
	supervisorID := uuid.New()
	delivered := asignacion.ToolDeliveryStatusDelivered
	listed := []asignacion.AssignedTool{
		{ID: uuid.New(), AssignmentID: uuid.New(), ToolID: uuid.New(), Name: "Pulidora", DeliveryStatus: delivered},
	}
	toolRepo := &toolRepositorySpy{listed: listed}
	service := asignacion.NewAssignmentApplicationServiceWithDependencies(
		&assignmentRepositorySpy{}, &assignmentCacheSpy{}, &activityRepositorySpy{},
		toolRepo, &evaluationRepositorySpy{}, nil, &eventPublisherSpy{},
		fixedClock(time.Now().UTC()),
	)

	tools, err := service.ListToolsBySupervisor(context.Background(), supervisorID, &delivered)

	require.NoError(t, err)
	require.Len(t, tools, 1)
	assert.Equal(t, "Pulidora", tools[0].Name)
	assert.Equal(t, supervisorID, toolRepo.listedSupervisorID)
	require.NotNil(t, toolRepo.listedStatus)
	assert.Equal(t, delivered, *toolRepo.listedStatus)
}

func TestAssignmentApplicationServiceListToolsBySupervisorReturnsAllWithoutFilter(t *testing.T) {
	supervisorID := uuid.New()
	toolRepo := &toolRepositorySpy{listed: []asignacion.AssignedTool{{ID: uuid.New(), Name: "A"}, {ID: uuid.New(), Name: "B"}}}
	service := asignacion.NewAssignmentApplicationServiceWithDependencies(
		&assignmentRepositorySpy{}, &assignmentCacheSpy{}, &activityRepositorySpy{},
		toolRepo, &evaluationRepositorySpy{}, nil, &eventPublisherSpy{},
		fixedClock(time.Now().UTC()),
	)

	tools, err := service.ListToolsBySupervisor(context.Background(), supervisorID, nil)

	require.NoError(t, err)
	assert.Len(t, tools, 2)
	assert.Equal(t, supervisorID, toolRepo.listedSupervisorID)
	assert.Nil(t, toolRepo.listedStatus, "no status filter means nil is forwarded to the repo")
}

func TestAssignmentApplicationServiceSubmitEvaluationPersistsRatingWithinRange(t *testing.T) {
	clock := time.Date(2026, 6, 8, 18, 46, 0, 0, time.UTC)
	evaluationRepo := &evaluationRepositorySpy{}
	service := asignacion.NewAssignmentApplicationServiceWithDependencies(
		&assignmentRepositorySpy{}, &assignmentCacheSpy{}, &activityRepositorySpy{},
		&toolRepositorySpy{}, evaluationRepo, nil, &eventPublisherSpy{}, fixedClock(clock),
	)

	evaluation, err := service.SubmitEvaluation(context.Background(), asignacion.SubmitEvaluationInput{
		AssignmentID: uuid.New(),
		EmployeeID:   uuid.New(),
		EvaluatorID:  uuid.New(),
		Rating:       4.5,
		Comments:     "Excelente actitud",
	})

	require.NoError(t, err)
	require.NotNil(t, evaluation)
	assert.Equal(t, 4, evaluation.Rating, "rating is persisted as integer 1-5 per the spec")
	assert.True(t, evaluation.EvaluationDate.Equal(clock))
	assert.True(t, evaluation.CreatedAt.Equal(clock))
	assert.Equal(t, "Excelente actitud", evaluation.Comments)
	require.NotNil(t, evaluationRepo.created)
}

func TestAssignmentApplicationServiceSubmitEvaluationRejectsRatingOutOfRange(t *testing.T) {
	evaluationRepo := &evaluationRepositorySpy{}
	service := asignacion.NewAssignmentApplicationServiceWithDependencies(
		&assignmentRepositorySpy{}, &assignmentCacheSpy{}, &activityRepositorySpy{},
		&toolRepositorySpy{}, evaluationRepo, nil, &eventPublisherSpy{}, fixedClock(time.Now().UTC()),
	)

	_, err := service.SubmitEvaluation(context.Background(), asignacion.SubmitEvaluationInput{
		AssignmentID: uuid.New(),
		EmployeeID:   uuid.New(),
		EvaluatorID:  uuid.New(),
		Rating:       6.0,
	})

	require.ErrorIs(t, err, asignacion.ErrInvalidEvaluationRating)
	assert.Nil(t, evaluationRepo.created, "no evaluation must be written on validation failure")
}

func TestAssignmentApplicationServiceGetEvaluationHistoryComputesAverageAndPagination(t *testing.T) {
	employeeID := uuid.New()
	evaluationRepo := &evaluationRepositorySpy{listed: []asignacion.EmployeeEvaluation{
		{ID: uuid.New(), EmployeeID: employeeID, Rating: 5},
		{ID: uuid.New(), EmployeeID: employeeID, Rating: 4},
		{ID: uuid.New(), EmployeeID: employeeID, Rating: 3},
	}}
	service := asignacion.NewAssignmentApplicationServiceWithDependencies(
		&assignmentRepositorySpy{}, &assignmentCacheSpy{}, &activityRepositorySpy{},
		&toolRepositorySpy{}, evaluationRepo, nil, &eventPublisherSpy{}, fixedClock(time.Now().UTC()),
	)

	result, err := service.GetEvaluationHistory(context.Background(), employeeID, asignacion.Pagination{Limit: 10, Offset: 0})

	require.NoError(t, err)
	assert.Equal(t, employeeID, evaluationRepo.listByID)
	assert.Equal(t, 10, evaluationRepo.listFilter.Limit)
	assert.Equal(t, 0, evaluationRepo.listFilter.Offset)
	assert.Equal(t, 3, result.EvaluationCount)
	assert.InDelta(t, 4.0, result.AverageRating, 1e-9)
	assert.Equal(t, asignacion.PaginationResult{Page: 1, Limit: 10, TotalRecords: 3, TotalPages: 1}, result.Pagination)
}

func TestAssignmentApplicationServiceGetEvaluationHistoryHandlesEmptyResult(t *testing.T) {
	employeeID := uuid.New()
	evaluationRepo := &evaluationRepositorySpy{listed: nil}
	service := asignacion.NewAssignmentApplicationServiceWithDependencies(
		&assignmentRepositorySpy{}, &assignmentCacheSpy{}, &activityRepositorySpy{},
		&toolRepositorySpy{}, evaluationRepo, nil, &eventPublisherSpy{}, fixedClock(time.Now().UTC()),
	)

	result, err := service.GetEvaluationHistory(context.Background(), employeeID, asignacion.Pagination{Limit: 10, Offset: 0})

	require.NoError(t, err)
	assert.Equal(t, 0, result.EvaluationCount)
	assert.Equal(t, 0.0, result.AverageRating)
	assert.Equal(t, asignacion.PaginationResult{Page: 1, Limit: 10, TotalRecords: 0, TotalPages: 0}, result.Pagination)
}
