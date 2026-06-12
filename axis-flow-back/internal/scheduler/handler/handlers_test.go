// Package handler_test — handlers_test.go is the in-process REST
// surface test for the four admin endpoints exposed by the
// Scheduler Service: GET /api/v1/scheduler/jobs, PATCH
// /api/v1/scheduler/jobs/{job_id}/pause, POST
// /api/v1/scheduler/jobs/{job_id}/trigger, GET
// /api/v1/scheduler/executions. The HealthHandler is out of scope
// for this PR. Tests are black-box (package handler_test) and
// follow the testify/mock pattern used by
// tests/unit/auth_service_test.go and service/cron_runner_test.go.
// The handler package exposes a small Runner interface (see
// runner.go) so the cron runner can be swapped for a testify/mock
// fake; production wiring is unaffected because *service.CronRunner
// satisfies Runner structurally. JWT middleware is NOT applied — the
// handlers don't check auth, the middleware is mounted at the
// route level in PR 4B.
package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"axis-flow-back/internal/scheduler"
	"axis-flow-back/internal/scheduler/handler"
	"axis-flow-back/internal/scheduler/service"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// --- mocks (testify/mock) -------------------------------------------------

type mockJobRepo struct{ mock.Mock }

func (m *mockJobRepo) GetByKey(ctx context.Context, key string) (scheduler.Job, error) {
	a := m.Called(ctx, key)
	return a.Get(0).(scheduler.Job), a.Error(1)
}
func (m *mockJobRepo) GetByID(ctx context.Context, id uuid.UUID) (scheduler.Job, error) {
	a := m.Called(ctx, id)
	return a.Get(0).(scheduler.Job), a.Error(1)
}
func (m *mockJobRepo) List(ctx context.Context, f scheduler.JobListFilter) ([]scheduler.Job, int, error) {
	a := m.Called(ctx, f)
	if v := a.Get(0); v != nil {
		return v.([]scheduler.Job), a.Int(1), a.Error(2)
	}
	return nil, a.Int(1), a.Error(2)
}
func (m *mockJobRepo) ListActive(ctx context.Context) ([]scheduler.Job, error) {
	a := m.Called(ctx)
	if v := a.Get(0); v != nil {
		return v.([]scheduler.Job), a.Error(1)
	}
	return nil, a.Error(1)
}
func (m *mockJobRepo) Create(ctx context.Context, j scheduler.Job) (scheduler.Job, error) {
	a := m.Called(ctx, j)
	return a.Get(0).(scheduler.Job), a.Error(1)
}
func (m *mockJobRepo) UpdatePause(ctx context.Context, id uuid.UUID, paused bool) (scheduler.Job, error) {
	a := m.Called(ctx, id, paused)
	return a.Get(0).(scheduler.Job), a.Error(1)
}
func (m *mockJobRepo) UpdateLastRun(ctx context.Context, id uuid.UUID, last, next *time.Time) (scheduler.Job, error) {
	a := m.Called(ctx, id, last, next)
	return a.Get(0).(scheduler.Job), a.Error(1)
}

type mockExecutionRepo struct{ mock.Mock }

func (m *mockExecutionRepo) Insert(ctx context.Context, e scheduler.Execution) (scheduler.Execution, error) {
	a := m.Called(ctx, e)
	return a.Get(0).(scheduler.Execution), a.Error(1)
}
func (m *mockExecutionRepo) Finalize(ctx context.Context, id uuid.UUID, st scheduler.ExecutionStatus, log *string) (scheduler.Execution, error) {
	a := m.Called(ctx, id, st, log)
	return a.Get(0).(scheduler.Execution), a.Error(1)
}
func (m *mockExecutionRepo) List(ctx context.Context, f scheduler.ExecutionListFilter) ([]scheduler.Execution, int, error) {
	a := m.Called(ctx, f)
	if v := a.Get(0); v != nil {
		return v.([]scheduler.Execution), a.Int(1), a.Error(2)
	}
	return nil, a.Int(1), a.Error(2)
}

