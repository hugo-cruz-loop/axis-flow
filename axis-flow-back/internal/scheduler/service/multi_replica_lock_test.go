// Package service_test — multi_replica_lock_test.go covers Gherkin
// scenario 3 from the Scheduler Service specification: two
// scheduler replicas ("Réplica A" and "Réplica B") try to run
// "inactiva_empleado" simultaneously; only the SET NX winner runs,
// the loser aborts and increments
// scheduler_redis_lock_failures_total. PR 5C (task 5.6) drives this
// end-to-end against an in-process miniredis with two independent
// *redis.Client instances mimicking the two replicas.
//
// Three scenarios are covered:
//  1. Only-one-wins-the-lock — SET NX, loser increments the
//     counter, winner releases, loser takes over on retry.
//  2. Wrong-owner release does NOT evict — the spec's Lua release
//     contract (GET == value → DEL) keeps the real owner's lease
//     alive when a non-owner calls Release.
//  3. TTL expiry frees the lock — proves the SET PX fallback when
//     the winner never releases (crash mid-run).
package service_test

import (
	"context"
	"testing"
	"time"

	"axis-flow-back/internal/scheduler/service"

	"github.com/alicebob/miniredis/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// testJobKey is the job_key under test. The Gherkin 3 scenario
	// fixes it to inactiva_empleado.
	testJobKey = "inactiva_empleado"
	// testLockKey is the resulting Redis key. The lock manager
	// assembles it as "scheduler:lock:" + jobKey — we hard-code it
	// here so miniredis.Exists assertions do not duplicate that
	// contract.
	testLockKey = "scheduler:lock:inactiva_empleado"
	// testLockTTL is the spec's documented 10-minute TTL.
	testLockTTL = 10 * time.Minute
	// replicaExecA / replicaExecB are the per-replica execution
	// tokens used as the SET value. In production these are
	// UUIDs minted by runLockGated; for the test any distinct
	// non-empty strings satisfy the contract.
	replicaExecA = "exec-A"
	replicaExecB = "exec-B"
)

// newMultiReplicaTestHarness spins up a single in-process miniredis
// (the "shared Redis deployment"), builds two independent
// *redis.Client instances pointed at it (Replica A and Replica B),
// and returns the two LockManager instances plus a *SchedulerMetrics
// registered against an isolated Prometheus registry. Tests can
// inspect the miniredis directly via the returned handle to assert
// key existence or call FastForward to advance the simulated clock.
func newMultiReplicaTestHarness(t *testing.T) (
	lockA, lockB service.LockManager,
	metrics *service.SchedulerMetrics,
	redisSrv *miniredis.Miniredis,
) {
	t.Helper()
	redisSrv = miniredis.RunT(t)
	clientA := redis.NewClient(&redis.Options{Addr: redisSrv.Addr()})
	clientB := redis.NewClient(&redis.Options{Addr: redisSrv.Addr()})
	t.Cleanup(func() {
		_ = clientA.Close()
		_ = clientB.Close()
	})
	registry := prometheus.NewRegistry()
	m, err := service.RegisterMetrics(registry)
	require.NoError(t, err, "RegisterMetrics against a fresh registry must succeed")
	return service.NewRedisLockManager(clientA),
		service.NewRedisLockManager(clientB),
		m,
		redisSrv
}

