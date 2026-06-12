// Package jobs_test — black-box tests for InactivaEmpleadoJob
// (PR 5B-ii task 5.5 — Gherkin scenario 2 happy path + edge cases).
//
// The job's data-access surface is mocked through the typed
// EmployeeRepository + EmployeeTxFactory contracts. No real
// Postgres, no real Parametrizacion service, no real outbox.
package jobs_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"axis-flow-back/internal/scheduler/events"
	"axis-flow-back/internal/scheduler/jobs"
	"axis-flow-back/internal/scheduler/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

type mockEmpleadoRepo struct {
	mu sync.Mutex

	empresas   []int64
	empresasErr error

	empleados        map[int64][]int64 // empresaID -> empleadoIDs
	empleadosErr     error

	deactivateResult bool
	deactivateErr    error
	deactivateCalls  []int64

	revokeErr error
}

func (m *mockEmpleadoRepo) EmpresasWithUnjustifiedStreaks(_ context.Context, _ int) ([]int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.empresas, m.empresasErr
}

func (m *mockEmpleadoRepo) EmpleadosWithUnjustifiedStreakAtLeast(_ context.Context, empresaID int64, _ int) ([]int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.empleadosErr != nil {
		return nil, m.empleadosErr
	}
	return m.empleados[empresaID], nil
}

func (m *mockEmpleadoRepo) DeactivateEmpleadoTx(_ context.Context, _ jobs.Tx, empleadoID int64, _ time.Time) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deactivateCalls = append(m.deactivateCalls, empleadoID)
	return m.deactivateResult, m.deactivateErr
}

func (m *mockEmpleadoRepo) RevokeIdentityTx(_ context.Context, _ jobs.Tx, _ int64, _ time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.revokeErr
}

type mockTx struct {
	mu             sync.Mutex
	commitCalls    int
	rollbackCalls  int
	commitErr      error
	rollbackErr    error
}

func (m *mockTx) Commit(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.commitCalls++
	return m.commitErr
}

func (m *mockTx) Rollback(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rollbackCalls++
	return m.rollbackErr
}

type mockTxFactory struct {
	tx        jobs.Tx
	emps      jobs.EmployeeRepository
	beginErr  error
	beginCalls int
}

func (m *mockTxFactory) BeginTx(_ context.Context) (jobs.Tx, jobs.EmployeeRepository, error) {
	m.beginCalls++
	if m.beginErr != nil {
		return nil, nil, m.beginErr
	}
	return m.tx, m.emps, nil
}

type mockParametrizacion struct {
	threshold int
	err       error
}

func (m *mockParametrizacion) GetInactivityThreshold(_ context.Context, _ int64) (int, error) {
	return m.threshold, m.err
}

type outboxInsert struct {
	EventType   string
	AggregateID int64
	Payload     []byte
}

type mockOutbox struct {
	mu       sync.Mutex
	inserts  []outboxInsert
	insertErr error
}

func (m *mockOutbox) Insert(_ context.Context, _ jobs.Tx, eventType string, aggregateID int64, payload []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.insertErr != nil {
		return m.insertErr
	}
	cp := make([]byte, len(payload))
	copy(cp, payload)
	m.inserts = append(m.inserts, outboxInsert{
		EventType:   eventType,
		AggregateID: aggregateID,
		Payload:     cp,
	})
	return nil
}

type mockInactivaPublisher struct {
	mu            sync.Mutex
	empleadoInact []events.EmpleadoInactivadoEvent
}

func (m *mockInactivaPublisher) PublishPreShiftAlert(_ context.Context, _ events.PreShiftAlertEvent) error {
	return nil
}

func (m *mockInactivaPublisher) PublishEmpleadoInactivado(_ context.Context, evt events.EmpleadoInactivadoEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.empleadoInact = append(m.empleadoInact, evt)
	return nil
}

// ---------------------------------------------------------------------------
// Job fixture
// ---------------------------------------------------------------------------

