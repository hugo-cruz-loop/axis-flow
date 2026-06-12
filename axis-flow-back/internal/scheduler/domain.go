// Package scheduler provides background job orchestration for the axis-flow
// platform. The package owns the scheduler_jobs and scheduler_executions
// persistence under the scheduler schema and exposes domain contracts that
// other modules can depend on.
package scheduler

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
)

// ---------------------------------------------------------------------------
// ExecutionStatus — lifecycle status of a scheduled job execution.
// ---------------------------------------------------------------------------

// ExecutionStatus mirrors the ck_scheduler_executions_status CHECK constraint.
type ExecutionStatus string

const (
	// StatusRunning indicates the execution has started but not yet finalized.
	StatusRunning ExecutionStatus = "RUNNING"
	// StatusSuccess indicates the execution completed without error.
	StatusSuccess ExecutionStatus = "SUCCESS"
	// StatusFailed indicates the execution terminated with an error.
	StatusFailed ExecutionStatus = "FAILED"
)

// Valid reports whether the status is one of the supported lifecycle values.
func (s ExecutionStatus) Valid() bool {
	switch s {
	case StatusRunning, StatusSuccess, StatusFailed:
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// Job — the registered scheduled job metadata.
// ---------------------------------------------------------------------------

// Job mirrors the scheduler.scheduler_jobs table exactly. Cron and interval
// schedules are mutually exclusive: exactly one of CronExpression and
// IntervalSeconds must be set (see the ck_scheduler_jobs_schedule check).
type Job struct {
	JobID           uuid.UUID
	JobKey          string
	CronExpression  *string
	IntervalSeconds *int
	JobClass        string
	Module          string
	Description     *string
	IsActive        bool
	Metadata        json.RawMessage
	LastRunAt       *time.Time
	NextRunAt       *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
	DeletedAt       *time.Time
}

// ScheduleKind reports which schedule mechanism the job uses.
func (j Job) ScheduleKind() string {
	switch {
	case j.CronExpression != nil:
		return "cron"
	case j.IntervalSeconds != nil:
		return "interval"
	default:
		return "none"
	}
}

// ValidateSchedule enforces the cron XOR interval constraint required by the
// ck_scheduler_jobs_schedule CHECK constraint and the
// ck_scheduler_jobs_interval CHECK constraint. It is safe to call on a Job
// before persistence and returns the first violation it finds.
func (j Job) ValidateSchedule() error {
	hasCron := j.CronExpression != nil && *j.CronExpression != ""
	hasInterval := j.IntervalSeconds != nil
	switch {
	case hasCron && hasInterval:
		return fmt.Errorf("%w: cron_expression and interval_seconds are mutually exclusive", ErrInvalidInput)
	case !hasCron && !hasInterval:
		return fmt.Errorf("%w: either cron_expression or interval_seconds must be set", ErrInvalidInput)
	}
	if hasInterval && *j.IntervalSeconds <= 0 {
		return fmt.Errorf("%w: interval_seconds must be > 0", ErrInvalidInput)
	}
	if hasCron {
		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		if _, err := parser.Parse(*j.CronExpression); err != nil {
			return fmt.Errorf("%w: invalid cron_expression: %v", ErrInvalidInput, err)
		}
	}
	return nil
}

// Validate enforces the minimum required fields for a Job record. It runs
// ValidateSchedule internally and additionally checks module, job_class and
// job_key for presence.
func (j Job) Validate() error {
	if j.JobKey == "" {
		return fmt.Errorf("%w: job_key is required", ErrInvalidInput)
	}
	if j.JobClass == "" {
		return fmt.Errorf("%w: job_class is required", ErrInvalidInput)
	}
	if j.Module == "" {
		return fmt.Errorf("%w: module is required", ErrInvalidInput)
	}
	return j.ValidateSchedule()
}

// ---------------------------------------------------------------------------
// Execution — a single run of a scheduled job.
// ---------------------------------------------------------------------------

// Execution mirrors the scheduler.scheduler_executions table. DurationSeconds
// is generated server-side from started_at / ended_at.
type Execution struct {
	ExecutionID       uuid.UUID
	JobID             uuid.UUID
	Status            ExecutionStatus
	StartedAt         time.Time
	EndedAt           *time.Time
	ErrorLog          *string
	ExecutionMetadata json.RawMessage
	CreatedAt         time.Time
	// DurationSeconds is populated by PostgreSQL via the generated column.
	// It is non-nil only after a successful round-trip with the database.
	DurationSeconds *float64
}

// Validate enforces the minimum required fields for an Execution record.
func (e Execution) Validate() error {
	if e.JobID == uuid.Nil {
		return fmt.Errorf("%w: job_id is required", ErrInvalidInput)
	}
	if !e.Status.Valid() {
		return fmt.Errorf("%w: status must be RUNNING, SUCCESS or FAILED", ErrInvalidInput)
	}
	if e.StartedAt.IsZero() {
		return fmt.Errorf("%w: started_at is required", ErrInvalidInput)
	}
	return nil
}

// ---------------------------------------------------------------------------
// JobListFilter — list filters shared with the repository layer.
// ---------------------------------------------------------------------------

// JobListFilter narrows down a repository List call. The zero value applies
// no filter and returns all jobs (active and paused) ordered by job_key.
type JobListFilter struct {
	Module   *string
	IsActive *bool
	Limit    int
	Offset   int
}

// ExecutionListFilter narrows down a repository List call for executions.
type ExecutionListFilter struct {
	JobID     *uuid.UUID
	Status    *ExecutionStatus
	StartDate *time.Time
	EndDate   *time.Time
	Limit     int
	Offset    int
}

// ---------------------------------------------------------------------------
// Sentinel errors.
// ---------------------------------------------------------------------------

var (
	// ErrNotFound is returned when a job or execution is not present.
	ErrNotFound = errors.New("scheduler: not found")
	// ErrInvalidInput is returned when domain validation fails.
	ErrInvalidInput = errors.New("scheduler: invalid input")
	// ErrConflict is returned on unique-key collisions or other DB-level
	// conflicts the caller should treat as retryable business errors.
	ErrConflict = errors.New("scheduler: conflict")
)
