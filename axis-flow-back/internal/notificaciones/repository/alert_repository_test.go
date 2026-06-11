package repository_test

import (
	"context"
	"testing"

	"axis-flow-back/internal/notificaciones"
	"axis-flow-back/internal/notificaciones/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func newAlertRepo(t *testing.T) *repository.InMemAlertRepository {
	t.Helper()
	return repository.NewInMemAlertRepository()
}

func sampleAlert(userID uuid.UUID) notificaciones.NotificacionAtencion {
	ticketID := 42
	return notificaciones.NotificacionAtencion{
		UserID:   userID,
		TicketID: &ticketID,
		Mensaje:  "su ticket fue actualizado",
		Estatus:  notificaciones.EstatusUnread,
	}
}

// ---------------------------------------------------------------------------
// InsertAlert
// ---------------------------------------------------------------------------

func TestInMemAlertRepositoryInsertAlertAssignsIDAndTimestamps(t *testing.T) {
	repo := newAlertRepo(t)
	ctx := context.Background()
	userID := uuid.New()

	got, err := repo.InsertAlert(ctx, sampleAlert(userID))
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, got.ID)
	assert.False(t, got.CreatedAt.IsZero())
	assert.False(t, got.UpdatedAt.IsZero())
}

func TestInMemAlertRepositoryInsertAlertPreservesFields(t *testing.T) {
	repo := newAlertRepo(t)
	ctx := context.Background()
	userID := uuid.New()
	n := sampleAlert(userID)

	got, err := repo.InsertAlert(ctx, n)
	require.NoError(t, err)
	assert.Equal(t, userID, got.UserID)
	assert.Equal(t, n.Mensaje, got.Mensaje)
	assert.Equal(t, notificaciones.EstatusUnread, got.Estatus)
	require.NotNil(t, got.TicketID)
	assert.Equal(t, *n.TicketID, *got.TicketID)
}

// ---------------------------------------------------------------------------
// UpdateEstatus
// ---------------------------------------------------------------------------

func TestInMemAlertRepositoryUpdateEstatusReturnsNotFoundForMissing(t *testing.T) {
	repo := newAlertRepo(t)
	ctx := context.Background()

	_, err := repo.UpdateEstatus(ctx, uuid.New(), int(notificaciones.EstatusRead))
	require.ErrorIs(t, err, notificaciones.ErrNotFound)
}

func TestInMemAlertRepositoryUpdateEstatusChangesEstatus(t *testing.T) {
	repo := newAlertRepo(t)
	ctx := context.Background()
	userID := uuid.New()

	inserted, err := repo.InsertAlert(ctx, sampleAlert(userID))
	require.NoError(t, err)

	updated, err := repo.UpdateEstatus(ctx, inserted.ID, int(notificaciones.EstatusRead))
	require.NoError(t, err)
	assert.Equal(t, notificaciones.EstatusRead, updated.Estatus)
}

func TestInMemAlertRepositoryUpdateEstatusUpdatesUpdatedAt(t *testing.T) {
	repo := newAlertRepo(t)
	ctx := context.Background()
	userID := uuid.New()

	inserted, err := repo.InsertAlert(ctx, sampleAlert(userID))
	require.NoError(t, err)
	before := inserted.UpdatedAt

	updated, err := repo.UpdateEstatus(ctx, inserted.ID, int(notificaciones.EstatusRead))
	require.NoError(t, err)
	assert.False(t, updated.UpdatedAt.Before(before))
}

// ---------------------------------------------------------------------------
// CountUnread
// ---------------------------------------------------------------------------

func TestInMemAlertRepositoryCountUnreadReturnsZeroForUnknownUser(t *testing.T) {
	repo := newAlertRepo(t)
	ctx := context.Background()

	count, err := repo.CountUnread(ctx, uuid.New())
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestInMemAlertRepositoryCountUnreadCountsOnlyUnread(t *testing.T) {
	repo := newAlertRepo(t)
	ctx := context.Background()
	userID := uuid.New()

	// insert 2 unread, 1 read
	a1, err := repo.InsertAlert(ctx, sampleAlert(userID))
	require.NoError(t, err)
	_, err = repo.InsertAlert(ctx, sampleAlert(userID))
	require.NoError(t, err)
	_, err = repo.InsertAlert(ctx, sampleAlert(userID))
	require.NoError(t, err)

	// mark one as read
	_, err = repo.UpdateEstatus(ctx, a1.ID, int(notificaciones.EstatusRead))
	require.NoError(t, err)

	count, err := repo.CountUnread(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestInMemAlertRepositoryCountUnreadExcludesOtherUsers(t *testing.T) {
	repo := newAlertRepo(t)
	ctx := context.Background()
	userA := uuid.New()
	userB := uuid.New()

	_, err := repo.InsertAlert(ctx, sampleAlert(userA))
	require.NoError(t, err)
	_, err = repo.InsertAlert(ctx, sampleAlert(userB))
	require.NoError(t, err)

	count, err := repo.CountUnread(ctx, userA)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

// ---------------------------------------------------------------------------
// ListByUser
// ---------------------------------------------------------------------------

func TestInMemAlertRepositoryListByUserReturnsEmptyForUnknownUser(t *testing.T) {
	repo := newAlertRepo(t)
	ctx := context.Background()

	items, total, err := repo.ListByUser(ctx, uuid.New(), 1, 10)
	require.NoError(t, err)
	assert.Equal(t, 0, total)
	assert.Empty(t, items)
}

func TestInMemAlertRepositoryListByUserReturnsPaginatedResultsOrderedDesc(t *testing.T) {
	repo := newAlertRepo(t)
	ctx := context.Background()
	userID := uuid.New()

	for i := 0; i < 5; i++ {
		_, err := repo.InsertAlert(ctx, sampleAlert(userID))
		require.NoError(t, err)
	}

	items, total, err := repo.ListByUser(ctx, userID, 1, 3)
	require.NoError(t, err)
	assert.Equal(t, 5, total)
	assert.Len(t, items, 3)
}

func TestInMemAlertRepositoryListByUserPageBeyondEndReturnsEmpty(t *testing.T) {
	repo := newAlertRepo(t)
	ctx := context.Background()
	userID := uuid.New()

	_, err := repo.InsertAlert(ctx, sampleAlert(userID))
	require.NoError(t, err)

	items, total, err := repo.ListByUser(ctx, userID, 2, 10)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Empty(t, items)
}

func TestInMemAlertRepositoryListByUserExcludesOtherUsers(t *testing.T) {
	repo := newAlertRepo(t)
	ctx := context.Background()
	userA := uuid.New()
	userB := uuid.New()

	_, err := repo.InsertAlert(ctx, sampleAlert(userA))
	require.NoError(t, err)
	_, err = repo.InsertAlert(ctx, sampleAlert(userB))
	require.NoError(t, err)

	items, total, err := repo.ListByUser(ctx, userA, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, items, 1)
	assert.Equal(t, userA, items[0].UserID)
}