// TestMultiReplica_OnlyOneWinsTheLock is the headline Gherkin 3
// scenario: when two replicas try to acquire the same lock key
// simultaneously, exactly one wins. The winner runs (here, just
// releases); the loser observes acquired=false, the cron runner
// would increment scheduler_redis_lock_failures_total, and once
// the winner releases, the loser can take over on the next tick.
func TestMultiReplica_OnlyOneWinsTheLock(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	lockA, lockB, metrics, s := newMultiReplicaTestHarness(t)

	require.Equal(t, 0.0,
		testutil.ToFloat64(metrics.RedisLockFailures.WithLabelValues(testJobKey)),
		"RedisLockFailures counter must start at 0")

	// Replica A wins the SET NX on a fresh key.
	acquired, err := lockA.Acquire(ctx, testJobKey, replicaExecA, testLockTTL)
	require.NoError(t, err)
	require.True(t, acquired, "Replica A must win the SET NX on a fresh key")
	assert.True(t, s.Exists(testLockKey), "lock key must exist after Replica A acquires")

	// Replica B tries simultaneously — collision. SET NX returns
	// nil on a held key, which the contract surfaces as
	// acquired=false with a nil error (business condition, not a
	// transport failure).
	acquired, err = lockB.Acquire(ctx, testJobKey, replicaExecB, testLockTTL)
	require.NoError(t, err, "collision is a business condition, not a transport error")
	require.False(t, acquired, "Replica B must NOT win the lock while Replica A holds it")

	// The cron runner's runLockGated closure calls
	// IncRedisLockFailure exactly here, in production.
	metrics.IncRedisLockFailure(testJobKey)
	assert.Equal(t, 1.0,
		testutil.ToFloat64(metrics.RedisLockFailures.WithLabelValues(testJobKey)),
		"scheduler_redis_lock_failures_total{job_id=inactiva_empleado} must be 1 after one collision")

	// Replica A finishes its (mock) job and releases the lock.
	require.NoError(t, lockA.Release(ctx, testJobKey, replicaExecA))
	assert.False(t, s.Exists(testLockKey), "release from real owner must delete the key")

	// Replica B retries after release — it should now succeed.
	acquired, err = lockB.Acquire(ctx, testJobKey, replicaExecB, testLockTTL)
	require.NoError(t, err)
	require.True(t, acquired, "Replica B must take over once Replica A has released")
	require.NoError(t, lockB.Release(ctx, testJobKey, replicaExecB))
}

// TestMultiReplica_WrongOwnerReleaseDoesNotEvict proves the Lua
// release contract from spec §"Distributed Locking Strategy": a
// Release call from a replica that did not own the lock is a no-op
// — the key survives and the real owner can still renew / release.
// This is the "prevent race condition evictions" guarantee the spec
// calls out explicitly.
func TestMultiReplica_WrongOwnerReleaseDoesNotEvict(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	lockA, lockB, _, s := newMultiReplicaTestHarness(t)

	// Replica A acquires.
	acquired, err := lockA.Acquire(ctx, testJobKey, replicaExecA, testLockTTL)
	require.NoError(t, err)
	require.True(t, acquired)

	// Replica B (a different replica) calls Release with the wrong
	// execution_id. The Lua script must NOT delete the key, and the
	// manager must surface ErrLockLost so the caller can log it.
	err = lockB.Release(ctx, testJobKey, replicaExecB)
	require.Error(t, err, "wrong-owner release must surface ErrLockLost")
	assert.ErrorIs(t, err, service.ErrLockLost)
	assert.True(t, s.Exists(testLockKey), "wrong-owner release must NOT evict the key")

	// The real owner (Replica A) can still renew the heartbeat —
	// the value is intact and the Lua script's GET check passes.
	renewed, err := lockA.RenewHeartbeat(ctx, testJobKey, replicaExecA, testLockTTL)
	require.NoError(t, err)
	require.True(t, renewed, "real owner must still be able to renew the heartbeat")

	// Replica A's real release evicts the key.
	require.NoError(t, lockA.Release(ctx, testJobKey, replicaExecA))
	assert.False(t, s.Exists(testLockKey), "real owner release must delete the key")
}

// TestMultiReplica_TTLExpiryFreesLock proves the SET PX TTL
// fallback: if Replica A acquires and never releases (e.g. it
// crashes mid-run), the lock eventually expires and a competing
// replica can take over. miniredis' FastForward advances the
// simulated clock past the TTL, avoiding any real-time sleep.
func TestMultiReplica_TTLExpiryFreesLock(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	lockA, lockB, _, s := newMultiReplicaTestHarness(t)

	// Replica A acquires with a 100ms TTL.
	acquired, err := lockA.Acquire(ctx, testJobKey, replicaExecA, 100*time.Millisecond)
	require.NoError(t, err)
	require.True(t, acquired)

	// Replica B tries immediately — collision.
	acquired, err = lockB.Acquire(ctx, testJobKey, replicaExecB, testLockTTL)
	require.NoError(t, err)
	require.False(t, acquired, "Replica B must lose while the TTL is alive")

	// Replica A "crashes" — we never call Release. FastForward
	// the miniredis clock past the TTL.
	s.FastForward(150 * time.Millisecond)
	assert.False(t, s.Exists(testLockKey), "lock must be gone after the TTL elapses")

	// Replica B can now take over.
	acquired, err = lockB.Acquire(ctx, testJobKey, replicaExecB, testLockTTL)
	require.NoError(t, err)
	require.True(t, acquired, "Replica B must take over once the TTL has elapsed")
	require.NoError(t, lockB.Release(ctx, testJobKey, replicaExecB))
}
