package atencionseguimiento_test

import (
	"context"
	"testing"

	"axis-flow-back/internal/atencionseguimiento"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEstatusConstants asserts the three estatus values are correctly defined.
func TestEstatusConstants(t *testing.T) {
	assert.Equal(t, 1, atencionseguimiento.EstatusPendiente)
	assert.Equal(t, 2, atencionseguimiento.EstatusEnProceso)
	assert.Equal(t, 3, atencionseguimiento.EstatusFinalizado)
}

// TestRolConstants asserts UltimaResp role constants are distinct and ordered.
func TestRolConstants(t *testing.T) {
	assert.Equal(t, 1, atencionseguimiento.RolEmpleadoCliente)
	assert.Equal(t, 2, atencionseguimiento.RolRHGestor)
	assert.Equal(t, 3, atencionseguimiento.RolSupervisor)

	// Triangulate: all three are distinct
	assert.NotEqual(t, atencionseguimiento.RolEmpleadoCliente, atencionseguimiento.RolRHGestor)
	assert.NotEqual(t, atencionseguimiento.RolRHGestor, atencionseguimiento.RolSupervisor)
}

// TestSentinelErrors asserts each sentinel error has a unique non-empty message.
func TestSentinelErrors(t *testing.T) {
	errs := []error{
		atencionseguimiento.ErrNotFound,
		atencionseguimiento.ErrForbidden,
		atencionseguimiento.ErrConflict,
		atencionseguimiento.ErrInvalidInput,
		atencionseguimiento.ErrSLAConfigMissing,
	}

	seen := make(map[string]bool)
	for _, e := range errs {
		require.NotNil(t, e)
		msg := e.Error()
		assert.NotEmpty(t, msg)
		assert.False(t, seen[msg], "duplicate error message: %s", msg)
		seen[msg] = true
	}
}

// TestRedisCacheKeyConstantsAreDefined asserts all five Redis key constants exist
// and are non-empty strings.
func TestRedisCacheKeyConstantsAreDefined(t *testing.T) {
	keys := []string{
		atencionseguimiento.KeyClienteUnreadTickets,
		atencionseguimiento.KeyClienteActiveTickets,
		atencionseguimiento.KeyEmpleadoUnreadComplaints,
		atencionseguimiento.KeyEmpresaUnreadComplaints,
		atencionseguimiento.KeyEmpresaUnreadTickets,
	}

	seen := make(map[string]bool)
	for _, k := range keys {
		assert.NotEmpty(t, k, "Redis key constant must not be empty")
		assert.False(t, seen[k], "duplicate Redis key constant: %s", k)
		seen[k] = true
	}
}

// TestRepositoryPortsCompile verifies that stub types satisfy the repository
// interfaces defined in the domain — compile-time contract.
func TestRepositoryPortsCompile(t *testing.T) {
	var _ atencionseguimiento.SolicitudQuejaRepository = (*solicitudQuejaRepositoryStub)(nil)
	var _ atencionseguimiento.TicketServicioRepository = (*ticketServicioRepositoryStub)(nil)
	var _ atencionseguimiento.IncidenciaSupervisorRepository = (*incidenciaSupervisorRepositoryStub)(nil)
}

// ---------------------------------------------------------------------------
// Stubs — satisfy interfaces at compile time.
// ---------------------------------------------------------------------------

type solicitudQuejaRepositoryStub struct{}

func (solicitudQuejaRepositoryStub) Create(_ context.Context, _ *atencionseguimiento.SolicitudQueja) error {
	return nil
}
func (solicitudQuejaRepositoryStub) GetByID(_ context.Context, _ uuid.UUID, _ int64) (*atencionseguimiento.SolicitudQueja, error) {
	return nil, nil
}
func (solicitudQuejaRepositoryStub) ListByEmpresa(_ context.Context, _ uuid.UUID, _ *int, _, _ int) ([]*atencionseguimiento.SolicitudQueja, int, error) {
	return nil, 0, nil
}
func (solicitudQuejaRepositoryStub) UpdateEstatus(_ context.Context, _ uuid.UUID, _ int) error {
	return nil
}
func (solicitudQuejaRepositoryStub) CreateRespuesta(_ context.Context, _ *atencionseguimiento.RespuestaQueja) error {
	return nil
}
func (solicitudQuejaRepositoryStub) ListRespuestas(_ context.Context, _ uuid.UUID) ([]*atencionseguimiento.RespuestaQueja, error) {
	return nil, nil
}

type ticketServicioRepositoryStub struct{}

func (ticketServicioRepositoryStub) Create(_ context.Context, _ *atencionseguimiento.TicketServicio) error {
	return nil
}
func (ticketServicioRepositoryStub) GetByID(_ context.Context, _, _ uuid.UUID) (*atencionseguimiento.TicketServicio, error) {
	return nil, nil
}
func (ticketServicioRepositoryStub) ListByCliente(_ context.Context, _ uuid.UUID, _ *int, _, _ int) ([]*atencionseguimiento.TicketServicio, int, error) {
	return nil, 0, nil
}
func (ticketServicioRepositoryStub) UpdateEstatus(_ context.Context, _ uuid.UUID, _, _ int) error {
	return nil
}
func (ticketServicioRepositoryStub) GetStatsByEmpresa(_ context.Context, _ uuid.UUID) (*atencionseguimiento.TicketStats, error) {
	return nil, nil
}
func (ticketServicioRepositoryStub) CreateRespuesta(_ context.Context, _ *atencionseguimiento.RespuestaServicio) error {
	return nil
}
func (ticketServicioRepositoryStub) ListRespuestas(_ context.Context, _ uuid.UUID) ([]*atencionseguimiento.RespuestaServicio, error) {
	return nil, nil
}

type incidenciaSupervisorRepositoryStub struct{}

func (incidenciaSupervisorRepositoryStub) Create(_ context.Context, _ *atencionseguimiento.IncidenciaSupervisor) error {
	return nil
}
func (incidenciaSupervisorRepositoryStub) ListByEmpresa(_ context.Context, _ uuid.UUID, _, _ int) ([]*atencionseguimiento.IncidenciaSupervisor, int, error) {
	return nil, 0, nil
}
