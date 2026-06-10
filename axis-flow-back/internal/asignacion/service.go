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

// defaultAssignmentProvisioner materializes AssignedActivity and AssignedTool
// rows for a newly created or modified assignment, derived deterministically
// from the assignment's ServiceID. It is a placeholder for the future
// service-template-driven provisioner that will read from a servicios table.
type defaultAssignmentProvisioner struct {
	activityRepo AssignedActivityRepository
	toolRepo     AssignedToolRepository
	clock        Clock
}

// NewDefaultAssignmentProvisioner creates a provisioner that persists
// deterministic activity and tool rows. Pass nil for either repo to disable
// persistence for that side (counts still return so handlers can report them).
func NewDefaultAssignmentProvisioner(activityRepo AssignedActivityRepository, toolRepo AssignedToolRepository, clock Clock) AssignmentProvisioner {
	if clock == nil {
		clock = func() time.Time { return time.Now().UTC() }
	}
	return &defaultAssignmentProvisioner{activityRepo: activityRepo, toolRepo: toolRepo, clock: clock}
}

// ProvisionCreatedAssignment materializes activities and tools for a new assignment.
func (p *defaultAssignmentProvisioner) ProvisionCreatedAssignment(ctx context.Context, a Assignment) (AssignmentProvisioningResult, error) {
	return p.provision(ctx, a, false)
}

// RefreshModifiedAssignment re-materializes activities and tools after an assignment update.
func (p *defaultAssignmentProvisioner) RefreshModifiedAssignment(ctx context.Context, a Assignment) (AssignmentProvisioningResult, error) {
	return p.provision(ctx, a, true)
}

func (p *defaultAssignmentProvisioner) provision(ctx context.Context, a Assignment, refresh bool) (AssignmentProvisioningResult, error) {
	now := p.clock()
	activities := buildSeedActivities(a, now)
	tools := buildSeedTools(a, now)
	if p.activityRepo != nil {
		if err := p.activityRepo.CreateBatch(ctx, activities); err != nil {
			return AssignmentProvisioningResult{}, fmt.Errorf("default_provisioner.CreateBatch activities: %w", err)
		}
	}
	if p.toolRepo != nil {
		if err := p.toolRepo.CreateBatch(ctx, tools); err != nil {
			return AssignmentProvisioningResult{}, fmt.Errorf("default_provisioner.CreateBatch tools: %w", err)
		}
	}
	if refresh {
		// Re-materialize clears the previous counts; the handler uses Updated*Counts
		// for the response. We return the same counts under both names so callers
		// can render either pair depending on the operation.
		return AssignmentProvisioningResult{
			ActivitiesCount: len(activities),
			ToolsCount:      len(tools),
		}, nil
	}
	return AssignmentProvisioningResult{
		ActivitiesCount: len(activities),
		ToolsCount:      len(tools),
	}, nil
}

// buildSeedActivities derives 2-4 deterministic activity rows from the service ID.
func buildSeedActivities(a Assignment, now time.Time) []AssignedActivity {
	serviceBytes := a.ServiceID[:]
	activityCount := 2 + int(serviceBytes[0])%3 // 2..4
	rows := make([]AssignedActivity, 0, activityCount)
	for i := 0; i < activityCount; i++ {
		id := uuid.New()
		rows = append(rows, AssignedActivity{
			ID:           id,
			AssignmentID: a.ID,
			ActivityID:   id,
			Description:  defaultActivityDescription(i),
			Frequency:    "DIARIA",
			Order:        i + 1,
			Status:       ActivityStatusPending,
			CreatedAt:    now,
			UpdatedAt:    now,
		})
	}
	return rows
}

// buildSeedTools derives 1-2 deterministic tool rows from the service ID.
func buildSeedTools(a Assignment, now time.Time) []AssignedTool {
	serviceBytes := a.ServiceID[:]
	toolCount := 1 + int(serviceBytes[1])%2 // 1..2
	rows := make([]AssignedTool, 0, toolCount)
	for i := 0; i < toolCount; i++ {
		id := uuid.New()
		rows = append(rows, AssignedTool{
			ID:             id,
			AssignmentID:   a.ID,
			ToolID:         id,
			Name:           defaultToolName(i),
			Quantity:       1,
			Specifications: "",
			DeliveryStatus: ToolDeliveryStatusPending,
			CreatedAt:      now,
			UpdatedAt:      now,
		})
	}
	return rows
}

func defaultActivityDescription(idx int) string {
	switch idx % 4 {
	case 0:
		return "Inspeccion general del area asignada"
	case 1:
		return "Reporte de novedades al supervisor"
	case 2:
		return "Carga de evidencia fotografica"
	default:
		return "Verificacion de cierre y entrega"
	}
}

func defaultToolName(idx int) string {
	switch idx % 2 {
	case 0:
		return "Equipo de proteccion personal"
	default:
		return "Dispositivo de marcaje"
	}
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

func (noOpAssignmentEventPublisher) PublishEmpleadoEvaluado(context.Context, EmpleadoEvaluadoEvent) error {
	return nil
}

// NewNoOpAssignmentEventPublisher returns a no-op AssignmentEventPublisher. It is the default
// wiring when no Redis Streams transport is registered (PR-3). A real transport will replace this.
func NewNoOpAssignmentEventPublisher() AssignmentEventPublisher {
	return noOpAssignmentEventPublisher{}
}

// AssignmentApplicationService coordinates assignment use cases.
type AssignmentApplicationService struct {
	repository       AssignmentRepository
	activityRepo     AssignedActivityRepository
	toolRepo         AssignedToolRepository
	evaluationRepo   EmployeeEvaluationRepository
	cache            AssignmentCacheInvalidator
	provisioner      AssignmentProvisioner
	publisher        AssignmentEventPublisher
	evidenceUploader EvidenceUploader
	clock            Clock
}

// NewAssignmentApplicationService creates an assignment application service.
func NewAssignmentApplicationService(repository AssignmentRepository, cache AssignmentCacheInvalidator, clock Clock) *AssignmentApplicationService {
	return NewAssignmentApplicationServiceWithDependencies(repository, cache, nil, nil, nil, noOpAssignmentProvisioner{}, noOpAssignmentEventPublisher{}, nil, clock)
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
	uploader EvidenceUploader,
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
		repository:       repository,
		activityRepo:     activityRepo,
		toolRepo:         toolRepo,
		evaluationRepo:   evaluationRepo,
		cache:            cache,
		provisioner:      provisioner,
		publisher:        publisher,
		evidenceUploader: uploader,
		clock:            clock,
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
	if s.evidenceUploader != nil {
		for _, file := range input.Files {
			canonical, err := s.evidenceUploader.Upload(ctx, activityID, file.Slot, file.Reader, file.ContentType, file.Filename)
			if err != nil {
				return nil, fmt.Errorf("upload evidence slot %d: %w", file.Slot, err)
			}
			urls = append(urls, canonical)
		}
	} else {
		for _, file := range input.Files {
			urls = append(urls, file.SubmittedEvidenceURL(activityID))
		}
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
	if s.publisher != nil {
		if err := s.publisher.PublishEmpleadoEvaluado(ctx, EmpleadoEvaluadoEvent{
			EvaluationID:   evaluation.ID,
			AssignmentID:   evaluation.AssignmentID,
			EmployeeID:     evaluation.EmployeeID,
			EvaluatorID:    evaluation.EvaluatorID,
			Rating:         evaluation.Rating,
			EvaluationDate: evaluation.EvaluationDate,
		}); err != nil {
			return nil, fmt.Errorf("publish empleado evaluado: %w", err)
		}
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
