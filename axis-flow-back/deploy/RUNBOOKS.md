# Scheduler Service — Operational Runbooks

> **Audience**: on-call operators running the Scheduler Service
> in production (VPS + Docker Compose). For full design
> context, see
> [`docs/services/16_Scheduler_Service_Spec/specification.md`](../../../docs/services/16_Scheduler_Service_Spec/specification.md).
>
> **What this file is**: a one-screen pointer to the three
> runbooks the spec mandates, plus a quick-reference card for
> the four cron jobs, the health endpoints, and the manual
> trigger flow. The full symptom + resolution text for each
> runbook lives in the spec — this file links to it and
> restates the resolution steps so an operator does not have
> to context-switch under pressure.

---

## Quick Reference

### The four production jobs

| `job_key`                     | Cron           | Module         | Runbook coverage |
|-------------------------------|----------------|----------------|------------------|
| `notificaciones_en_tiempo_real` | `*/5 * * * *`  | notificaciones | Runbook 3 (manual trigger) |
| `inactiva_empleado`           | `0 2 * * *`    | empleados      | Runbooks 1, 2, 3 |
| `cleanup_expired_sessions`    | `0 3 * * *`    | empleados      | Runbook 3 |
| `close_lapsed_assignments`    | `30 3 * * *`   | asignacion     | Runbook 3 |

The fifth job (`sincronizacion_solicitada`) is event-driven
(Redis Streams / SQS); it is NOT cron-scheduled and is not
listed above. It is wired separately in
`cmd/server/scheduler_wiring.go`.

### Health endpoints

| Endpoint           | Purpose                                              | Auth |
|--------------------|------------------------------------------------------|------|
| `GET /health/live`   | Liveness — returns 200 if the process is up.        | none |
| `GET /health/ready`  | Readiness — pings PostgreSQL + Redis; 503 on fail.  | none |
| `GET /metrics`       | Prometheus scrape (gated by `METRICS_ENABLED`).     | none |

The Docker image is `gcr.io/distroless/static-debian12:nonroot`
and has no shell — the Dockerfile deliberately omits the
`HEALTHCHECK` directive (see the header comment in
`scheduler.Dockerfile` for the full rationale). Drive
readiness from a sidecar or the host network.

### Triggering a job manually

When a cron job has failed and the operator needs to force
a re-run without waiting for the next tick:

```bash
curl -X POST \
  -H "Authorization: Bearer <ADMIN_JWT>" \
  http://<scheduler-host>:8080/api/v1/scheduler/jobs/<job_key>/trigger
```

The endpoint returns `202 Accepted` and records a new row in
`scheduler.scheduler_executions` with `status = 'RUNNING'`.
The per-job Redis distributed lock (`scheduler:lock:<job_key>`)
still applies, so a manual trigger on a multi-replica cluster
will only run on the replica that wins the lock.

### Container lifecycle

```bash
# Rolling update after a secret rotation or image bump.
docker compose -f docker-compose.yml \
               -f axis-flow-back/deploy/scheduler.docker-compose.yml \
  up -d --force-recreate scheduler-service

# Tail the structured logfmt output.
docker logs -f axis-flow-scheduler

# Inspect the running container's resource footprint.
docker stats axis-flow-scheduler
```

---

## Runbook 1 — Releasing Stuck Redis Locks

**Spec section**: `docs/services/16_Scheduler_Service_Spec/specification.md` → `Runbook 1`.

### Symptom
A specific scheduled job (e.g. `inactiva_empleado`) fails to
trigger during its scheduled time, and logs report:
`msg="Job execution skipped due to locking" job_id=inactiva_empleado`.

### Resolution

1. Connect to the Redis instance:
   ```bash
   redis-cli -u $REDIS_URL
   ```
2. Check if the lock key exists:
   ```redis
   EXISTS scheduler:lock:inactiva_empleado
   ```
3. Query the remaining TTL (Time to Live):
   ```redis
   TTL scheduler:lock:inactiva_empleado
   ```
4. If the TTL is stuck or the lock must be forced open
   immediately, delete the key:
   ```redis
   DEL scheduler:lock:inactiva_empleado
   ```

After the lock is cleared the next cron tick on any replica
will acquire it normally and execute the job.

---

## Runbook 2 — Firebase Credentials Rotation

**Spec section**: `docs/services/16_Scheduler_Service_Spec/specification.md` → `Runbook 2`.

### Symptom
Push notification logs report:
`msg="Push delivery failed" error="401 Unauthorized" provider=firebase`.

### Resolution

1. Navigate to the **Firebase Console → Project Settings → Service Accounts**.
2. Click **Generate New Private Key** to download the new
   private key JSON file.
3. Compress or format the JSON payload as a clean single-line
   string. Example of the file content the operator must
   produce (single line, single-quoted so Compose preserves
   the inner JSON quoting):

   ```
   FIREBASE_CREDENTIALS_JSON='{"type":"service_account","project_id":"…",…}'
   ```
4. Update the operator-managed env file at
   `axis-flow-back/deploy/secrets/firebase.creds.env` with
   the new value of `FIREBASE_CREDENTIALS_JSON`. **Never
   commit the file** — confirm `secrets/` is in the project
   root `.gitignore` (it is, per the spec section
   "Seguridad > Firebase Credentials Protection").
5. Trigger a rolling update of the scheduler replicas so the
   new env var is picked up by every container:
   ```bash
   docker compose -f docker-compose.yml \
                  -f axis-flow-back/deploy/scheduler.docker-compose.yml \
     up -d --force-recreate scheduler-service
   ```

The binary reads `FIREBASE_CREDENTIALS_JSON` at process start
inside `cmd/server/scheduler_wiring.go` (the FCM client is
constructed once, not refreshed at runtime). A container
restart is therefore required to pick up the new credentials.

---

## Runbook 3 — Recovering from Failed Job Executions

**Spec section**: `docs/services/16_Scheduler_Service_Spec/specification.md` → `Runbook 3`.

### Symptom
Critical daily maintenance jobs fail (e.g. `inactiva_empleado`
failed due to database locks or network issues) and the
execution history table records a `FAILED` status.

### Resolution

1. Check the failed executions list in PostgreSQL:
   ```sql
   SELECT * FROM scheduler.scheduler_executions
   WHERE  status = 'FAILED'
   ORDER  BY started_at DESC
   LIMIT  10;
   ```
2. Trigger the job manually using the HTTP admin API route
   (Bearer JWT required):
   ```bash
   curl -X POST \
     -H "Authorization: Bearer <ADMIN_TOKEN>" \
     http://localhost:8080/api/v1/scheduler/jobs/inactiva_empleado/trigger
   ```
3. Validate that the execution is recorded successfully as a
   new row in `scheduler.scheduler_executions`:
   ```sql
   SELECT status, started_at, ended_at, duration_seconds, error_log
   FROM   scheduler.scheduler_executions
   WHERE  job_id = (
            SELECT job_id FROM scheduler.scheduler_jobs
            WHERE  job_key = 'inactiva_empleado'
          )
   ORDER  BY started_at DESC
   LIMIT  5;
   ```

A `SUCCESS` status with a non-null `ended_at` confirms the
recovery. If the manual trigger also fails, escalate to the
`inactiva_empleado` job's code path
(`axis-flow-back/internal/scheduler/jobs/inactiva_empleado.go`)
and the parametrizacion client wiring
(`axis-flow-back/internal/scheduler/service/parametrizacion_client.go`).
