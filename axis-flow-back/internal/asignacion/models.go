// Package asignacion defines the Asignacion bounded context domain models.
package asignacion

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// AssignmentStatus represents the lifecycle state of an assignment.
type AssignmentStatus int

const (
	// DateLayout is the OpenAPI date format for assignment requests and responses.
	DateLayout = "2006-01-02"
)

const (
	// AssignmentStatusActive marks the assignment as operationally active.
	AssignmentStatusActive AssignmentStatus = 1
	// AssignmentStatusInactive marks the assignment as inactive or baja.
	AssignmentStatusInactive AssignmentStatus = 2
)

// ActivityStatus represents the completion state of an assigned activity.
type ActivityStatus int

const (
	// ActivityStatusPending means the activity still needs execution.
	ActivityStatusPending ActivityStatus = 1
	// ActivityStatusCompleted means the activity was completed with its required evidence.
	ActivityStatusCompleted ActivityStatus = 2
	// ActivityStatusCancelled means the activity was cancelled or failed operationally.
	ActivityStatusCancelled ActivityStatus = 3
)

// ToolDeliveryStatus represents the delivery state of an assigned tool.
type ToolDeliveryStatus int

const (
	// ToolDeliveryStatusPending means the tool still needs to be delivered.
	ToolDeliveryStatusPending ToolDeliveryStatus = 1
	// ToolDeliveryStatusDelivered means the employee received the tool.
	ToolDeliveryStatusDelivered ToolDeliveryStatus = 2
	// ToolDeliveryStatusReturned means the employee returned the tool.
	ToolDeliveryStatusReturned ToolDeliveryStatus = 3
)

var (
	// ErrInvalidAssignmentDateWindow is returned when an assignment end date is before the start date.
	ErrInvalidAssignmentDateWindow = errors.New("assignment end date must be on or after start date")
	// ErrMissingAssignmentRequiredField is returned when a required assignment identifier or date is missing.
	ErrMissingAssignmentRequiredField = errors.New("assignment is missing a required field")
	// ErrAssignmentNotFound is returned when an assignment cannot be found.
	ErrAssignmentNotFound = errors.New("assignment not found")
	// ErrAssignmentConflict is returned when an assignment conflicts with current state.
	ErrAssignmentConflict = errors.New("assignment conflict")
	// ErrAssignmentSupervisorScopeNotImplemented is returned until the data model exposes a real supervisor-to-assignment scope relation.
	ErrAssignmentSupervisorScopeNotImplemented = errors.New("active assignments by supervisor is not implemented until a supervisor scope relation exists")
	// ErrInvalidActivityStatusFilter is returned when an activity status filter is not in the allowed set.
	ErrInvalidActivityStatusFilter = errors.New("activity status filter must be PENDING, COMPLETED or FAILED")
	// ErrInvalidToolDeliveryStatusFilter is returned when a tool delivery status filter is not in the allowed set.
	ErrInvalidToolDeliveryStatusFilter = errors.New("tool delivery status filter must be ASSIGNED, DELIVERED or RETURNED")
	// ErrInvalidEvaluationRating is returned when an evaluation rating is outside the [1,5] range.
	ErrInvalidEvaluationRating = errors.New("evaluation rating must be between 1.0 and 5.0")
	// ErrActivityNotFound is returned when an assigned activity cannot be found.
	ErrActivityNotFound = errors.New("activity not found")
	// ErrBatchUpdateEmpty is returned when a batch update is requested with no entries.
	ErrBatchUpdateEmpty = errors.New("batch update must contain at least one activity")
	// ErrInvalidEvidenceFile is returned when an uploaded evidence file fails validation.
	ErrInvalidEvidenceFile = errors.New("evidence file is invalid")
)

// NewAssignmentParams contains the required input for a new assignment entity.
type NewAssignmentParams struct {
	ID          uuid.UUID
	CompanyID   uuid.UUID
	EmployeeID  uuid.UUID
	LocationID  uuid.UUID
	ServiceID   uuid.UUID
	ShiftID     uuid.UUID
	StartDate   time.Time
	EndDate     *time.Time
	GeneratedAt time.Time
}

// Assignment is the source-of-truth domain entity for linking an employee to a client location, service and shift.
type Assignment struct {
	ID                       uuid.UUID
	CompanyID                uuid.UUID
	EmployeeID               uuid.UUID
	LocationID               uuid.UUID
	ServiceID                uuid.UUID
	ShiftID                  uuid.UUID
	Status                   AssignmentStatus
	IsCurrent                bool
	StartDate                time.Time
	EndDate                  *time.Time
	CreatedAt                time.Time
	UpdatedAt                time.Time
	GeneratedActivitiesCount int
	GeneratedToolsCount      int
	UpdatedActivitiesCount   int
	UpdatedToolsCount        int
}