type mockRunner struct{ mock.Mock }

func (m *mockRunner) Pause(ctx context.Context, jobKey string) error {
	return m.Called(ctx, jobKey).Error(0)
}
func (m *mockRunner) Resume(ctx context.Context, jobKey string) error {
	return m.Called(ctx, jobKey).Error(0)
}
func (m *mockRunner) TriggerNow(ctx context.Context, jobKey string) error {
	return m.Called(ctx, jobKey).Error(0)
}

// --- envelope types reused across tests -----------------------------------

type listResp struct {
	Data       []map[string]any `json:"data"`
	Pagination *struct {
		Page, Limit, Total, Pages int
	} `json:"pagination"`
}

type errorEnv struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// --- helpers --------------------------------------------------------------

func newTestRouter(jr *mockJobRepo, er *mockExecutionRepo, run *mockRunner) http.Handler {
	jobsH := handler.NewJobsHandler(handler.JobsHandlerDeps{JobRepo: jr, Runner: run, Logger: nil})
	triggerH := handler.NewTriggerHandler(handler.TriggerHandlerDeps{JobRepo: jr, Runner: run, Logger: nil})
	execsH := handler.NewExecutionsHandler(handler.ExecutionsHandlerDeps{ExecRepo: er, JobRepo: jr, Logger: nil})
	r := chi.NewRouter()
	r.Route("/api/v1/scheduler", func(r chi.Router) {
		r.Get("/jobs", jobsH.List)
		r.Patch("/jobs/{job_id}/pause", jobsH.Pause)
		r.Post("/jobs/{job_id}/trigger", triggerH.Trigger)
		r.Get("/executions", execsH.List)
	})
	return r
}

func newTestJob(jobKey string) (scheduler.Job, uuid.UUID) {
	jobID := uuid.New()
	expr := "*/5 * * * *"
	return scheduler.Job{
		JobID: jobID, JobKey: jobKey, CronExpression: &expr,
		JobClass: "test.Job", Module: "tests", IsActive: true,
	}, jobID
}

func strp(s string) *string { return &s }

func doRequest(t *testing.T, h http.Handler, method, path string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBuffer(body))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

// --- JobsHandler.List ------------------------------------------------------

// TestJobsHandler_List_OK — 2 mocked jobs, total=2 → pages=ceil(2/20)=1.
func TestJobsHandler_List_OK(t *testing.T) {
	t.Parallel()
	jobs := []scheduler.Job{
		{JobID: uuid.New(), JobKey: "a_job", CronExpression: strp("*/5 * * * *"), JobClass: "x", Module: "m", IsActive: true},
		{JobID: uuid.New(), JobKey: "b_job", CronExpression: strp("*/10 * * * *"), JobClass: "x", Module: "m", IsActive: false},
	}
	jr := &mockJobRepo{}; er := &mockExecutionRepo{}; run := &mockRunner{}
	jr.On("List", mock.Anything, mock.MatchedBy(func(f scheduler.JobListFilter) bool {
		return f.Limit == 20 && f.Offset == 0
	})).Return(jobs, 2, nil).Once()

	w := doRequest(t, newTestRouter(jr, er, run), "GET", "/api/v1/scheduler/jobs?page=1&limit=20", nil)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
	var resp listResp
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Len(t, resp.Data, 2)
	require.NotNil(t, resp.Pagination)
	assert.Equal(t, 1, resp.Pagination.Page)
	assert.Equal(t, 20, resp.Pagination.Limit)
	assert.Equal(t, 2, resp.Pagination.Total)
	assert.Equal(t, 1, resp.Pagination.Pages)
	jr.AssertExpectations(t)
}

