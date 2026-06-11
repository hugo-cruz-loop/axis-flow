// Package service implements use-case logic for the Formularios module.
//
// Layout (PR-3 — Services):
//   - ports.go            : CacheInvalidator + Locker ports (no concrete deps)
//   - hooks.go            : postWriteHook helper (fire-and-forget publish +
//                           cache invalidation; added in task 3.5)
//   - formulario_service  : Formulario CRUD + pregunta add (Gherkin 1)
//   - evento_service      : Evento + iniciado check-in (Gherkin 2)
//   - respuesta_service   : Respuesta capture (Gherkin 2)
//   - pdf_service         : PDF report render + upload (Gherkin 3)
//
// Mirrors the structure of internal/atencionseguimiento/service/ (PR-3 of
// 09_AtencionSeguimiento_Service_Spec): an interface per service, an
// unexported concrete struct, and a constructor that takes the repository
// port + an EventPublisher. This module additionally takes a CacheInvalidator
// so the service can fire UNLINK on the cached list keys after a mutation
// that would otherwise serve stale data.
package service

import (
	"context"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// CacheInvalidator — narrow port satisfied by the Pgx repo cache layer.
//
// The concrete *repository.RedisFormulariosCacheInvalidator (PR-2) satisfies
// this interface; a compile-time assertion lives in ports_test.go. The
// service layer is the right place to issue the invalidation: the Pgx repo
// persists, the service orchestrates publish + cache busting, and the
// Pgx repo's own (currently unused) cache field is kept for the cases where
// a repo method needs to invalidate without going through the service.
// ---------------------------------------------------------------------------

// CacheInvalidator removes cached list entries after a mutation that would
// otherwise serve stale data. Implementations are expected to be
// non-blocking (UNLINK, not DEL) so the caller's transaction is not held
// up by Redis latency.
type CacheInvalidator interface {
	// UnlinkFormulariosByEmpresa invalidates the cached formularios list
	// for a given empresa.
	UnlinkFormulariosByEmpresa(ctx context.Context, empresaID uuid.UUID) error

	// UnlinkEventosByEmpCte invalidates the cached eventos list for a
	// given (empresa, cliente) pair.
	UnlinkEventosByEmpCte(ctx context.Context, empresaID, clienteID uuid.UUID) error

	// UnlinkRespuestasByIniciado invalidates the cached respuestas list
	// for a given check-in.
	UnlinkRespuestasByIniciado(ctx context.Context, iniciadoID uuid.UUID) error
}

// ---------------------------------------------------------------------------
// Locker — distributed lock + pending-set tracking port for the PDF service.
//
// Real implementation lands in PR-6 (PDF/S3 Hardening) — for PR-3 the
// service uses a stub so the Gherkin-3 path is exercised end-to-end without
// a real Redis lock client.
//
// The 09 module does not have an equivalent port (the PDF flow is
// formularios-specific), so this interface is NOT a 1:1 mirror.
// ---------------------------------------------------------------------------

// Locker acquires short-lived distributed locks and tracks pending work in
// a Redis set. The PDF service uses it to deduplicate concurrent PDF
// generation jobs for the same check-in (KeyReporteS3Lock) and to keep a
// "pending" set (KeyReportePending) for observability.
//
// PR-6 (6.3): the port signature changed to take uuid.UUID + duration
// instead of string + int. The previous string-based signature was a
// leak from the locker's internal Redis key shape; the new signature
// makes the locker responsible for building the canonical
// "formularios:reporte:lock:<uuid>" key. The previous Locker port
// was satisfied by a test stub in pdf_service_test.go; that stub is
// updated in PR-6 to match the new shape.
type Locker interface {
	// Acquire takes a Redis SET-NX-with-TTL lock on the
	// "formularios:reporte:lock:<iniciadoID>" key. Returns an
	// UnlockFn that releases the lock; the caller MUST invoke it
	// via defer. Returns formularios.ErrConflict if the lock is
	// held by another process.
	Acquire(ctx context.Context, iniciadoID uuid.UUID) (UnlockFn, error)

	// AddToPending adds the iniciadoID to the pending report set
	// (KeyReportePending).
	AddToPending(ctx context.Context, iniciadoID uuid.UUID) error

	// RemoveFromPending removes the iniciadoID from the pending
	// report set. Safe to call even if the member is not
	// present.
	RemoveFromPending(ctx context.Context, iniciadoID uuid.UUID) error
}

// UnlockFn releases a distributed lock. Safe to call multiple times; the
// implementation is expected to no-op on subsequent calls.
type UnlockFn func(ctx context.Context) error