// NewAssignment creates an active current assignment after validating required invariants.
func NewAssignment(params NewAssignmentParams) (*Assignment, error) {
	if params.CompanyID == uuid.Nil || params.EmployeeID == uuid.Nil || params.LocationID == uuid.Nil || params.ServiceID == uuid.Nil || params.ShiftID == uuid.Nil || params.StartDate.IsZero() {
		return nil, ErrMissingAssignmentRequiredField
	}
	if err := ValidateAssignmentDateWindow(params.StartDate, params.EndDate); err != nil {
		return nil, err
	}

	id := params.ID
	if id == uuid.Nil {
		id = uuid.New()
	}
	generatedAt := params.GeneratedAt
	if generatedAt.IsZero() {
		generatedAt = time.Now().UTC()
	}

	return &Assignment{
		ID:         id,
		CompanyID:  params.CompanyID,
		EmployeeID: params.EmployeeID,
		LocationID: params.LocationID,
		ServiceID:  params.ServiceID,
		ShiftID:    params.ShiftID,
		Status:     AssignmentStatusActive,
		IsCurrent:  true,
		StartDate:  params.StartDate,
		EndDate:    params.EndDate,
		CreatedAt:  generatedAt,
		UpdatedAt:  generatedAt,
	}, nil
}

// ValidateAssignmentDateWindow enforces that an optional end date cannot precede the start date.
func ValidateAssignmentDateWindow(startDate time.Time, endDate *time.Time) error {
	if startDate.IsZero() {
		return ErrMissingAssignmentRequiredField
	}
	if endDate != nil && endDate.Before(startDate) {
		return ErrInvalidAssignmentDateWindow
	}
	return nil
}

// ActivityStatusFromString parses a human-readable activity status into the enum.
func ActivityStatusFromString(value string) (ActivityStatus, error) {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "PENDING", "PENDIENTE", "1":
		return ActivityStatusPending, nil
	case "COMPLETED", "COMPLETADO", "2":
		return ActivityStatusCompleted, nil
	case "FAILED", "CANCELLED", "CANCELADO", "3":
		return ActivityStatusCancelled, nil
	default:
		return 0, ErrInvalidActivityStatusFilter
	}
}

// String returns the canonical wire form for the activity status.
func (s ActivityStatus) String() string {
	switch s {
	case ActivityStatusPending:
		return "PENDING"
	case ActivityStatusCompleted:
		return "COMPLETED"
	case ActivityStatusCancelled:
		return "FAILED"
	default:
		return ""
	}
}

// ToolDeliveryStatusFromString parses a human-readable tool status into the enum.
func ToolDeliveryStatusFromString(value string) (ToolDeliveryStatus, error) {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "ASSIGNED", "PENDIENTE", "1":
		return ToolDeliveryStatusPending, nil
	case "DELIVERED", "ENTREGADO", "2":
		return ToolDeliveryStatusDelivered, nil
	case "RETURNED", "DEVUELTO", "3":
		return ToolDeliveryStatusReturned, nil
	default:
		return 0, ErrInvalidToolDeliveryStatusFilter
	}
}

// String returns the canonical wire form for the tool delivery status.
func (s ToolDeliveryStatus) String() string {
	switch s {
	case ToolDeliveryStatusPending:
		return "ASSIGNED"
	case ToolDeliveryStatusDelivered:
		return "DELIVERED"
	case ToolDeliveryStatusReturned:
		return "RETURNED"
	default:
		return ""
	}
}

// AllowedEvidenceContentTypes is the allowlist of evidence image MIME types.
var AllowedEvidenceContentTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/jpg":  ".jpg",
	"image/png":  ".png",
}

// MaxEvidenceFileSize caps a single evidence upload at 5MB per the spec.
const MaxEvidenceFileSize int64 = 5 * 1024 * 1024

// ValidateEvidenceFile checks that one uploaded evidence file is non-empty, within the 5MB cap, and uses an allowed MIME type.
func ValidateEvidenceFile(file EvidenceFile) error {
	if strings.TrimSpace(file.Filename) == "" {
		return fmt.Errorf("%w: filename is required", ErrInvalidEvidenceFile)
	}
	if file.SizeBytes <= 0 {
		return fmt.Errorf("%w: file is empty", ErrInvalidEvidenceFile)
	}
	if file.SizeBytes > MaxEvidenceFileSize {
		return fmt.Errorf("%w: file exceeds 5MB", ErrInvalidEvidenceFile)
	}
	if _, ok := AllowedEvidenceContentTypes[strings.ToLower(file.ContentType)]; !ok {
		return fmt.Errorf("%w: only image/jpeg and image/png are allowed", ErrInvalidEvidenceFile)
	}
	return nil
}

