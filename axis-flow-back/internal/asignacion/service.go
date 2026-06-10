package asignacion

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Clock returns the current time. It is injectable to keep service tests deterministic.
type Clock func() time.Time

type noOpAssignmentProvisioner struct{}

func (noOpAssignmentProvisioner) ProvisionCreatedAssignment(context.Context, Assignment) (AssignmentProvisioningResult, error) {
	return AssignmentProvisioningResult{}, nil
}

func (noOpAssignmentProvisioner) RefreshModifiedAssignment(context.Context, Assignment) (AssignmentProvisioningResult, error) {
	return AssignmentProvisioningResult{}, nil
}

// noOpAssignmentEventPublisher is the default in-memory publisher used until a real
// Redis Streams transport is wired in (deferred to a follow-up PR).
type noOpAssignmentEventPublisher struct{}

func (noOpAssignmentEventPublisher) PublishAsignacionModificada(context.Context, AsignacionModificadaEvent) error {
	return nil
}

func (noOpAssignmentEventPublisher) PublishEvidenciaCargada(context.Context, EvidenciaCargadaEvent) error {
	return nil
}

// NewNoOpAssignmentEventPublisher returns a no-op AssignmentEventPublisher. It is the default
// wiring when no Redis Streams transport is registered (PR-3). A real transport will replace this.
func NewNoOpAssignmentEventPublisher() AssignmentEventPublisher {
	return noOpAssignmentEventPublisher{}
}

// AssignmentApplicationService coordinates assignment use cases.
type AssignmentApplicationService struct {
	repository     AssignmentRepository
	activityRepo   AssignedActivityRepository
	toolRepo       AssignedToolRepository
	evaluationRepo EmployeeEvaluationRepository
	cache          AssignmentCacheInvalidator
	provisioner    AssignmentProvisioner
	publisher      AssignmentEventPublisher
	clock          Clock
}

// NewAssignmentApplicationService creates an assignment application service.
func NewAssignmentApplicationService(repository AssignmentRepository, cache AssignmentCacheInvalidator, clock Clock) *AssignmentApplicationService {
	return NewAssignmentApplicationServiceWithDependencies(repository, cache, nil, nil, nil, noOpAssignmentProvisioner{}, noOpAssignmentEventPublisher{}, clock)
}

// NewAssignmentApplicationServiceWithDependencies creates an assignment service with explicit infrastructure ports.
func NewAssignmentApplicationServiceWithDependencies(
	repository AssignmentRepository,
	cache AssignmentCacheInvalidator,
	activityRepo AssignedActivityRepository,
	toolRepo AssignedToolRepository,
	evaluationRepo EmployeeEvaluationRepository,
	provisioner AssignmentProvisioner,
	publisher AssignmentEventPublisher,
	clock Clock,
) *AssignmentApplicationService {
	if clock == nil {
		clock = func() time.Time { return time.Now().UTC() }
	}
	if provisioner == nil {
		provisioner = noOpAssignmentProvisioner{}
	}
	if publisher == nil {
		publisher = noOpAssignmentEventPublisher{}
	}
	return &AssignmentApplicationService{
		repository:     repository,
		activityRepo:   activityRepo,
		toolRepo:       toolRepo,
		evaluationRepo: evaluationRepo,
		cache:          cache,
		provisioner:    provisioner,
		publisher:      publisher,
		clock:          clock,
	}
}

// CreateAssignment atomically replaces the employee's current assignment and creates the new current assignment.
func (s *AssignmentApplicationService) CreateAssignment(ctx context.Context, input CreateAssignmentInput) (*Assignment, error) {
	now := s.clock()
	assignment, err := NewAssignment(NewAssignmentParams{
		CompanyID:   input.CompanyID,
		EmployeeID:  input.EmployeeID,
		LocationID:  input.LocationID,
		ServiceID:   input.ServiceID,
		ShiftID:     input.ShiftID,
		StartDate:   input.StartDate,
		EndDate:     input.EndDate,
		GeneratedAt: now,
	})
	if err != nil {
		return nil, err
	}

	if err := s.repository.ReplaceCurrentForEmployee(ctx, assignment, now); err != nil {
		return nil, fmt.Errorf("replace current assignment: %w", err)
	}

	provisioned, err := s.provisioner.ProvisionCreatedAssignment(ctx, *assignment)
	if err != nil {
		return nil, fmt.Errorf("provision assignment resources: %w", err)
	}
	assignment.GeneratedActivitiesCount = provisioned.ActivitiesCount
	assignment.GeneratedToolsCount = provisioned.ToolsCount

	if s.cache != nil {
		if err := s.cache.InvalidateEmployeeAssignments(ctx, input.EmployeeID); err != nil {
			return nil, fmt.Errorf("invalidate assignment cache: %w", err)
		}
	}
	return assignment, nil
}