func newInactivaJob(
	t *testing.T,
	emps *mockEmpleadoRepo,
	tx *mockTx,
	txFactory *mockTxFactory,
	param *mockParametrizacion,
	outbox *mockOutbox,
	pub *mockInactivaPublisher,
) *jobs.InactivaEmpleadoJob {
	t.Helper()
	// The factory's second return value (EmployeeRepository) is the
	// same `emps` instance, so in-tx calls route to the same mock.
	txFactory.tx = tx
	txFactory.emps = emps

	job, err := jobs.NewInactivaEmpleadoJob(jobs.InactivaEmpleadoJobDeps{
		Emps:           emps,
		TxFactory:      txFactory,
		Parametrizacion: param,
		Outbox:         outbox,
		EventPublisher: pub,
		Logger:         newTestLogger(),
	})
	require.NoError(t, err)
	return job
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestInactivaEmpleadoJob_HappyPath_Gherkin2 — Gherkin scenario 2:
// empresa 12 with threshold 5, empleado 42 with the streak.
// Expect: BeginTx, DeactivateEmpleadoTx (returns true), RevokeIdentityTx,
// Outbox.Insert with event "EmpleadoInactivadoAutomaticamente", Commit.
func TestInactivaEmpleadoJob_HappyPath_Gherkin2(t *testing.T) {
	emps := &mockEmpleadoRepo{
		empresas: []int64{12},
		empleados: map[int64][]int64{12: {42}},
		deactivateResult: true,
	}
	tx := &mockTx{}
	txFactory := &mockTxFactory{}
	param := &mockParametrizacion{threshold: 5}
	outbox := &mockOutbox{}
	pub := &mockInactivaPublisher{}

	job := newInactivaJob(t, emps, tx, txFactory, param, outbox, pub)

	err := job.Run(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 1, txFactory.beginCalls, "expected one BeginTx for the single candidato")
	require.Equal(t, 1, tx.commitCalls, "Commit must be called once")
	assert.Equal(t, 0, tx.rollbackCalls, "Rollback must NOT be called on success")
	assert.Equal(t, []int64{42}, emps.deactivateCalls)
	assert.NotEmpty(t, outbox.inserts, "expected one outbox insert")

	ins := outbox.inserts[0]
	assert.Equal(t, "EmpleadoInactivadoAutomaticamente", ins.EventType)
	assert.Equal(t, int64(42), ins.AggregateID)
	assert.NotEmpty(t, ins.Payload, "payload must be JSON-encoded event body")
}

// TestInactivaEmpleadoJob_AlreadyInactive_SkipsEnvelop — DeactivateEmpleadoTx
// returns (false, nil) (the WHERE estatus <> 4 guard matched 0 rows).
// Expect: NO Commit (the function short-circuits before commit; defer
// runs Rollback). No RevokeIdentityTx, no Outbox.Insert.
func TestInactivaEmpleadoJob_AlreadyInactive_SkipsEnvelop(t *testing.T) {
	emps := &mockEmpleadoRepo{
		empresas:         []int64{12},
		empleados:        map[int64][]int64{12: {42}},
		deactivateResult: false, // already inactive
	}
	tx := &mockTx{}
	txFactory := &mockTxFactory{}
	param := &mockParametrizacion{threshold: 5}
	outbox := &mockOutbox{}
	pub := &mockInactivaPublisher{}

	job := newInactivaJob(t, emps, tx, txFactory, param, outbox, pub)

	err := job.Run(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 0, tx.commitCalls, "Commit must NOT be called when the deactivate short-circuits")
	assert.Equal(t, 1, tx.rollbackCalls, "Rollback runs via defer on short-circuit")
	assert.Empty(t, outbox.inserts, "no outbox insert expected for already-inactive employee")
}

// TestInactivaEmpleadoJob_Parametrizacion4xx_SkipsEmpresa — the
// Parametrizacion client returns ErrParametrizacionBadRequest. The
// empresa is skipped; no BeginTx is called; no outbox inserts.
func TestInactivaEmpleadoJob_Parametrizacion4xx_SkipsEmpresa(t *testing.T) {
	emps := &mockEmpleadoRepo{
		empresas:  []int64{12},
		empleados: map[int64][]int64{12: {42}},
	}
	tx := &mockTx{}
	txFactory := &mockTxFactory{}
	param := &mockParametrizacion{err: service.ErrParametrizacionBadRequest}
	outbox := &mockOutbox{}
	pub := &mockInactivaPublisher{}

	job := newInactivaJob(t, emps, tx, txFactory, param, outbox, pub)

	err := job.Run(context.Background())
	require.NoError(t, err, "per-empresa 4xx is non-fatal")

	assert.Equal(t, 0, txFactory.beginCalls, "BeginTx must NOT be called when parametrizacion 4xx")
	assert.Empty(t, outbox.inserts)
	assert.Equal(t, 0, tx.commitCalls)
}

// TestInactivaEmpleadoJob_DeactivateError_RollsBack — DeactivateEmpleadoTx
// returns an error. Expect: Rollback called via defer, Commit NOT called,
// no outbox inserts.
func TestInactivaEmpleadoJob_DeactivateError_RollsBack(t *testing.T) {
	emps := &mockEmpleadoRepo{
		empresas:         []int64{12},
		empleados:        map[int64][]int64{12: {42}},
		deactivateResult: false,
		deactivateErr:    errors.New("db connection lost"),
	}
	tx := &mockTx{}
	txFactory := &mockTxFactory{}
	param := &mockParametrizacion{threshold: 5}
	outbox := &mockOutbox{}
	pub := &mockInactivaPublisher{}

	job := newInactivaJob(t, emps, tx, txFactory, param, outbox, pub)

	err := job.Run(context.Background())
	require.NoError(t, err, "per-empleado errors are logged and counted, not surfaced")

	assert.Equal(t, 1, tx.rollbackCalls, "Rollback must be called via defer on deactivate error")
	assert.Equal(t, 0, tx.commitCalls, "Commit must NOT be called on rollback")
	assert.Empty(t, outbox.inserts)
}

// TestInactivaEmpleadoJob_OutboxInsertError_RollsBack — the Outbox
// Insert fails. Expect: rollback, no commit, the failure is logged
// but not surfaced to the caller.
func TestInactivaEmpleadoJob_OutboxInsertError_RollsBack(t *testing.T) {
	emps := &mockEmpleadoRepo{
		empresas:         []int64{12},
		empleados:        map[int64][]int64{12: {42}},
		deactivateResult: true,
	}
	tx := &mockTx{}
	txFactory := &mockTxFactory{}
	param := &mockParametrizacion{threshold: 5}
	outbox := &mockOutbox{insertErr: errors.New("outbox table missing")}
	pub := &mockInactivaPublisher{}

	job := newInactivaJob(t, emps, tx, txFactory, param, outbox, pub)

	err := job.Run(context.Background())
	require.NoError(t, err, "outbox errors are non-fatal — counted via IncJobFailed")

	assert.Equal(t, 1, tx.rollbackCalls, "rollback expected when outbox insert fails")
	assert.Equal(t, 0, tx.commitCalls)
}