// ValidateCoordinate enforces the spec's GPS ranges for lat/lng.
func ValidateCoordinate(coord GPSCoordinate) error {
	if coord.Latitude < -90.0 || coord.Latitude > 90.0 {
		return fmt.Errorf("latitude must be between -90 and 90")
	}
	if coord.Longitude < -180.0 || coord.Longitude > 180.0 {
		return fmt.Errorf("longitude must be between -180 and 180")
	}
	return nil
}

// AssignedActivity is a generated activity checklist item for an assignment.
type AssignedActivity struct {
	ID              uuid.UUID
	AssignmentID    uuid.UUID
	ActivityID      uuid.UUID
	Description     string
	Frequency       string
	Order           int
	Status          ActivityStatus
	Comments        string
	Evidence1       string
	Evidence2       string
	Evidence3       string
	UploadLatitude  *float64
	UploadLongitude *float64
	ExecutionDate   *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// EvidenceURLs returns the persisted evidence URLs in storage order, excluding empty slots.
func (a AssignedActivity) EvidenceURLs() []string {
	urls := make([]string, 0, 3)
	for _, evidence := range []string{a.Evidence1, a.Evidence2, a.Evidence3} {
		if evidence != "" {
			urls = append(urls, evidence)
		}
	}
	return urls
}

// AssignedTool is a generated equipment requirement for an assignment.
type AssignedTool struct {
	ID             uuid.UUID
	AssignmentID   uuid.UUID
	ToolID         uuid.UUID
	Name           string
	Quantity       int
	Specifications string
	DeliveryStatus ToolDeliveryStatus
	DeliveredAt    *time.Time
	ReturnedAt     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// EmployeeEvaluation records a supervisor evaluation for an employee assignment.
type EmployeeEvaluation struct {
	ID                  uuid.UUID
	AssignmentID        uuid.UUID
	EmployeeID          uuid.UUID
	EvaluatorID         uuid.UUID
	Rating              int
	ActivitiesCompliant bool
	Comments            string
	EvaluationDate      time.Time
	CreatedAt           time.Time
}

// Pagination defines shared offset pagination input.
type Pagination struct {
	Limit  int
	Offset int
}

// Normalize applies default and max values to pagination input.
func (p Pagination) Normalize() Pagination {
	limit := p.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	offset := p.Offset
	if offset < 0 {
		offset = 0
	}
	return Pagination{Limit: limit, Offset: offset}
}

// Page returns the one-based page number for the normalized pagination input.
func (p Pagination) Page() int {
	normalized := p.Normalize()
	return (normalized.Offset / normalized.Limit) + 1
}

// PaginationResult is the OpenAPI pagination response metadata.
type PaginationResult struct {
	Page         int `json:"page"`
	Limit        int `json:"limit"`
	TotalRecords int `json:"total_records"`
	TotalPages   int `json:"total_pages"`
}

// NewPaginationResult creates response pagination metadata from input and total records.
func NewPaginationResult(input Pagination, totalRecords int) PaginationResult {
	normalized := input.Normalize()
	totalPages := 0
	if totalRecords > 0 {
		totalPages = (totalRecords + normalized.Limit - 1) / normalized.Limit
	}
	return PaginationResult{
		Page:         normalized.Page(),
		Limit:        normalized.Limit,
		TotalRecords: totalRecords,
		TotalPages:   totalPages,
	}
}

// AssignmentListFilter defines assignment history filtering input.
type AssignmentListFilter struct {
	Pagination
	OnlyCurrent *bool
}

// ActivityEvidenceUpdate contains evidence metadata to persist for an activity.
type ActivityEvidenceUpdate struct {
	EvidenceURLs []string
	Latitude     float64
	Longitude    float64
	Comment      string
	CompletedAt  time.Time
}

// ActivityStatusUpdate contains a status mutation for one assigned activity.
type ActivityStatusUpdate struct {
	ActivityID               uuid.UUID
	Status                   ActivityStatus
	Comment                  string
	UpdatedAt                time.Time
	GeneratedActivitiesCount int
	GeneratedToolsCount      int
	UpdatedActivitiesCount   int
	UpdatedToolsCount        int
}

// CreateAssignmentInput contains application-service input for creating an assignment.
type CreateAssignmentInput struct {
	CompanyID  uuid.UUID
	EmployeeID uuid.UUID
	LocationID uuid.UUID
	ServiceID  uuid.UUID
	ShiftID    uuid.UUID
	StartDate  time.Time
	EndDate    *time.Time
}

// ModifyAssignmentInput contains application-service input for changing an assignment.
type ModifyAssignmentInput struct {
	LocationID *uuid.UUID
	ShiftID    *uuid.UUID
	EndDate    *time.Time
	Status     *AssignmentStatus
}

// AssignmentListResult wraps assignment list data with pagination metadata.
type AssignmentListResult struct {
	Items      []Assignment
	Pagination PaginationResult
}

// AssignmentProvisioningResult reports generated or refreshed resource counts for API responses.
type AssignmentProvisioningResult struct {
	ActivitiesCount int
	ToolsCount      int
}

// GPSCoordinate represents a geographic point captured with an evidence upload.
type GPSCoordinate struct {
	Latitude  float64
	Longitude float64
}

// AssignmentChangeSummary details which activities or tools were added or removed when an assignment is modified.
type AssignmentChangeSummary struct {
	AddedActivityIDs   []uuid.UUID
	RemovedActivityIDs []uuid.UUID
	AddedToolIDs       []uuid.UUID
	RemovedToolIDs     []uuid.UUID
}

// AsignacionModificadaEvent is published after a successful assignment modification.
type AsignacionModificadaEvent struct {
	AssignmentID uuid.UUID
	EmployeeID   uuid.UUID
	LocationID   uuid.UUID
	Changes      AssignmentChangeSummary
	UpdatedAt    time.Time
}

// Name returns the canonical event name.
func (e AsignacionModificadaEvent) Name() string { return "AsignacionModificada" }

// EvidenciaCargadaEvent is published after a successful evidence upload.
type EvidenciaCargadaEvent struct {
	ActivityID     uuid.UUID
	AssignmentID   uuid.UUID
	EmployeeID     uuid.UUID
	EvidenceURLs   []string
	GPSCoordinates GPSCoordinate
	UploadedAt     time.Time
}

// Name returns the canonical event name.
func (e EvidenciaCargadaEvent) Name() string { return "EvidenciaCargada" }

// UploadActivityEvidenceInput is the application-service input for evidence upload.
type UploadActivityEvidenceInput struct {
	EmployeeID uuid.UUID
	Latitude   float64
	Longitude  float64
	Comment    string
	Files      []EvidenceFile
}

// EvidenceFile describes one uploaded evidence payload and its target slot.
type EvidenceFile struct {
	Slot        int
	Filename    string
	ContentType string
	SizeBytes   int64
}

// SubmittedEvidenceURL resolves the public storage URL for an uploaded evidence file.
func (e EvidenceFile) SubmittedEvidenceURL(activityID uuid.UUID) string {
	return BuildEvidenceStorageURL(activityID, e.Slot, e.Filename)
}

// BuildEvidenceStorageURL composes the placeholder S3 URL persisted alongside the activity.
// PR-3 ships a deterministic placeholder; a real S3 client is deferred to a follow-up slice.
func BuildEvidenceStorageURL(activityID uuid.UUID, slot int, filename string) string {
	ext := "jpg"
	lower := strings.ToLower(filename)
	switch {
	case strings.HasSuffix(lower, ".png"):
		ext = "png"
	case strings.HasSuffix(lower, ".jpeg"), strings.HasSuffix(lower, ".jpg"):
		ext = "jpg"
	}
	return fmt.Sprintf("https://s3.amazonaws.com/checkon-evidences/evidencia_%d_%s.%s", slot, activityID.String(), ext)
}

// EvaluationCriterion is a granular sub-score for a periodic performance evaluation.
type EvaluationCriterion struct {
	Name   string
	Points float64
}

// SubmitEvaluationInput is the application-service input for submitting a performance evaluation.
type SubmitEvaluationInput struct {
	AssignmentID uuid.UUID
	EmployeeID   uuid.UUID
	EvaluatorID  uuid.UUID
	Rating       float64
	Comments     string
	Criteria     []EvaluationCriterion
}

// EvaluationHistoryResult is the application-service output for paginated evaluation history.
type EvaluationHistoryResult struct {
	Items           []EmployeeEvaluation
	AverageRating   float64
	EvaluationCount int
	Pagination      PaginationResult
}
