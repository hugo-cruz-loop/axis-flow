// Package jobs_test — black-box tests for NotificacionesEnTiempoRealJob
// (PR 5B-ii task 5.4 — Gherkin scenario 1 happy path + edge cases).
//
// The job's data-access surface is mocked through the typed
// ShiftRepository contract (see shift_repository.go); the FCM client
// and event publisher are also mocked to keep the test self-contained.
// No real Postgres, no real FCM, no real Notificaciones service.
package jobs_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"axis-flow-back/internal/scheduler/events"
	"axis-flow-back/internal/scheduler/jobs"
	"axis-flow-back/internal/scheduler/service"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

type sentCall struct {
	Token   string
	Payload service.NotificationPayload
}

type mockShiftRepo struct {
	mu sync.Mutex

	upcoming       []jobs.UpcomingShift
	upcomingErr    error
	upcomingCalled int

	devices        map[int64][]jobs.Device
	devicesErr     error
	devicesCalled  int

	deactivateCalls []uuid.UUID
	deactivateErr   error
}

func (m *mockShiftRepo) UpcomingShiftsInWindow(_ context.Context, _, _ time.Time) ([]jobs.UpcomingShift, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.upcomingCalled++
	return m.upcoming, m.upcomingErr
}

func (m *mockShiftRepo) ActiveDevicesForEmpleado(_ context.Context, empleadoID int64) ([]jobs.Device, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.devicesCalled++
	if m.devicesErr != nil {
		return nil, m.devicesErr
	}
	return m.devices[empleadoID], nil
}

func (m *mockShiftRepo) DeactivateDevice(_ context.Context, id uuid.UUID, _ time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deactivateCalls = append(m.deactivateCalls, id)
	return m.deactivateErr
}

type mockFCM struct {
	mu   sync.Mutex
	sent []sentCall

	sendFunc func(ctx context.Context, token string, p service.NotificationPayload) (string, error)
}

func (m *mockFCM) Send(ctx context.Context, token string, p service.NotificationPayload) (string, error) {
	m.mu.Lock()
	m.sent = append(m.sent, sentCall{Token: token, Payload: p})
	m.mu.Unlock()
	if m.sendFunc != nil {
		return m.sendFunc(ctx, token, p)
	}
	return "msg-id", nil
}

func (m *mockFCM) Ping(_ context.Context) error { return nil }

type mockPreShiftPublisher struct {
	mu            sync.Mutex
	preShift      []events.PreShiftAlertEvent
	preShiftErr   error
}

func (m *mockPreShiftPublisher) PublishPreShiftAlert(_ context.Context, evt events.PreShiftAlertEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.preShift = append(m.preShift, evt)
	return m.preShiftErr
}

func (m *mockPreShiftPublisher) PublishEmpleadoInactivado(_ context.Context, _ events.EmpleadoInactivadoEvent) error {
	return nil
}

// ---------------------------------------------------------------------------
// Test fixture helpers
// ---------------------------------------------------------------------------

func newTestLogger() *service.Logger {
	return service.NewLoggerWithWriter(io.Discard, "logfmt", slog.LevelError)
}

