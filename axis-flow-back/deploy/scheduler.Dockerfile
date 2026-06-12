# syntax=docker/dockerfile:1.7
# =============================================================
# Scheduler Service — production image
#
# PR:         6 (Docker + Seed) of 16_Scheduler_Service_Spec
# Spec:       docs/services/16_Scheduler_Service_Spec/specification.md
#             — section "Configuración > Perfil de Recursos"
#               (0.5 vCPU, 128 MiB RAM)
#             — section "Configuración > Variables de Entorno"
#               (DATABASE_URL, REDIS_URL, FIREBASE_CREDENTIALS_JSON,
#                PARAMETRIZACION_SERVICE_URL, SQS_QUEUE_URL,
#                AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY,
#                AWS_REGION, LOG_FORMAT, SCHEDULER_LOCK_TTL_SEC)
#
# Build context: parent of this Dockerfile (the axis-flow-back
# module root). The build invocation expects the context to be
# axis-flow-back/, e.g.
#
#   docker build \
#     -f axis-flow-back/deploy/scheduler.Dockerfile \
#     -t axis-flow-back:scheduler \
#     axis-flow-back
#
# Multi-stage layout
# ------------------
#   1. builder  — golang:1.25-alpine. Compiles the unified
#                axis-flow-back binary at ./cmd/server. The
#                scheduler runtime is mounted into the same
#                HTTP listener as the rest of the platform
#                (see cmd/server/main.go: wireScheduler).
#                CGO is disabled; the output is a static
#                binary linked against libc-free alpine.
#   2. runtime — gcr.io/distroless/static-debian12:nonroot.
#                No shell, no package manager, ~2 MB base
#                image. The nonroot uid:gid (65532) is the
#                canonical unprivileged identity used by
#                distroless and matches Docker's default
#                "nonroot" user expectation.
#
# Resource profile
# ----------------
# The compose service that uses this image pins
#   deploy.resources.limits      = 0.5 vCPU / 128 MiB
#   deploy.resources.reservations = 0.1 vCPU /  64 MiB
# per the spec. The distroless static base + a Go binary with
# -ldflags="-s -w" fits comfortably inside the 128 MiB cap.
#
# Healthcheck
# -----------
# This Dockerfile OMITS the HEALTHCHECK directive. The reasons:
#   * The distroless/static image has no shell (no `wget`, no
#     `curl`, no `/bin/sh`); the canonical
#     `HEALTHCHECK CMD wget --spider ...` pattern is not
#     available without bloating the image.
#   * The binary does not currently expose a `--healthcheck`
#     subcommand; adding one is intentionally out of scope
#     for PR 6 (would be a production code change).
#   * Orchestrator (Docker / Docker Compose / any reverse
#     proxy) should drive readiness off the HTTP endpoints
#     already exposed on the binary: `/health/live` (liveness)
#     and `/health/ready` (readiness — pings PostgreSQL and
#     Redis). The compose service therefore uses
#     `healthcheck:` with `test: ["CMD", ...]` if/when the
#     runner supports HTTP health probes natively.
# The choice is documented in the spec's Runbook 3 and
# reflected in axis-flow-back/deploy/scheduler.docker-compose.yml.
# =============================================================

# ─── Stage 1: builder ──────────────────────────────────────────────────────
FROM golang:1.25-alpine AS builder

# git is required when go.sum references modules that are not in
# the proxy cache (e.g. private replace directives). The
# scheduler service has none today, but pinning the tool here
# future-proofs the build against a future dependency being
# pulled from a private git server.
# ca-certificates is required for the FCM / AWS / parametrizacion
# HTTPS clients at build time (when go mod download fetches
# from proxy.golang.org it is HTTPS) and at runtime (the same
# certs are inherited by the runtime stage via COPY --from).
RUN apk add --no-cache git ca-certificates

WORKDIR /src

# Copy module manifests first to leverage Docker's layer cache:
# as long as go.mod / go.sum are unchanged, the expensive
# `go mod download` step is reused across rebuilds.
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source. The scheduler service is built
# into the same binary as the rest of the platform
# (./cmd/server/main.go) — the scheduler runtime is mounted on
# the same chi router via wireScheduler. Building a separate
# `cmd/scheduler` binary is out of scope for PR 6.
COPY . .

# Build a static, stripped, release-mode binary.
#   CGO_ENABLED=0   — no glibc / musl coupling; matches
#                     distroless static (no libc available).
#   GOOS=linux      — the build host may be macOS or Windows.
#   -trimpath       — strips local filesystem paths from the
#                     binary for reproducible builds.
#   -ldflags "-s -w" — strips the symbol table and DWARF
#                     debugging info to shrink the binary.
RUN CGO_ENABLED=0 GOOS=linux go build \
        -trimpath \
        -ldflags="-s -w" \
        -o /out/axis-flow-back \
        ./cmd/server

# ─── Stage 2: runtime ──────────────────────────────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot AS runtime

# distroless/static:nonroot ships with a single unprivileged
# user (uid 65532, gid 65532, named "nonroot") and a
# /etc/passwd entry — USER nonroot:nonroot works out of the box.
# ca-certificates are copied from the builder so the runtime
# can dial HTTPS endpoints (FCM, AWS SQS, parametrizacion
# service) without TLS handshake failures.
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /out/axis-flow-back /usr/local/bin/axis-flow-back

USER nonroot:nonroot

# Default port: 8080 (matches axis-flow-back/internal/config/config.go
# which returns "8080" when SERVER_PORT is unset). The compose
# service maps 8080:8080; the binary itself can be moved with
# SERVER_PORT at runtime — no rebuild required.
EXPOSE 8080

# The binary is the entrypoint. The scheduler runtime starts
# inside main.go's wireScheduler call; the cron runner, the
# SQS/Streams consumer, the health server, the admin REST
# surface and the Prometheus /metrics endpoint all come up
# together. See cmd/server/main.go and cmd/server/scheduler_wiring.go.
ENTRYPOINT ["/usr/local/bin/axis-flow-back"]
