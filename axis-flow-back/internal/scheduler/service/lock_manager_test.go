// Package service_test contains black-box tests for the scheduler
// service layer. We deliberately use the external test package
// (`service_test`) instead of `service` so the tests verify the
// PUBLIC API of LockManager — exactly the surface the CronRunner
// consumes in production.
//
// PR 5A (this file) covers task 5.2: distributed-lock semantics
// for the redisLockManager implementation. The contract is described
// in the spec section "Datos > Distributed Locking Strategy" and
// implemented in service/lock_manager.go. We exercise:
//
//   - Acquire + Release: the happy path; the Redis key is created
//     with the right TTL and removed by the release script.
//   - Acquire collision: a second Acquire on the same key with a
//     different execution_id must fail (SET NX), and a Release from
//     the wrong owner must NOT evict the key.
//   - RenewHeartbeat: a Lua-driven PEXPIRE that extends the TTL
//     only when the caller still owns the key.
//   - RenewHeartbeat on a missing key: the script returns 0 and the
//     caller learns the lock is gone (ErrLockLost).
//
// Each test uses a fresh in-process miniredis so they can run in
// parallel and cannot interfere with one another. The mini-redis
// FastForward method lets us simulate elapsed wall time without
// blocking the test runner on real sleeps.
package service_test

import (
	"context"
	"testing"
	"time"

	"axis-flow-back/internal/scheduler/service"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestLockManager spins up an in-process miniredis, builds a
// *redis.Client pointed at it, and returns the LockManager plus the
// miniredis handle (so tests can assert key existence / TTL directly
// and call FastForward to advance the simulated clock).
func newTestLockManager(t *testing.T) (service.LockManager, *miniredis.Miniredis, *redis.Client) {
	t.Helper()
	s := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return service.NewRedisLockManager(client), s, client
}

// TestLockManager_AcquireRelease is the happy path: a successful
// SET NX PX creates the scheduler:lock:<jobKey> key with a positive
// TTL, and the release script deletes it when called with the
// matching execution_id.
func TestLockManager_AcquireRelease(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	lm, s, _ := newTestLockManager(t)

	acquired, err := lm.Acquire(ctx, "job1", "exec-A", time.Second)
	require.NoError(t, err)
	require.True(t, acquired, "first Acquire on a fresh key must succeed")

	lockKey := "scheduler:lock:job1"
	assert.True(t, s.Exists(lockKey), "miniredis must observe the lock key")
	ttl := s.TTL(lockKey)
	assert.Greater(t, ttl, time.Duration(0), "lock must carry a positive TTL")
	assert.LessOrEqual(t, ttl, time.Second, "lock TTL must not exceed the requested ttl")

	require.NoError(t, lm.Release(ctx, "job1", "exec-A"))
	assert.False(t, s.Exists(lockKey), "release must delete the lock key")
}

// TestLockManager_AcquireConflict verifies the SET-NX collision
// behaviour: a second Acquire on the same key from a different
// execution_id must fail without an error, and a Release from the
// wrong owner must NOT delete the key. The real owner can still
// release it cleanly.
func TestLockManager_AcquireConflict(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	lm, s, _ := newTestLockManager(t)

	// exec-A wins the lock.
	acquired, err := lm.Acquire(ctx, "job1", "exec-A", 5*time.Second)
	require.NoError(t, err)
	require.True(t, acquired)

	// exec-B tries to take the same key — SET NX must fail. The
	// contract is acquired=false with a nil error: this is a
	// business-level "skip the run", not a transport failure.
	acquired, err = lm.Acquire(ctx, "job1", "exec-B", 5*time.Second)
	require.NoError(t, err, "collision is a business condition, not an error")
	assert.False(t, acquired, "second Acquire on a held key must return acquired=false")

	// exec-B tries to release — the Lua script checks the value
	// before deleting, so the key survives and the caller is told
	// the lock is lost (different owner).
	lockKey := "scheduler:lock:job1"
	err = lm.Release(ctx, "job1", "exec-B")
	require.Error(t, err, "release from a non-owner must report ErrLockLost")
	assert.ErrorIs(t, err, service.ErrLockLost)
	assert.True(t, s.Exists(lockKey), "wrong-owner release must NOT delete the key")

	// The real owner can still release the lock cleanly.
	require.NoError(t, lm.Release(ctx, "job1", "exec-A"))
	assert.False(t, s.Exists(lockKey), "real owner must be able to release the key")
}

// TestLockManager_RenewHeartbeat covers the PEXPIRE path: the
// script extends the TTL only when the value still matches. We
// fast-forward the miniredis clock by 300ms (well below the 500ms
// initial TTL) to prove the renew works mid-life rather than just
// resetting a fresh lock.
func TestLockManager_RenewHeartbeat(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	lm, s, _ := newTestLockManager(t)

	acquired, err := lm.Acquire(ctx, "job1", "exec-A", 500*time.Millisecond)
	require.NoError(t, err)
	require.True(t, acquired)

	// Advance the simulated clock. The lock's remaining TTL drops
	// to ~200ms but the key is still alive.
	s.FastForward(300 * time.Millisecond)
	lockKey := "scheduler:lock:job1"
	assert.True(t, s.Exists(lockKey), "lock must still be alive after 300ms of a 500ms TTL")

	renewed, err := lm.RenewHeartbeat(ctx, "job1", "exec-A", 2*time.Second)
	require.NoError(t, err)
	require.True(t, renewed, "renew with the real owner must succeed")

	ttl := s.TTL(lockKey)
	assert.Greater(t, ttl, time.Second, "new TTL must be > 1s after a 2s renew")
	assert.LessOrEqual(t, ttl, 2*time.Second, "new TTL must not exceed the requested ttl")

	// A renew from a non-owner must fail and surface ErrLockLost so
	// the heartbeat loop can stop cleanly.
	renewed, err = lm.RenewHeartbeat(ctx, "job1", "exec-B", 2*time.Second)
	require.Error(t, err, "renew from a non-owner must report ErrLockLost")
	assert.ErrorIs(t, err, service.ErrLockLost)
	assert.False(t, renewed)
}

// TestLockManager_RenewHeartbeatNoKey documents the missing-key
// path: with no Acquire ever performed, the renew script sees the
// key absent, returns 0, and the manager surfaces ErrLockLost. This
// is the recovery signal the cron runner's heartbeat loop relies on
// to abort an orphaned in-flight run.
func TestLockManager_RenewHeartbeatNoKey(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	lm, s, _ := newTestLockManager(t)

	// Sanity: no key yet.
	assert.False(t, s.Exists("scheduler:lock:job1"))

	renewed, err := lm.RenewHeartbeat(ctx, "job1", "exec-A", 2*time.Second)
	require.Error(t, err, "renewing a non-existent key must return ErrLockLost")
	assert.ErrorIs(t, err, service.ErrLockLost)
	assert.False(t, renewed, "renewed must be false when the key is missing")
}