func newJob(t *testing.T, repo *mockShiftRepo, fcm *mockFCM, pub *mockPreShiftPublisher) *jobs.NotificacionesEnTiempoRealJob {
	t.Helper()
	job, err := jobs.NewNotificacionesEnTiempoRealJob(jobs.NotificacionesJobDeps{
		Shifts:         repo,
		FCM:            fcm,
		EventPublisher: pub,
		Logger:         newTestLogger(),
	})
	require.NoError(t, err)
	return job
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestNotificacionesJob_HappyPath_Gherkin1 — Gherkin scenario 1:
// one shift starting in ~15 min for empleado 42, one active device
// with FCM token. Expect: 1 FCM send, 1 PreShiftAlert event.
func TestNotificacionesJob_HappyPath_Gherkin1(t *testing.T) {
	shiftID := uuid.New()
	deviceID := uuid.New()

	repo := &mockShiftRepo{
		upcoming: []jobs.UpcomingShift{
			{
				ShiftID:    shiftID,
				EmpleadoID: 42,
				TipoEvento: "entrada",
				HoraInicio: time.Now().UTC().Add(14 * time.Minute),
			},
		},
		devices: map[int64][]jobs.Device{
			42: {{ID: deviceID, FCMToken: "fcm-test-001", IsActive: true}},
		},
	}
	fcm := &mockFCM{}
	pub := &mockPreShiftPublisher{}
	job := newJob(t, repo, fcm, pub)

	err := job.Run(context.Background())
	require.NoError(t, err)

	require.Len(t, fcm.sent, 1, "expected exactly one FCM send")
	assert.Equal(t, "fcm-test-001", fcm.sent[0].Token)
	assert.Equal(t, "Recuerda que tu inicio de turno es pronto", fcm.sent[0].Payload.Body)
	assert.Equal(t, "entrada", fcm.sent[0].Payload.Data["tipo_evento"])
	assert.Equal(t, "42", fcm.sent[0].Payload.Data["empleado_id"])
	assert.Equal(t, shiftID.String(), fcm.sent[0].Payload.Data["shift_id"])

	require.Len(t, pub.preShift, 1, "expected exactly one PreShiftAlert event")
	assert.Equal(t, int64(42), pub.preShift[0].EmpleadoID)
	assert.Equal(t, "entrada", pub.preShift[0].TipoEvento)

	assert.Empty(t, repo.deactivateCalls, "no device deactivation expected on success path")
}

// TestNotificacionesJob_NoShifts_NoOp — empty 15-min window: no FCM
// sends, no events, no deactivations.
func TestNotificacionesJob_NoShifts_NoOp(t *testing.T) {
	repo := &mockShiftRepo{upcoming: nil}
	fcm := &mockFCM{}
	pub := &mockPreShiftPublisher{}
	job := newJob(t, repo, fcm, pub)

	err := job.Run(context.Background())
	require.NoError(t, err)

	assert.Empty(t, fcm.sent)
	assert.Empty(t, pub.preShift)
	assert.Empty(t, repo.deactivateCalls)
}

// TestNotificacionesJob_FCMPermanentFailure_DeactivatesDevice — FCM
// returns ErrFCMPermanentFailure; the job must call
// ShiftRepository.DeactivateDevice with the device id.
func TestNotificacionesJob_FCMPermanentFailure_DeactivatesDevice(t *testing.T) {
	shiftID := uuid.New()
	deviceID := uuid.New()

	repo := &mockShiftRepo{
		upcoming: []jobs.UpcomingShift{
			{ShiftID: shiftID, EmpleadoID: 42, TipoEvento: "entrada",
				HoraInicio: time.Now().UTC().Add(14 * time.Minute)},
		},
		devices: map[int64][]jobs.Device{
			42: {{ID: deviceID, FCMToken: "fcm-dead-token", IsActive: true}},
		},
	}
	fcm := &mockFCM{
		sendFunc: func(_ context.Context, _ string, _ service.NotificationPayload) (string, error) {
			return "", service.ErrFCMPermanentFailure
		},
	}
	pub := &mockPreShiftPublisher{}
	job := newJob(t, repo, fcm, pub)

	err := job.Run(context.Background())
	require.NoError(t, err, "permanent FCM failure is a per-device lifecycle event, not a job failure")

	require.Len(t, fcm.sent, 1)
	require.Len(t, repo.deactivateCalls, 1, "expected DeactivateDevice to be called once")
	assert.Equal(t, deviceID, repo.deactivateCalls[0])
}

// TestNotificacionesJob_UpcomingQueryError_FailsRun — the repo's
// UpcomingShiftsInWindow returns an error; the job propagates it.
func TestNotificacionesJob_UpcomingQueryError_FailsRun(t *testing.T) {
	repo := &mockShiftRepo{upcomingErr: errors.New("db down")}
	fcm := &mockFCM{}
	pub := &mockPreShiftPublisher{}
	job := newJob(t, repo, fcm, pub)

	err := job.Run(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "query upcoming shifts")
	assert.Empty(t, fcm.sent)
}

// TestNotificacionesJob_DedupCache_PreventsDoubleSend — calling Run
// twice within the 30s dedup window must result in only ONE FCM send.
// This validates the in-memory dedup cache works end-to-end via Run.
func TestNotificacionesJob_DedupCache_PreventsDoubleSend(t *testing.T) {
	shiftID := uuid.New()
	deviceID := uuid.New()

	repo := &mockShiftRepo{
		upcoming: []jobs.UpcomingShift{
			{ShiftID: shiftID, EmpleadoID: 42, TipoEvento: "entrada",
				HoraInicio: time.Now().UTC().Add(14 * time.Minute)},
		},
		devices: map[int64][]jobs.Device{
			42: {{ID: deviceID, FCMToken: "fcm-test-001", IsActive: true}},
		},
	}
	fcm := &mockFCM{}
	pub := &mockPreShiftPublisher{}
	job := newJob(t, repo, fcm, pub)

	require.NoError(t, job.Run(context.Background()))
	require.NoError(t, job.Run(context.Background())) // dedup should skip

	assert.Len(t, fcm.sent, 1, "second run within poll grace should be deduped")
	assert.Len(t, pub.preShift, 1)
}