// ModifyAssignment updates mutable assignment fields and invalidates active lookup cache.
func (s *AssignmentApplicationService) ModifyAssignment(ctx context.Context, id uuid.UUID, input ModifyAssignmentInput) (*Assignment, error) {
	assignment, err := s.repository.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if assignment == nil {
		return nil, ErrAssignmentNotFound
	}

	if input.LocationID != nil {
		assignment.LocationID = *input.LocationID
	}
	if input.ShiftID != nil {
		assignment.ShiftID = *input.ShiftID
	}
	if input.EndDate != nil {
		assignment.EndDate = input.EndDate
	}
	if input.Status != nil {
		assignment.Status = *input.Status
	}
	if err := ValidateAssignmentDateWindow(assignment.StartDate, assignment.EndDate); err != nil {
		return nil, err
	}
	assignment.UpdatedAt = s.clock()

	if err := s.repository.Update(ctx, assignment); err != nil {
		return nil, fmt.Errorf("update assignment: %w", err)
	}
	provisioned, err := s.provisioner.RefreshModifiedAssignment(ctx, *assignment)
	if err != nil {
		return nil, fmt.Errorf("refresh assignment resources: %w", err)
	}
	assignment.UpdatedActivitiesCount = provisioned.ActivitiesCount
	assignment.UpdatedToolsCount = provisioned.ToolsCount

	if s.cache != nil {
		if err := s.cache.InvalidateEmployeeAssignments(ctx, assignment.EmployeeID); err != nil {
			return nil, fmt.Errorf("invalidate assignment cache: %w", err)
		}
	}
	if s.publisher != nil {
		if err := s.publisher.PublishAsignacionModificada(ctx, AsignacionModificadaEvent{
			AssignmentID: assignment.ID,
			EmployeeID:   assignment.EmployeeID,
			LocationID:   assignment.LocationID,
			UpdatedAt:    assignment.UpdatedAt,
		}); err != nil {
			return nil, fmt.Errorf("publish asignacion modificada: %w", err)
		}
	}
	return assignment, nil
}

// ListAssignmentsByClient returns paginated assignments for a client.
func (s *AssignmentApplicationService) ListAssignmentsByClient(ctx context.Context, clientID uuid.UUID, filter AssignmentListFilter) (AssignmentListResult, error) {
	filter.Pagination = filter.Pagination.Normalize()
	items, err := s.repository.ListByClient(ctx, clientID, filter)
	if err != nil {
		return AssignmentListResult{}, fmt.Errorf("list assignments by client: %w", err)
	}
	total, err := s.repository.CountByClient(ctx, clientID, filter)
	if err != nil {
		return AssignmentListResult{}, fmt.Errorf("count assignments by client: %w", err)
	}
	return AssignmentListResult{Items: items, Pagination: NewPaginationResult(filter.Pagination, total)}, nil
}

// ListActiveAssignmentsBySupervisor returns a clear service error until a real supervisor scope relation exists.
func (s *AssignmentApplicationService) ListActiveAssignmentsBySupervisor(ctx context.Context, supervisorID uuid.UUID, pagination Pagination) (AssignmentListResult, error) {
	return AssignmentListResult{}, ErrAssignmentSupervisorScopeNotImplemented
}

// ListActivitiesByAssignment returns the activities for an assignment, optionally filtered by status.
func (s *AssignmentApplicationService) ListActivitiesByAssignment(ctx context.Context, assignmentID uuid.UUID, status *ActivityStatus) ([]AssignedActivity, error) {
	if s.activityRepo == nil {
		return nil, fmt.Errorf("activity repository not configured")
	}
	return s.activityRepo.ListByAssignment(ctx, assignmentID, status)
}

// UploadActivityEvidence persists evidence URLs and coordinates, marks the activity as completed, publishes EvidenciaCargada and invalidates the assignment cache.
func (s *AssignmentApplicationService) UploadActivityEvidence(ctx context.Context, activityID uuid.UUID, input UploadActivityEvidenceInput) (*AssignedActivity, error) {
	if s.activityRepo == nil {
		return nil, fmt.Errorf("activity repository not configured")
	}
	if len(input.Files) == 0 {
		return nil, fmt.Errorf("%w: at least one evidence file is required", ErrInvalidEvidenceFile)
	}
	if err := ValidateCoordinate(GPSCoordinate{Latitude: input.Latitude, Longitude: input.Longitude}); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidEvidenceFile, err)
	}
	for _, file := range input.Files {
		if err := ValidateEvidenceFile(file); err != nil {
			return nil, err
		}
	}
	urls := make([]string, 0, len(input.Files))
	for _, file := range input.Files {
		urls = append(urls, file.SubmittedEvidenceURL(activityID))
	}
	now := s.clock()
	update := ActivityEvidenceUpdate{
		EvidenceURLs: urls,
		Latitude:     input.Latitude,
		Longitude:    input.Longitude,
		Comment:      input.Comment,
		CompletedAt:  now,
	}
	activity, err := s.activityRepo.UpdateEvidence(ctx, activityID, update)
	if err != nil {
		return nil, fmt.Errorf("upload activity evidence: %w", err)
	}
	if s.publisher != nil {
		if err := s.publisher.PublishEvidenciaCargada(ctx, EvidenciaCargadaEvent{
			ActivityID:   activityID,
			AssignmentID: activity.AssignmentID,
			EmployeeID:   input.EmployeeID,
			EvidenceURLs: urls,
			GPSCoordinates: GPSCoordinate{
				Latitude:  input.Latitude,
				Longitude: input.Longitude,
			},
			UploadedAt: now,
		}); err != nil {
			return nil, fmt.Errorf("publish evidencia cargada: %w", err)
		}
	}
	if s.cache != nil {
		if err := s.cache.InvalidateEmployeeAssignments(ctx, input.EmployeeID); err != nil {
			return nil, fmt.Errorf("invalidate assignment cache: %w", err)
		}
	}
	return activity, nil
}

