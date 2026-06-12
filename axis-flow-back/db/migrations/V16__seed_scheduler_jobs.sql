-- =============================================================
-- Seed: Scheduler Service — the four production cron jobs
--
-- File:          V16__seed_scheduler_jobs.sql
-- PR:            6 (Docker + Seed) of 16_Scheduler_Service_Spec
-- Companion:     V15__create_scheduler_schema.sql
--
-- Purpose
-- -------
-- Insert the canonical production cron jobs into
-- scheduler.scheduler_jobs. The scheduler binary (PR 4B)
-- re-resolves these rows at startup to fetch each job's UUID
-- primary key; the cron runner (PR 2B-I) uses the schedule
-- fields to register the robfig/cron/v3 entries. Without these
-- rows the runner registers the metadata but logs
-- "job row missing in database; runner will not schedule it
-- until it is seeded" for every job and never fires.
--
-- The fifth "sincronizacion_solicitada" job handled by the
-- SyncConsumer is intentionally NOT seeded here — it is
-- event-driven (Redis Streams / SQS) rather than cron-driven
-- and is wired separately in cmd/server/scheduler_wiring.go.
--
-- Prerequisites
-- --------------
--   1. V15__create_scheduler_schema.sql has been applied
--      (provides the scheduler.scheduler_jobs table and the
--      uq_scheduler_jobs_key_active partial unique index).
--
-- Idempotency
-- -----------
-- Re-running this script is safe. Each INSERT uses
--   ON CONFLICT (job_key) WHERE deleted_at IS NULL DO NOTHING
-- The conflict target is the V15 partial unique index
--   uq_scheduler_jobs_key_active ON scheduler.scheduler_jobs
--     (job_key) WHERE (deleted_at IS NULL)
-- which only enforces uniqueness among active (non-soft-deleted)
-- rows. Soft-deleted rows (deleted_at IS NOT NULL) keep their
-- job_key and the seed will NOT resurrect them — that is the
-- intended behavior because a soft-deleted job was retired
-- intentionally and a future re-seed should be a manual
-- operator decision.
--
-- Default values omitted from the column list
-- -------------------------------------------
--   * job_id         → gen_random_uuid() (V15 column default)
--   * is_active      → TRUE               (V15 column default)
--   * metadata       → seed JSONB payload (see below)
--   * last_run_at    → NULL               (no execution yet)
--   * next_run_at    → NULL               (cron computes on start)
--   * created_at     → CURRENT_TIMESTAMP  (V15 column default)
--   * updated_at     → CURRENT_TIMESTAMP  (V15 column default;
--                      V15 BEFORE UPDATE trigger refreshes
--                      this column on every UPDATE)
--
-- Module + JobClass convention
-- ----------------------------
--   module    = the bounded-context the job belongs to
--               ("notificaciones", "empleados", "asignacion",
--               "sync"). Used by the admin handler to group
--               jobs on the listing response.
--   job_class = the fully-qualified Go type that implements
--               the job. The cron runner reflects on this
--               string for diagnostic logging; the actual
--               handler is wired in scheduler_wiring.go, not
--               loaded dynamically.
-- =============================================================

SET search_path TO scheduler, public;

INSERT INTO scheduler.scheduler_jobs (
    job_key,
    cron_expression,
    job_class,
    module,
    description,
    is_active,
    metadata
) VALUES
    -- 1) Push notifications — every 5 minutes, looks 15 min ahead.
    (
        'notificaciones_en_tiempo_real',
        '*/5 * * * *',
        'jobs.NotificacionesEnTiempoRealJob',
        'notificaciones',
        'Alertas Push 15 min antes de entrada/comida/salida.',
        TRUE,
        '{"source": "v16_seed", "version": "1.0.0", "window_minutes": 15}'::jsonb
    ),
    -- 2) Auto-deactivation — daily 02:00, threshold fetched from
    --    the Parametrizacion service per company.
    (
        'inactiva_empleado',
        '0 2 * * *',
        'jobs.InactivaEmpleadoJob',
        'empleados',
        'Desactivación automática por inasistencias consecutivas (umbral parametrizado por empresa).',
        TRUE,
        '{"source": "v16_seed", "version": "1.0.0", "threshold_source": "parametrizacion"}'::jsonb
    ),
    -- 3) Session cleanup — daily 03:00, terminates expired rows
    --    in empleados_user_devices.
    (
        'cleanup_expired_sessions',
        '0 3 * * *',
        'jobs.CleanupExpiredSessionsJob',
        'empleados',
        'Cierra sesiones de dispositivos expiradas en empleados_user_devices.',
        TRUE,
        '{"source": "v16_seed", "version": "1.0.0"}'::jsonb
    ),
    -- 4) Lapsed assignment closure — daily 03:30, closes
    --    asignacion_asignacion rows whose fecha_fin is in the past.
    (
        'close_lapsed_assignments',
        '30 3 * * *',
        'jobs.CloseLapsedAssignmentsJob',
        'asignacion',
        'Cierra asignaciones cuya fecha_fin está en el pasado.',
        TRUE,
        '{"source": "v16_seed", "version": "1.0.0"}'::jsonb
    )
ON CONFLICT (job_key) WHERE deleted_at IS NULL DO NOTHING;

-- ─── Verification query ────────────────────────────────────────────────────
-- Run this to confirm the four rows are in place:
--
-- SELECT job_key, cron_expression, module, is_active
-- FROM   scheduler.scheduler_jobs
-- WHERE  deleted_at IS NULL
-- ORDER  BY
--    CASE job_key
--       WHEN 'notificaciones_en_tiempo_real' THEN 1
--       WHEN 'inactiva_empleado'              THEN 2
--       WHEN 'cleanup_expired_sessions'       THEN 3
--       WHEN 'close_lapsed_assignments'       THEN 4
--    END;