// TestJobsHandler_List_DefaultsPagination — no query string → defaults page=1, limit=20.
func TestJobsHandler_List_DefaultsPagination(t *testing.T) {
	t.Parallel()
	jr := &mockJobRepo{}; er := &mockExecutionRepo{}; run := &mockRunner{}
	jr.On("List", mock.Anything, mock.MatchedBy(func(f scheduler.JobListFilter) bool {
		return f.Limit == 20 && f.Offset == 0
	})).Return([]scheduler.Job{}, 0, nil).Once()

	w := doRequest(t, newTestRouter(jr, er, run), "GET", "/api/v1/scheduler/jobs", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var resp listResp
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotNil(t, resp.Pagination)
	assert.Equal(t, 1, resp.Pagination.Page)
	assert.Equal(t, 20, resp.Pagination.Limit)
	assert.Equal(t, 0, resp.Pagination.Total)
	assert.Equal(t, 0, resp.Pagination.Pages, "pages=0 when total=0 (see writeList contract)")
	jr.AssertExpectations(t)
}

// TestJobsHandler_List_InvalidLimit — limit=999 rejected with 400 (openapi cap=100).
func TestJobsHandler_List_InvalidLimit(t *testing.T) {
	t.Parallel()
	jr := &mockJobRepo{}; er := &mockExecutionRepo{}; run := &mockRunner{}
	w := doRequest(t, newTestRouter(jr, er, run), "GET", "/api/v1/scheduler/jobs?limit=999", nil)
	require.Equal(t, http.StatusBadRequest, w.Code)
	var env errorEnv
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
	assert.Equal(t, "VALIDATION_ERROR", env.Error.Code)
	assert.Contains(t, env.Error.Message, "limit")
}

// --- JobsHandler.Pause -----------------------------------------------------

// TestJobsHandler_Pause_OK — PATCH paused=true → 200 with status="PAUSED".
func TestJobsHandler_Pause_OK(t *testing.T) {
	t.Parallel()
	job, jobID := newTestJob("j1")
	paused := job
	paused.IsActive = false

	jr := &mockJobRepo{}; er := &mockExecutionRepo{}; run := &mockRunner{}
	jr.On("GetByKey", mock.Anything, "j1").Return(job, nil).Once()
	run.On("Pause", mock.Anything, "j1").Return(nil).Once()
	jr.On("GetByID", mock.Anything, jobID).Return(paused, nil).Once()

	w := doRequest(t, newTestRouter(jr, er, run), "PATCH", "/api/v1/scheduler/jobs/j1/pause",
		[]byte(`{"paused": true}`))
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var resp struct {
		Data struct {
			JobID  string `json:"job_id"`
			Status string `json:"status"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "j1", resp.Data.JobID, "job_id must round-trip the public job_key")
	assert.Equal(t, "PAUSED", resp.Data.Status)
	jr.AssertExpectations(t)
	run.AssertExpectations(t)
}

// TestJobsHandler_Pause_NotFound — runner.Pause returns ErrNoHandler → 404.
func TestJobsHandler_Pause_NotFound(t *testing.T) {
	t.Parallel()
	job, _ := newTestJob("j1")
	jr := &mockJobRepo{}; er := &mockExecutionRepo{}; run := &mockRunner{}
	jr.On("GetByKey", mock.Anything, "j1").Return(job, nil).Once()
	run.On("Pause", mock.Anything, "j1").Return(service.ErrNoHandler).Once()

	w := doRequest(t, newTestRouter(jr, er, run), "PATCH", "/api/v1/scheduler/jobs/j1/pause",
		[]byte(`{"paused": true}`))
	require.Equal(t, http.StatusNotFound, w.Code)
	var env errorEnv
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
	assert.Equal(t, "NOT_FOUND", env.Error.Code)
}

// TestJobsHandler_Pause_BadBody — string in "paused" slot → 400. The
// handler resolves the job first; runner must NOT be called.
func TestJobsHandler_Pause_BadBody(t *testing.T) {
	t.Parallel()
	job, _ := newTestJob("j1")
	jr := &mockJobRepo{}; er := &mockExecutionRepo{}; run := &mockRunner{}
	jr.On("GetByKey", mock.Anything, "j1").Return(job, nil).Once()

	w := doRequest(t, newTestRouter(jr, er, run), "PATCH", "/api/v1/scheduler/jobs/j1/pause",
		[]byte(`{"paused": "not-a-bool"}`))
	require.Equal(t, http.StatusBadRequest, w.Code)
	var env errorEnv
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
	assert.Equal(t, "VALIDATION_ERROR", env.Error.Code)
	run.AssertExpectations(t)
}

// --- TriggerHandler.Trigger ------------------------------------------------

// TestTriggerHandler_Trigger_OK — 202 Accepted with status="TRIGGERED".
// (Openapi + trigger_handler.go agree on "TRIGGERED", not "TRIGGER_QUEUED".)
func TestTriggerHandler_Trigger_OK(t *testing.T) {
	t.Parallel()
	job, _ := newTestJob("j1")
	jr := &mockJobRepo{}; er := &mockExecutionRepo{}; run := &mockRunner{}
	jr.On("GetByKey", mock.Anything, "j1").Return(job, nil).Once()
	run.On("TriggerNow", mock.Anything, "j1").Return(nil).Once()

	w := doRequest(t, newTestRouter(jr, er, run), "POST", "/api/v1/scheduler/jobs/j1/trigger", nil)
	require.Equal(t, http.StatusAccepted, w.Code, "body: %s", w.Body.String())

	var resp struct {
		Data struct {
			JobID       string    `json:"job_id"`
			TriggeredAt time.Time `json:"triggered_at"`
			Status      string    `json:"status"`
			Message     string    `json:"message"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "j1", resp.Data.JobID)
	assert.Equal(t, "TRIGGERED", resp.Data.Status)
	assert.Equal(t, "Job triggered successfully in the background.", resp.Data.Message)
	assert.False(t, resp.Data.TriggeredAt.IsZero())
	jr.AssertExpectations(t)
	run.AssertExpectations(t)
}

// TestTriggerHandler_Trigger_NotFound — repo returns ErrNotFound → 404.
func TestTriggerHandler_Trigger_NotFound(t *testing.T) {
	t.Parallel()
	jr := &mockJobRepo{}; er := &mockExecutionRepo{}; run := &mockRunner{}
	jr.On("GetByKey", mock.Anything, "missing").Return(scheduler.Job{}, scheduler.ErrNotFound).Once()

	w := doRequest(t, newTestRouter(jr, er, run), "POST", "/api/v1/scheduler/jobs/missing/trigger", nil)
	require.Equal(t, http.StatusNotFound, w.Code)
	var env errorEnv
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
	assert.Equal(t, "NOT_FOUND", env.Error.Code)
	run.AssertExpectations(t)
}

// --- ExecutionsHandler.List ------------------------------------------------

// TestExecutionsHandler_List_OK — job_id+status filters; response uses the
// public job_key (not the UUID) in the data array.
func TestExecutionsHandler_List_OK(t *testing.T) {
	t.Parallel()
	job, jobID := newTestJob("j1")
	startedAt := time.Now().UTC().Add(-time.Hour)
	errMsg := "boom"
	execs := []scheduler.Execution{
		{ExecutionID: uuid.New(), JobID: jobID, Status: scheduler.StatusFailed, StartedAt: startedAt, ErrorLog: &errMsg},
		{ExecutionID: uuid.New(), JobID: jobID, Status: scheduler.StatusFailed, StartedAt: startedAt.Add(time.Minute)},
	}

	jr := &mockJobRepo{}; er := &mockExecutionRepo{}; run := &mockRunner{}
	jr.On("GetByKey", mock.Anything, "j1").Return(job, nil).Once()
	jr.On("ListActive", mock.Anything).Return([]scheduler.Job{job}, nil).Once()
	er.On("List", mock.Anything, mock.MatchedBy(func(f scheduler.ExecutionListFilter) bool {
		return f.JobID != nil && *f.JobID == jobID &&
			f.Status != nil && *f.Status == scheduler.StatusFailed &&
			f.Limit == 20 && f.Offset == 0
	})).Return(execs, 2, nil).Once()

	w := doRequest(t, newTestRouter(jr, er, run), "GET",
		"/api/v1/scheduler/executions?job_id=j1&status=FAILED&page=1&limit=20", nil)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var resp struct {
		Data []struct {
			ExecutionID string `json:"execution_id"`
			JobID       string `json:"job_id"`
			Status      string `json:"status"`
		} `json:"data"`
		Pagination *struct{ Page, Limit, Total, Pages int } `json:"pagination"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Len(t, resp.Data, 2)
	for i, d := range resp.Data {
		assert.Equal(t, execs[i].ExecutionID.String(), d.ExecutionID)
		assert.Equal(t, "j1", d.JobID, "job_id must be the public job_key, not the UUID")
		assert.Equal(t, "FAILED", d.Status)
	}
	require.NotNil(t, resp.Pagination)
	assert.Equal(t, 1, resp.Pagination.Page)
	assert.Equal(t, 20, resp.Pagination.Limit)
	assert.Equal(t, 2, resp.Pagination.Total)
	assert.Equal(t, 1, resp.Pagination.Pages)
	jr.AssertExpectations(t)
	er.AssertExpectations(t)
}

// TestExecutionsHandler_List_InvalidStatus — status=BANANA → 400; no repo call.
func TestExecutionsHandler_List_InvalidStatus(t *testing.T) {
	t.Parallel()
	jr := &mockJobRepo{}; er := &mockExecutionRepo{}; run := &mockRunner{}
	w := doRequest(t, newTestRouter(jr, er, run), "GET", "/api/v1/scheduler/executions?status=BANANA", nil)
	require.Equal(t, http.StatusBadRequest, w.Code)
	var env errorEnv
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
	assert.Equal(t, "VALIDATION_ERROR", env.Error.Code)
	assert.Contains(t, env.Error.Message, "status")
	jr.AssertExpectations(t)
	er.AssertExpectations(t)
}

// TestExecutionsHandler_List_OmitsErrorLog_OnSuccess — exception_message
// is only present on FAILED (per openapi). For SUCCESS the key is
// dropped via json:",omitempty" on a nil *string.
func TestExecutionsHandler_List_OmitsErrorLog_OnSuccess(t *testing.T) {
	t.Parallel()
	job, jobID := newTestJob("j1")
	successExec := scheduler.Execution{
		ExecutionID: uuid.New(), JobID: jobID, Status: scheduler.StatusSuccess,
		StartedAt: time.Now().UTC().Add(-time.Hour),
	}

	jr := &mockJobRepo{}; er := &mockExecutionRepo{}; run := &mockRunner{}
	jr.On("GetByKey", mock.Anything, "j1").Return(job, nil).Once()
	jr.On("ListActive", mock.Anything).Return([]scheduler.Job{job}, nil).Once()
	er.On("List", mock.Anything, mock.Anything).Return([]scheduler.Execution{successExec}, 1, nil).Once()

	w := doRequest(t, newTestRouter(jr, er, run), "GET", "/api/v1/scheduler/executions?job_id=j1", nil)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	// Decode into a permissive map so we can probe for the
	// presence/absence of "exception_message"/"traceback" by key.
	var raw struct {
		Data []map[string]json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &raw))
	require.Len(t, raw.Data, 1)
	item := raw.Data[0]
	_, hasException := item["exception_message"]
	_, hasTraceback := item["traceback"]
	assert.False(t, hasException,
		"exception_message MUST be absent on a SUCCESS execution: %s", w.Body.String())
	assert.False(t, hasTraceback,
		"traceback MUST be absent on a SUCCESS execution: %s", w.Body.String())
	assert.Equal(t, `"SUCCESS"`, string(item["status"]))
	jr.AssertExpectations(t)
	er.AssertExpectations(t)
}
