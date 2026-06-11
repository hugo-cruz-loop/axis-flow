package repository_test

import (
	"context"
	"testing"
	"time"

	"axis-flow-back/internal/notificaciones"
	"axis-flow-back/internal/notificaciones/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func newPushRepo(t *testing.T) *repository.InMemPushRepository {
	t.Helper()
	return repository.NewInMemPushRepository()
}

func samplePush(userID uuid.UUID) notificaciones.NotificacionEnviada {
	notiID := "fcm-abc-123"
	return notificaciones.NotificacionEnviada{
		UserID:     userID,
		Token:      "device-token-xyz",
		NotiID:     &notiID,
		DeviceType: notificaciones.DeviceTypeAndroid,
	}
}

// ---------------------------------------------------------------------------
// InsertPush
// ---------------------------------------------------------------------------

func TestInMemPushRepositoryInsertPushAssignsIDAndCreatedAt(t *testing.T) {
	repo := newPushRepo(t)
	ctx := context.Background()
	userID := uuid.New()

	got, err := repo.InsertPush(ctx, nil, samplePush(userID))
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, got.ID)
	assert.False(t, got.CreatedAt.IsZero())
	assert.Equal(t, userID, got.UserID)
}

func TestInMemPushRepositoryInsertPushPreservesFields(t *testing.T) {
	repo := newPushRepo(t)
	ctx := context.Background()
	userID := uuid.New()
	n := samplePush(userID)

	got, err := repo.InsertPush(ctx, nil, n)
	require.NoError(t, err)
	assert.Equal(t, n.DeviceType, got.DeviceType)
	assert.Equal(t, n.Token, got.Token)
	require.NotNil(t, got.NotiID)
	assert.Equal(t, *n.NotiID, *got.NotiID)
}

// ---------------------------------------------------------------------------
// ListPushByUser
// ---------------------------------------------------------------------------

func TestInMemPushRepositoryListPushByUserReturnsEmptyForUnknownUser(t *testing.T) {
	repo := newPushRepo(t)
	ctx := context.Background()

	items, total, err := repo.ListPushByUser(ctx, uuid.New(), 1, 10)
	require.NoError(t, err)
	assert.Equal(t, 0, total)
	assert.Empty(t, items)
}

func TestInMemPushRepositoryListPushByUserReturnsPaginatedResults(t *testing.T) {
	repo := newPushRepo(t)
	ctx := context.Background()
	userID := uuid.New()

	// insert 5 records
	for i := 0; i < 5; i++ {
		n := samplePush(userID)
		n.CreatedAt = time.Now().Add(time.Duration(i) * time.Second)
		_, err := repo.InsertPush(ctx, nil, n)
		require.NoError(t, err)
	}

	items, total, err := repo.ListPushByUser(ctx, userID, 1, 3)
	require.NoError(t, err)
	assert.Equal(t, 5, total)
	assert.Len(t, items, 3)
}

func TestInMemPushRepositoryListPushByUserPageBeyondEndReturnsEmpty(t *testing.T) {
	repo := newPushRepo(t)
	ctx := context.Background()
	userID := uuid.New()

	_, err := repo.InsertPush(ctx, nil, samplePush(userID))
	require.NoError(t, err)

	items, total, err := repo.ListPushByUser(ctx, userID, 2, 10)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Empty(t, items)
}

func TestInMemPushRepositoryListPushByUserExcludesOtherUsers(t *testing.T) {
	repo := newPushRepo(t)
	ctx := context.Background()
	userA := uuid.New()
	userB := uuid.New()

	_, err := repo.InsertPush(ctx, nil, samplePush(userA))
	require.NoError(t, err)
	_, err = repo.InsertPush(ctx, nil, samplePush(userB))
	require.NoError(t, err)

	items, total, err := repo.ListPushByUser(ctx, userA, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, items, 1)
	assert.Equal(t, userA, items[0].UserID)
}
