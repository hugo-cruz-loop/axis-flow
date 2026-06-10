package asignacion

import (
	"context"
	"io"
	"time"

	"github.com/google/uuid"
)

// AssignmentRepository abstracts persistence for assignment aggregate roots.
type AssignmentRepository interface {
	Create(ctx context.Context, assignment *Assignment) error
	ReplaceCurrentForEmployee(ctx context.Context, assignment *Assignment, deactivatedAt time.Time) error
	FindByID(ctx context.Context, id uuid.UUID) (*Assignment, error)
	FindCurrentByEmployee(ctx context.Context, employeeID uuid.UUID) (*Assignment, error)
	ListByClient(ctx context.Context, clientID uuid.UUID, filter AssignmentListFilter) ([]Assignment, error)
	CountByClient(ctx context.Context, clientID uuid.UUID, filter AssignmentListFilter) (int, error)
	ListActiveBySupervisor(ctx context.Context, supervisorID uuid.UUID, filter Pagination) ([]Assignment, error)
	CountActiveBySupervisor(ctx context.Context, supervisorID uuid.UUID) (int, error)
	DeactivateCurrentForEmployee(ctx context.Context, employeeID uuid.UUID, deactivatedAt time.Time) error
	Update(ctx context.Context, assignment *Assignment) error
}

// AssignedActivityRepository abstracts persistence for generated assignment activities.
type AssignedActivityRepository interface {
	CreateBatch(ctx context.Context, activities []AssignedActivity) error
	ListByAssignment(ctx context.Context, assignmentID uuid.UUID, status *ActivityStatus) ([]AssignedActivity, error)
	UpdateEvidence(ctx context.Context, activityID uuid.UUID, evidence ActivityEvidenceUpdate) (*AssignedActivity, error)
	BatchUpdateStatus(ctx context.Context, updates []ActivityStatusUpdate) ([]AssignedActivity, error)
}

// AssignedToolRepository abstracts persistence for generated assignment tools.
type AssignedToolRepository interface {
	CreateBatch(ctx context.Context, tools []AssignedTool) error
	ListBySupervisor(ctx context.Context, supervisorID uuid.UUID, status *ToolDeliveryStatus) ([]AssignedTool, error)
}

// EmployeeEvaluationRepository abstracts persistence for assignment employee evaluations.
type EmployeeEvaluationRepository interface {
	Create(ctx context.Context, evaluation *EmployeeEvaluation) error
	ListByEmployee(ctx context.Context, employeeID uuid.UUID, filter Pagination) ([]EmployeeEvaluation, error)
}

// AssignmentService defines the application port used by HTTP handlers for assignment commands.
type AssignmentService interface {
	CreateAssignment(ctx context.Context, input CreateAssignmentInput) (*Assignment, error)
	ModifyAssignment(ctx context.Context, id uuid.UUID, input ModifyAssignmentInput) (*Assignment, error)
	ListAssignmentsByClient(ctx context.Context, clientID uuid.UUID, filter AssignmentListFilter) (AssignmentListResult, error)
	ListActiveAssignmentsBySupervisor(ctx context.Context, supervisorID uuid.UUID, pagination Pagination) (AssignmentListResult, error)
	ListActivitiesByAssignment(ctx context.Context, assignmentID uuid.UUID, status *ActivityStatus) ([]AssignedActivity, error)
	UploadActivityEvidence(ctx context.Context, activityID uuid.UUID, input UploadActivityEvidenceInput) (*AssignedActivity, error)
	BatchUpdateActivityStatus(ctx context.Context, updates []ActivityStatusUpdate) ([]AssignedActivity, error)
	ListToolsBySupervisor(ctx context.Context, supervisorID uuid.UUID, status *ToolDeliveryStatus) ([]AssignedTool, error)
	SubmitEvaluation(ctx context.Context, input SubmitEvaluationInput) (*EmployeeEvaluation, error)
	GetEvaluationHistory(ctx context.Context, employeeID uuid.UUID, pagination Pagination) (EvaluationHistoryResult, error)
}

// AssignmentEventPublisher publishes domain events for downstream consumers.
type AssignmentEventPublisher interface {
	PublishAsignacionModificada(ctx context.Context, event AsignacionModificadaEvent) error
	PublishEvidenciaCargada(ctx context.Context, event EvidenciaCargadaEvent) error
	PublishEmpleadoEvaluado(ctx context.Context, event EmpleadoEvaluadoEvent) error
}

// AssignmentCacheInvalidator invalidates cached assignment lookups after mutations.
type AssignmentCacheInvalidator interface {
	InvalidateEmployeeAssignments(ctx context.Context, employeeID uuid.UUID) error
}

// AssignmentProvisioner generates or refreshes assignment activities and tools.
type AssignmentProvisioner interface {
	ProvisionCreatedAssignment(ctx context.Context, assignment Assignment) (AssignmentProvisioningResult, error)
	RefreshModifiedAssignment(ctx context.Context, assignment Assignment) (AssignmentProvisioningResult, error)
}

// EvidenceUploader uploads one evidence payload to durable storage and returns
// the canonical public URL that should be persisted alongside the activity.
type EvidenceUploader interface {
	Upload(ctx context.Context, activityID uuid.UUID, slot int, content io.Reader, contentType, filename string) (canonicalURL string, err error)
}
