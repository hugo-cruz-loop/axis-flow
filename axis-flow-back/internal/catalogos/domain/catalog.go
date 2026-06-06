// Package domain defines the core catalog entities and sentinel errors.
package domain

import (
	"errors"
	"time"
)

// Sentinel errors returned by repository implementations.
var (
	ErrNotFound      = errors.New("catalog: not found")
	ErrDuplicateCode = errors.New("catalog: code already exists")
	ErrHasDependents = errors.New("catalog: cannot delete — has dependents")
)

// Country represents catalogos.catalog_countries.
type Country struct {
	ID        int64
	Code      string // ISO 3166-1 alpha-2
	Name      string
	PhoneCode string
	DeletedAt *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

// State represents catalogos.catalog_states.
type State struct {
	ID        int64
	CountryID int64
	Code      string
	Name      string
	DeletedAt *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

// City represents catalogos.catalog_cities.
type City struct {
	ID        int64
	StateID   int64
	Name      string
	DeletedAt *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

// LocalityType represents catalogos.catalog_locality_types.
type LocalityType struct {
	ID        int64
	Code      string
	Name      string
	DeletedAt *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Bank represents catalogos.catalog_banks.
type Bank struct {
	ID        int64
	Code      string
	Name      string
	DeletedAt *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

// TaxRegime represents catalogos.catalog_tax_regimes.
type TaxRegime struct {
	ID            int64
	Code          string
	Name          string
	PersonaFisica bool
	PersonaMoral  bool
	DeletedAt     *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// PaymentForm represents catalogos.catalog_payment_forms.
type PaymentForm struct {
	ID        int64
	Code      string
	Name      string
	DeletedAt *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

// PaymentCondition represents catalogos.catalog_payment_conditions.
type PaymentCondition struct {
	ID        int64
	Code      string
	Name      string
	Days      int
	DeletedAt *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

// WorkflowStatus represents catalogos.catalog_workflow_statuses.
type WorkflowStatus struct {
	ID        int64
	Code      string
	Name      string
	RoleID    int16
	DeletedAt *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ComplaintType represents catalogos.catalog_complaint_types.
type ComplaintType struct {
	ID          int64
	Code        string
	Name        string
	Description string
	DeletedAt   *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Service represents catalogos.catalog_services.
type Service struct {
	ID          int64
	Code        string
	Name        string
	Description string
	Price       float64
	IsActive    bool
	DeletedAt   *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// SubscriptionPlan represents catalogos.catalog_subscription_plans.
type SubscriptionPlan struct {
	ID        int64
	Code      string
	Name      string
	Amount    float64
	DeletedAt *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

// DatePeriodicity represents catalogos.catalog_date_periodicities.
type DatePeriodicity struct {
	ID        int64
	Code      string
	Name      string
	DeletedAt *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

// HrAbsenceType represents catalogos.catalog_hr_absence_types.
type HrAbsenceType struct {
	ID                    int64
	Code                  string
	Name                  string
	RequiresJustification bool
	DeletedAt             *time.Time
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

// JobCategory represents catalogos.catalog_job_categories.
// Name is always lowercase (enforced by DB CHECK + service layer).
type JobCategory struct {
	ID        int64
	Code      string
	Name      string
	DeletedAt *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

// JobType represents catalogos.catalog_job_types.
// Name is always lowercase (enforced by DB CHECK + service layer).
type JobType struct {
	ID        int64
	Code      string
	Name      string
	DeletedAt *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}
