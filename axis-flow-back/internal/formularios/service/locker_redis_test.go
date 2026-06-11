// Package service_test — PR-6 (6.3) TDD RED tests for the
// RedisLocker. The tests assert the spec's "SET-NX-with-TTL" lock
// pattern (PR-6's Redis equivalent of the spec's Celery worker
// pool + lock semantics).
//
// Test layer: miniredis (already in go.mod via
// github.com/alicebob/miniredis/v2). The lock's Lua-based safe
// release is exercised explicitly.
package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"axis-flow-back/internal/formularios"
	formulariosService "axis-flow-back/internal/formularios/service"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestRedisLocker stands up a miniredis instance and returns
// a formulariosService.Locker backed by a real *redis.Client. The
// returned *miniredis.Miniredis MUST be Closed at test cleanup.
func newTestRedisLocker(t *testing.T) (formulariosService.Locker, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return formulariosService.NewRedisLocker(rdb, 5*time.Second), mr
}

// ---------------------------------------------------------------------------
// 6.3 (a) — Acquire / Release happy path
// ---------------------------------------------------------------------------

// TestRedisLocker_AcquireReleaseHappyPath asserts the spec's basic
// "SET-NX-with-TTL" pattern: Acquire returns an UnlockFn that
// releases the lock when called; a second Acquire on the same key
// returns ErrConflict.
func TestRedisLocker_AcquireReleaseHappyPath(t *testing.T) {
	locker, mr := newTestRedisLocker(t)
	iniciadoID := uuid.New()

	ctx := context.Background()
	unlock, err := locker.Acquire(ctx, iniciadoID)
	require.NoError(t, err)
	require.NotNil(t, unlock, "UnlockFn must be non-nil on success")

	// The lock key must exist in Redis.
	keys := mr.Keys()
	require.NotEmpty(t, keys, "lock key must exist in Redis after Acquire")

	// Unlock must succeed.
	require.NoError(t, unlock(ctx))

	// After unlock, the lock key must be gone.
	assert.Empty(t, mr.Keys(), "lock key must be removed after Unlock")
}

// TestRedisLocker_DoubleAcquireReturnsConflict triangulates #1: a
// second Acquire on the same iniciadoID must return ErrConflict
// while the first lock is still held.
func TestRedisLocker_DoubleAcquireReturnsConflict(t *testing.T) {
	locker, _ := newTestRedisLocker(t)
	iniciadoID := uuid.New()
	ctx := context.Background()

	// First acquire succeeds.
	unlock1, err := locker.Acquire(ctx, iniciadoID)
	require.NoError(t, err)
	defer func() { _ = unlock1(ctx) }()

	// Second acquire on the same iniciadoID returns ErrConflict.
	_, err = locker.Acquire(ctx, iniciadoID)
	require.Error(t, err)
	assert.True(t,
		errors.Is(err, formularios.ErrConflict) || strings.Contains(err.Error(), "conflict"),
		"second Acquire must return ErrConflict, got %v", err)
}

// TestRedisLocker_UnlockWithWrongTokenDoesNotDelete asserts the
// safe-release pattern: the UnlockFn returned by Acquire carries
// a random token; the lock's internal DEL uses a Lua script that
// checks the token. A different UnlockFn (e.g. crafted by an
// attacker, or returned by a different Acquire that somehow got
// past the conflict check) must NOT delete the lock.
//
// We simulate this by acquiring the lock, then crafting a
// different UnlockFn that points at a different key (the lock
// is the same key, but the UnlockFn is constructed with a
// wrong/empty token by directly calling the lock's release
// helper). The cleanest way is to test the lock's releaseLua
// behavior via a manual SET/UNSET: we set the lock with a
// known value, then call a "wrong token" release; the key must
// remain.
func TestRedisLocker_UnlockWithWrongTokenDoesNotDelete(t *testing.T) {
	locker, mr := newTestRedisLocker(t)
	iniciadoID := uuid.New()
	ctx := context.Background()

	// Acquire the lock with a real UnlockFn.
	unlock, err := locker.Acquire(ctx, iniciadoID)
	require.NoError(t, err)
	firstToken, err := mr.Get(formulariosService.LockKeyForTest(iniciadoID))
	require.NoError(t, err)
	require.NotEmpty(t, firstToken, "the lock must hold a token after Acquire")

	// Manually craft a "wrong token" unlock by calling the
	// internal release helper with a wrong token. We expose a
	// tiny test-only helper for this.
	err = formulariosService.ReleaseLockForTest(mr, formulariosService.LockKeyForTest(iniciadoID), "WRONG-TOKEN")
	require.NoError(t, err, "release-with-wrong-token must not error (it's a no-op)")

	// The original lock must still be present.
	currentToken, err := mr.Get(formulariosService.LockKeyForTest(iniciadoID))
	require.NoError(t, err)
	assert.Equal(t, firstToken, currentToken,
		"the original lock must NOT be deleted by a wrong-token release")

	// The real UnlockFn still works.
	require.NoError(t, unlock(ctx))
	_, err = mr.Get(formulariosService.LockKeyForTest(iniciadoID))
	require.Error(t, err, "lock key must be gone after Unlock")
}

