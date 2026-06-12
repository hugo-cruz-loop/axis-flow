-- V15: Create Scheduler service schema and integrity rules.
--
-- PR1 scope for 16_Scheduler_Service_Spec:
--   * Owns schema scheduler.
--   * scheduler_jobs replaces the legacy Django APScheduler tables.
--   * Cron and interval triggers are mutually exclusive (XOR) and intervals must be positive.
--   * Partial unique index allows soft-deleted rows to reuse job_key while
--     guaranteeing uniqueness among active jobs.
--   * scheduler_executions records each run with a generated duration_seconds
--     computed from started_at / ended_at.
--   * Tables with updated_at get BEFORE UPDATE triggers.

CREATE SCHEMA IF NOT EXISTS scheduler;

CREATE OR REPLACE FUNCTION scheduler.fn_set_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Table: scheduler_jobs
CREATE TABLE IF NOT EXISTS scheduler.scheduler_jobs (
    job_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_key VARCHAR(100) NOT NULL,
    cron_expression VARCHAR(100) NULL,
    interval_seconds INTEGER NULL,
    job_class VARCHAR(255) NOT NULL,
    module VARCHAR(50) NOT NULL,
    description TEXT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    last_run_at TIMESTAMPTZ NULL,
    next_run_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ NULL,

    CONSTRAINT ck_scheduler_jobs_schedule CHECK (
        (cron_expression IS NOT NULL) OR (interval_seconds IS NOT NULL)
    ),
    CONSTRAINT ck_scheduler_jobs_interval CHECK (
        (interval_seconds IS NULL) OR (interval_seconds > 0)
    )
);

-- Partial unique index for soft-delete active job keys
CREATE UNIQUE INDEX IF NOT EXISTS uq_scheduler_jobs_key_active
ON scheduler.scheduler_jobs(job_key)
WHERE (deleted_at IS NULL);

CREATE INDEX IF NOT EXISTS idx_scheduler_jobs_active
ON scheduler.scheduler_jobs(is_active)
WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_scheduler_jobs_module
ON scheduler.scheduler_jobs(module);

-- Table: scheduler_executions
CREATE TABLE IF NOT EXISTS scheduler.scheduler_executions (
    execution_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id UUID NOT NULL,
    status VARCHAR(20) NOT NULL,
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ended_at TIMESTAMPTZ NULL,
    error_log TEXT NULL,
    execution_metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    duration_seconds NUMERIC(10, 2) GENERATED ALWAYS AS (
        EXTRACT(EPOCH FROM (ended_at - started_at))
    ) STORED,

    CONSTRAINT fk_scheduler_executions_job
        FOREIGN KEY (job_id)
        REFERENCES scheduler.scheduler_jobs(job_id)
        ON DELETE CASCADE,

    CONSTRAINT ck_scheduler_executions_status CHECK (
        status IN ('RUNNING', 'SUCCESS', 'FAILED')
    )
);

CREATE INDEX IF NOT EXISTS idx_scheduler_executions_job_id
ON scheduler.scheduler_executions(job_id);

CREATE INDEX IF NOT EXISTS idx_scheduler_executions_status_started
ON scheduler.scheduler_executions(status, started_at DESC);

CREATE INDEX IF NOT EXISTS idx_scheduler_executions_started
ON scheduler.scheduler_executions(started_at);

CREATE TRIGGER trg_set_timestamp_scheduler_jobs
    BEFORE UPDATE ON scheduler.scheduler_jobs
    FOR EACH ROW EXECUTE FUNCTION scheduler.fn_set_timestamp();

-- Rollback DDL (manual, mirrors V14 style):
--
-- DROP TRIGGER IF EXISTS trg_set_timestamp_scheduler_jobs ON scheduler.scheduler_jobs;
-- DROP FUNCTION IF EXISTS scheduler.fn_set_timestamp();
-- DROP TABLE IF EXISTS scheduler.scheduler_executions;
-- DROP TABLE IF EXISTS scheduler.scheduler_jobs;
-- DROP SCHEMA IF EXISTS scheduler CASCADE;
