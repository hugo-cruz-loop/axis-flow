// Package service_test — additional tests for EventoService.CancelEvento
// added in PR-5 (5.2a).
//
// The new method is consumed by the EventoCancelado Redis stream
// consumer (PR-5 5.2b). It implements the spec's "cancel any pending
// form executions and cleanup temporary resources" step. The minimum
// useful implementation is:
//
//   1. Mark the evento status as 'cancelado' (the iniciado rows are
//      left in their current state — they get a follow-up event
//      from the future Reporting consumer; the spec is silent on
//      cascading and PR-8 verify will exercise the full flow).
//   2. Invalidate the cached pendientes list for the (empresa, cliente)
//      pair (so the next list call shows the cancelled state).
//   3. NOT publish a new event from the service (the consumer
//      publishes its own follow-up if needed; the service is the
//      pure data-side action).
//
// IDOR: the evento must belong to the caller's empresa. Foreign
// tenants receive formularios.ErrNotFound (no leak).
package service_test

import (
	"context"
	"testing"
	"time"

	"axis-flow-back/internal/formularios"
	"axis-flow-back/internal/formularios/service"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEventoService_CancelEvento_MarksStatusAndInvalidates is the
// happy path: a known evento is cancelled, the status flips to
// 'cancelado', the cache for the (empresa, cliente) pair is
// invalidated, and the publisher does NOT receive any new event
// (the consumer is responsible for follow-up publishing).
func TestEventoService_CancelEvento_MarksStatusAndInvalidates(t *testing.T) {
	repo, pub, cache := newEventoFixture(t)
	svc := service.NewEventoService(repo, pub, cache)

	empresaID := uuid.New()
	clienteID := uuid.New()
	e := seedEvento(t, repo, empresaID, clienteID, uuid.New())

	require.NoError(t, svc.CancelEvento(context.Background(), e.ID, empresaID))

	// Status flipped to 'cancelado'.
	got, err := svc.GetEvento(context.Background(), e.ID, empresaID)
	require.NoError(t, err)
	assert.Equal(t, formularios.EventoStatusCancelado, got.Status, "evento status must flip to cancelado")

	// Cache: pendientes list invalidated for the (empresa, cliente) pair.
	require.Len(t, cache.eventoCalls, 1, "the pendientes list cache must be invalidated")
	assert.Equal(t, empresaID, cache.eventoCalls[0].Empresa)
	assert.Equal(t, clienteID, cache.eventoCalls[0].Cliente)

	// Publisher: CancelEvento does NOT publish (the consumer's job).
	assert.Empty(t, pub.events, "CancelEvento must not publish — the consumer publishes any follow-up")
}

// TestEventoService_CancelEvento_IDOR confirms that an evento in a
// foreign tenant returns ErrNotFound (no leak).
func TestEventoService_CancelEvento_IDOR(t *testing.T) {
	repo, pub, cache := newEventoFixture(t)
	svc := service.NewEventoService(repo, pub, cache)

	owner := uuid.New()
	intruder := uuid.New()
	e := seedEvento(t, repo, owner, uuid.New(), uuid.New())

	err := svc.CancelEvento(context.Background(), e.ID, intruder)
	require.ErrorIs(t, err, formularios.ErrNotFound, "IDOR violation must look like a not-found (no leak)")
	assert.Empty(t, cache.eventoCalls, "no cache invalidation on IDOR violation")
	assert.Empty(t, pub.events, "no publish on IDOR violation")
}

// TestEventoService_CancelEvento_UnknownID confirms that an unknown
// evento ID returns ErrNotFound.
func TestEventoService_CancelEvento_UnknownID(t *testing.T) {
	repo, pub, cache := newEventoFixture(t)
	svc := service.NewEventoService(repo, pub, cache)

	err := svc.CancelEvento(context.Background(), uuid.New(), uuid.New())
	require.ErrorIs(t, err, formularios.ErrNotFound)
	assert.Empty(t, cache.eventoCalls)
}

// TestEventoService_CancelEvento_AlreadyCancelled_Idempotent
// confirms that cancelling an already-cancelled evento is a no-op
// (not an error). The consumer is at-least-once and may redeliver.
// The service treats the second call as a true no-op: no status
// write, no cache invalidation (the status is already 'cancelado'
// and the cache was already busted by the first call).
func TestEventoService_CancelEvento_AlreadyCancelled_Idempotent(t *testing.T) {
	repo, pub, cache := newEventoFixture(t)
	svc := service.NewEventoService(repo, pub, cache)

	empresaID := uuid.New()
	clienteID := uuid.New()
	e := seedEvento(t, repo, empresaID, clienteID, uuid.New())

	// First cancel.
	require.NoError(t, svc.CancelEvento(context.Background(), e.ID, empresaID))
	// Second cancel is a true no-op (idempotent).
	require.NoError(t, svc.CancelEvento(context.Background(), e.ID, empresaID))

	got, err := svc.GetEvento(context.Background(), e.ID, empresaID)
	require.NoError(t, err)
	assert.Equal(t, formularios.EventoStatusCancelado, got.Status)

	// Cache was invalidated only once (the second call short-circuits
	// before the invalidation since the status is already 'cancelado').
	// The consumer dedupes downstream; the service is a true no-op
	// for the redundant call.
	assert.Len(t, cache.eventoCalls, 1, "second cancel must short-circuit before cache invalidation")
}

// TestEventoService_CancelEvento_RespectsTenantInCacheKey
// documents the cache-key boundary: cancelling one tenant's evento
// must NOT invalidate another tenant's cache entry.
func TestEventoService_CancelEvento_RespectsTenantInCacheKey(t *testing.T) {
	repo, pub, cache := newEventoFixture(t)
	svc := service.NewEventoService(repo, pub, cache)

	empresaA := uuid.New()
	clienteA := uuid.New()
	eA := seedEvento(t, repo, empresaA, clienteA, uuid.New())

	empresaB := uuid.New()
	_ = empresaB // unused — only empresaA's evento is cancelled

	require.NoError(t, svc.CancelEvento(context.Background(), eA.ID, empresaA))

	require.Len(t, cache.eventoCalls, 1)
	assert.Equal(t, empresaA, cache.eventoCalls[0].Empresa, "cache invalidation must use the cancelled evento's empresa, not the caller's")
}

// time is referenced here so the test file does not fail to compile
// if a future test needs a time helper.
var _ = time.Now