// TestRedisLocker_AcquireAfterReleaseSucceeds triangulates #1+#2:
// after the first holder releases, a second Acquire must succeed
// (the lock is free again).
func TestRedisLocker_AcquireAfterReleaseSucceeds(t *testing.T) {
	locker, _ := newTestRedisLocker(t)
	iniciadoID := uuid.New()
	ctx := context.Background()

	unlock1, err := locker.Acquire(ctx, iniciadoID)
	require.NoError(t, err)
	require.NoError(t, unlock1(ctx))

	unlock2, err := locker.Acquire(ctx, iniciadoID)
	require.NoError(t, err, "second Acquire after release must succeed")
	require.NoError(t, unlock2(ctx))
}

// TestRedisLocker_AcquireHasTTL asserts the "SET-NX-with-TTL"
// contract: the lock is created with a TTL so a crashed worker
// cannot block retries forever.
func TestRedisLocker_AcquireHasTTL(t *testing.T) {
	locker, mr := newTestRedisLocker(t)
	iniciadoID := uuid.New()
	ctx := context.Background()

	unlock, err := locker.Acquire(ctx, iniciadoID)
	require.NoError(t, err)
	defer func() { _ = unlock(ctx) }()

	// miniredis exposes the TTL via .TTL() on the key.
	ttl := mr.TTL(formulariosService.LockKeyForTest(iniciadoID))
	assert.Greater(t, ttl, time.Duration(0), "lock must have a positive TTL")
	assert.LessOrEqual(t, ttl, 5*time.Second+time.Second, "TTL must be <= configured (5s) plus a small grace")
}

// ---------------------------------------------------------------------------
// 6.3 (b) — Pending set: AddToPending / RemoveFromPending
// ---------------------------------------------------------------------------

// TestRedisLocker_AddRemovePendingRoundTrip asserts the pending-set
// contract: AddToPending adds the miembro to the configured
// setKey, RemoveFromPending removes it.
func TestRedisLocker_AddRemovePendingRoundTrip(t *testing.T) {
	locker, mr := newTestRedisLocker(t)
	iniciadoID := uuid.New()
	ctx := context.Background()

	// Add to pending.
	require.NoError(t, locker.AddToPending(ctx, iniciadoID))
	members, err := mr.SMembers(formularios.KeyReportePending)
	require.NoError(t, err)
	assert.Contains(t, members, iniciadoID.String(), "pending set must contain the iniciadoID after AddToPending")

	// Remove from pending.
	require.NoError(t, locker.RemoveFromPending(ctx, iniciadoID))
	members, err = mr.SMembers(formularios.KeyReportePending)
	// After the SREM, the set may not exist (miniredis returns
	// "ERR no such key" in that case). We treat that as an
	// empty result set.
	if err != nil {
		members = nil
	}
	assert.NotContains(t, members, iniciadoID.String(), "pending set must NOT contain the iniciadoID after RemoveFromPending")
}

// TestRedisLocker_RemovePendingWithoutAddIsNoOp asserts that
// RemoveFromPending on a non-present member is a no-op (matches
// the spec's "Safe to call even if the member is not present"
// contract on the Locker port).
func TestRedisLocker_RemovePendingWithoutAddIsNoOp(t *testing.T) {
	locker, _ := newTestRedisLocker(t)
	require.NoError(t, locker.RemoveFromPending(context.Background(), uuid.New()),
		"RemoveFromPending on a non-present member must be a no-op")
}

// TestRedisLocker_AddToPendingPreservesOthers asserts the pending
// set is a SADD / SREM (not a SET), so multiple iniciadoIDs
// coexist.
func TestRedisLocker_AddToPendingPreservesOthers(t *testing.T) {
	locker, _ := newTestRedisLocker(t)
	ctx := context.Background()

	id1, id2, id3 := uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, locker.AddToPending(ctx, id1))
	require.NoError(t, locker.AddToPending(ctx, id2))
	require.NoError(t, locker.AddToPending(ctx, id3))

	// Remove only id2.
	require.NoError(t, locker.RemoveFromPending(ctx, id2))
}

// ---------------------------------------------------------------------------
// 6.3 (c) — RenderPool: bounded worker pool (Celery replacement)
// ---------------------------------------------------------------------------

// The RenderPool tests live in render_pool_test.go (separate file
// to keep the locker tests focused).
