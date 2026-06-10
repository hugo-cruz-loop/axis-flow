package repository_test

import (
	"context"
	"testing"

	"axis-flow-back/internal/atencionseguimiento"
	"axis-flow-back/internal/atencionseguimiento/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func newTestInMemTicketRepo(t *testing.T) *repository.InMemTicketRepository {
	t.Helper()
	return repository.NewInMemTicketRepository()
}

func sampleTicket(empresaID, clienteID uuid.UUID, estatus int) *atencionseguimiento.TicketServicio {
	return &atencionseguimiento.TicketServicio{
		ID:          uuid.New(),
		EmpresaID:   empresaID,
		ClienteID:   clienteID,
		LocalidadID: uuid.New(),
		Asunto:      "soporte",
		Descripcion: "detalle",
		Estatus:     estatus,
		UltimaResp:  atencionseguimiento.RolRHGestor,
	}
}

// ---------------------------------------------------------------------------
// GetTicket: not found + correct record
// ---------------------------------------------------------------------------

func TestInMemTicketRepositoryGetTicketReturnsNotFoundForMissing(t *testing.T) {
	repo := newTestInMemTicketRepo(t)
	ctx := context.Background()

	_, err := repo.GetTicket(ctx, uuid.New())
	require.ErrorIs(t, err, atencionseguimiento.ErrNotFound)
}

func TestInMemTicketRepositoryGetTicketReturnsCorrectRecord(t *testing.T) {
	repo := newTestInMemTicketRepo(t)
	ctx := context.Background()

	empresaID := uuid.New()
	clienteID := uuid.New()
	ticket := sampleTicket(empresaID, clienteID, atencionseguimiento.EstatusPendiente)
	require.NoError(t, repo.CreateTicket(ctx, ticket))

	got, err := repo.GetTicket(ctx, ticket.ID)
	require.NoError(t, err)
	assert.Equal(t, ticket.ID, got.ID)
	assert.Equal(t, clienteID, got.ClienteID)
}

// ---------------------------------------------------------------------------
// GetTicketStats: correct counts, zero for missing buckets
// ---------------------------------------------------------------------------

func TestInMemTicketRepositoryGetTicketStatsReturnsCountsPerEstatusWithZeroForMissing(t *testing.T) {
	repo := newTestInMemTicketRepo(t)
	ctx := context.Background()

	empresaID := uuid.New()
	clienteID := uuid.New()

	require.NoError(t, repo.CreateTicket(ctx, sampleTicket(empresaID, clienteID, atencionseguimiento.EstatusPendiente)))
	require.NoError(t, repo.CreateTicket(ctx, sampleTicket(empresaID, clienteID, atencionseguimiento.EstatusPendiente)))
	require.NoError(t, repo.CreateTicket(ctx, sampleTicket(empresaID, clienteID, atencionseguimiento.EstatusEnProceso)))
	// no EstatusFinalizado tickets

	stats, err := repo.GetTicketStats(ctx, empresaID)
	require.NoError(t, err)
	assert.Equal(t, empresaID, stats.EmpresaID)
	assert.Equal(t, 2, stats.Pendiente)
	assert.Equal(t, 1, stats.EnProceso)
	assert.Equal(t, 0, stats.Finalizado) // missing bucket returns 0
}

func TestInMemTicketRepositoryGetTicketStatsExcludesOtherEmpresas(t *testing.T) {
	repo := newTestInMemTicketRepo(t)
	ctx := context.Background()

	empresaID := uuid.New()
	otherEmpresaID := uuid.New()

	require.NoError(t, repo.CreateTicket(ctx, sampleTicket(empresaID, uuid.New(), atencionseguimiento.EstatusPendiente)))
	require.NoError(t, repo.CreateTicket(ctx, sampleTicket(otherEmpresaID, uuid.New(), atencionseguimiento.EstatusPendiente)))

	stats, err := repo.GetTicketStats(ctx, empresaID)
	require.NoError(t, err)
	assert.Equal(t, 1, stats.Pendiente)
}

// ---------------------------------------------------------------------------
// UpdateTicketEstatus: tenant check
// ---------------------------------------------------------------------------