// BatchUpdateActivityStatus updates the status of many activities at once.
func (s *AssignmentApplicationService) BatchUpdateActivityStatus(ctx context.Context, updates []ActivityStatusUpdate) ([]AssignedActivity, error) {
	if len(updates) == 0 {
		return nil, ErrBatchUpdateEmpty
	}
	if s.activityRepo == nil {
		return nil, fmt.Errorf("activity repository not configured")
	}
	now := s.clock()
	stamped := make([]ActivityStatusUpdate, len(updates))
	copy(stamped, updates)
	for i := range stamped {
		if stamped[i].UpdatedAt.IsZero() {
			stamped[i].UpdatedAt = now
		}
	}
	updated, err := s.activityRepo.BatchUpdateStatus(ctx, stamped)
	if err != nil {
		return nil, fmt.Errorf("batch update activity status: %w", err)
	}
	return updated, nil
}

// ListToolsBySupervisor returns tools assigned under a supervisor scope, optionally filtered by delivery status.
func (s *AssignmentApplicationService) ListToolsBySupervisor(ctx context.Context, supervisorID uuid.UUID, status *ToolDeliveryStatus) ([]AssignedTool, error) {
	if s.toolRepo == nil {
		return nil, fmt.Errorf("tool repository not configured")
	}
	return s.toolRepo.ListBySupervisor(ctx, supervisorID, status)
}

// SubmitEvaluation persists a periodic performance evaluation for an assignment.
func (s *AssignmentApplicationService) SubmitEvaluation(ctx context.Context, input SubmitEvaluationInput) (*EmployeeEvaluation, error) {
	if input.AssignmentID == uuid.Nil || input.EmployeeID == uuid.Nil || input.EvaluatorID == uuid.Nil {
		return nil, ErrMissingAssignmentRequiredField
	}
	if input.Rating < 1.0 || input.Rating > 5.0 {
		return nil, ErrInvalidEvaluationRating
	}
	if s.evaluationRepo == nil {
		return nil, fmt.Errorf("evaluation repository not configured")
	}
	evaluation := &EmployeeEvaluation{
		ID:                  uuid.New(),
		AssignmentID:        input.AssignmentID,
		EmployeeID:          input.EmployeeID,
		EvaluatorID:         input.EvaluatorID,
		Rating:              int(input.Rating),
		ActivitiesCompliant: true,
		Comments:            input.Comments,
		EvaluationDate:      s.clock(),
		CreatedAt:           s.clock(),
	}
	if err := s.evaluationRepo.Create(ctx, evaluation); err != nil {
		return nil, fmt.Errorf("submit evaluation: %w", err)
	}
	return evaluation, nil
}

// GetEvaluationHistory returns paginated evaluation history and aggregate statistics for an employee.
func (s *AssignmentApplicationService) GetEvaluationHistory(ctx context.Context, employeeID uuid.UUID, pagination Pagination) (EvaluationHistoryResult, error) {
	if s.evaluationRepo == nil {
		return EvaluationHistoryResult{}, fmt.Errorf("evaluation repository not configured")
	}
	filter := pagination.Normalize()
	items, err := s.evaluationRepo.ListByEmployee(ctx, employeeID, filter)
	if err != nil {
		return EvaluationHistoryResult{}, fmt.Errorf("list evaluations: %w", err)
	}
	sum := 0
	for i := range items {
		sum += items[i].Rating
	}
	avg := 0.0
	if len(items) > 0 {
		avg = float64(sum) / float64(len(items))
	}
	return EvaluationHistoryResult{
		Items:           items,
		AverageRating:   avg,
		EvaluationCount: len(items),
		Pagination:      NewPaginationResult(filter, len(items)),
	}, nil
}