func TestInMemTicketRepositoryUpdateTicketEstatusReturnsForbiddenForWrongEmpresa(t *testing.T) {
	repo := newTestInMemTicketRepo(t)
	ctx := context.Background()

	empresaID := uuid.New()
	ticket := sampleTicket(empresaID, uuid.New(), atencionseguimiento.EstatusPendiente)
	require.NoError(t, repo.CreateTicket(ctx, ticket))

	wrongEmpresaID := uuid.New()
	_, err := repo.UpdateTicketEstatus(ctx, ticket.ID, wrongEmpresaID, atencionseguimiento.EstatusEnProceso)
	require.ErrorIs(t, err, atencionseguimiento.ErrForbidden)
}

func TestInMemTicketRepositoryUpdateTicketEstatusUpdatesCorrectly(t *testing.T) {
	repo := newTestInMemTicketRepo(t)
	ctx := context.Background()

	empresaID := uuid.New()
	ticket := sampleTicket(empresaID, uuid.New(), atencionseguimiento.EstatusPendiente)
	require.NoError(t, repo.CreateTicket(ctx, ticket))

	updated, err := repo.UpdateTicketEstatus(ctx, ticket.ID, empresaID, atencionseguimiento.EstatusEnProceso)
	require.NoError(t, err)
	assert.Equal(t, atencionseguimiento.EstatusEnProceso, updated.Estatus)
}

// ---------------------------------------------------------------------------
// CreateRespuestaServicio: Redis UNLINK called
// ---------------------------------------------------------------------------

func TestCreateRespuestaServicioInvalidatesRedisKeys(t *testing.T) {
	mr, client := newTestRedis(t)
	ctx := context.Background()

	clienteID := uuid.New()
	empresaID := uuid.New()
	mr.Set("atencion_seguimiento:cliente:"+clienteID.String()+":unread_tickets", "7")
	mr.Set("atencion_seguimiento:empresa:"+empresaID.String()+":unread_tickets", "4")

	invalidator := repository.NewRedisAtencionCacheInvalidator(client)

	require.NoError(t, invalidator.UnlinkTicketRespuestaKeys(ctx, clienteID, empresaID))

	assert.False(t, mr.Exists("atencion_seguimiento:cliente:"+clienteID.String()+":unread_tickets"))
	assert.False(t, mr.Exists("atencion_seguimiento:empresa:"+empresaID.String()+":unread_tickets"))
}

// ---------------------------------------------------------------------------
// CloseTicketsByCliente
// ---------------------------------------------------------------------------

func TestInMemTicketRepositoryCloseTicketsByClienteUpdatesOnlyOpen(t *testing.T) {
	repo := newTestInMemTicketRepo(t)
	ctx := context.Background()

	empresaID := uuid.New()
	clienteID := uuid.New()

	t1 := sampleTicket(empresaID, clienteID, atencionseguimiento.EstatusPendiente)
	t2 := sampleTicket(empresaID, clienteID, atencionseguimiento.EstatusEnProceso)
	t3 := sampleTicket(empresaID, clienteID, atencionseguimiento.EstatusFinalizado) // already closed
	other := sampleTicket(empresaID, uuid.New(), atencionseguimiento.EstatusPendiente)

	require.NoError(t, repo.CreateTicket(ctx, t1))
	require.NoError(t, repo.CreateTicket(ctx, t2))
	require.NoError(t, repo.CreateTicket(ctx, t3))
	require.NoError(t, repo.CreateTicket(ctx, other))

	require.NoError(t, repo.CloseTicketsByCliente(ctx, clienteID))

	got1, _ := repo.GetTicket(ctx, t1.ID)
	got2, _ := repo.GetTicket(ctx, t2.ID)
	got3, _ := repo.GetTicket(ctx, t3.ID)
	gotOther, _ := repo.GetTicket(ctx, other.ID)

	assert.Equal(t, atencionseguimiento.EstatusFinalizado, got1.Estatus)
	assert.Equal(t, atencionseguimiento.EstatusFinalizado, got2.Estatus)
	assert.Equal(t, atencionseguimiento.EstatusFinalizado, got3.Estatus)
	assert.Equal(t, atencionseguimiento.EstatusPendiente, gotOther.Estatus)
}
