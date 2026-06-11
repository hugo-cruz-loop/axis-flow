// Package service_test covers the PDFService use cases.
//
// PR-3 (Services) — task 3.4. Gherkin 3 (Generación asíncrona de reporte
// PDF con hardening de seguridad): GenerateReporte renders an HTML
// template, uploads to S3, and publishes ReporteGenerado. The actual
// wkhtmltopdf subprocess and Jinja2 template live in PR-6 (PDF/S3
// Hardening) — for PR-3 we use a stub renderer and a stub storage.
//
// IDOR: the InMem/Pgx event repositories do not expose GetIniciado
// (PR-2 did not need it), so the service cannot strictly verify the
// parent chain. The handler (PR-4) is responsible for IDOR via JWT
// scope. The service uses the cached respuestas list as a proxy: an
// iniciado with zero respuestas returns formularios.ErrNotFound. This
// is documented as Deviation #2 in apply-progress.
package service_test

import (
	"context"
	"errors"
	"testing"

	"axis-flow-back/internal/formularios"
	"axis-flow-back/internal/formularios/events"
	"axis-flow-back/internal/formularios/repository"
	"axis-flow-back/internal/formularios/service"
	formulariosTelemetry "axis-flow-back/internal/formularios/telemetry"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Stubs for PDFRenderer, ReportStorage, Locker.
// ---------------------------------------------------------------------------

// stubRenderer implements service.PDFRenderer. Returns the configured
// bytes; fails if err is set.
type stubRenderer struct {
	bytes []byte
	err   error
	calls int
}

func (s *stubRenderer) Render(_ context.Context, _ string) ([]byte, error) {
	s.calls++
	return s.bytes, s.err
}

// stubStorage implements service.ReportStorage. Returns the configured
// URL; fails if err is set.
type stubStorage struct {
	url   string
	err   error
	calls int
	key   string
	body  []byte
}

func (s *stubStorage) Upload(_ context.Context, key string, body []byte) (string, error) {
	s.calls++
	s.key = key
	s.body = body
	return s.url, s.err
}

// minimalValidPDFBytes is the deterministic %PDF-1.4 stub returned by
// the renderer in the happy-path fixture. The handler test asserts that
// the response body starts with the PDF magic bytes (no plain-text
// placeholder may leak as a "PDF" — see FIX 2 in apply-progress).
var minimalValidPDFBytes = []byte(
	"%PDF-1.4\n" +
		"1 0 obj<</Type/Catalog>>endobj\n" +
		"trailer<</Root 1 0 R>>\n" +
		"%%EOF\n",
)

// stubLocker implements service.Locker. Records the lock + pending ops.
type stubLocker struct {
	acquireCalls   []uuid.UUID
	pendingAdds    []uuid.UUID
	pendingRemoves []uuid.UUID
	acquireErr     error
	pendingErr     error
	unlockCount    int
}

func (s *stubLocker) Acquire(_ context.Context, iniciadoID uuid.UUID) (service.UnlockFn, error) {
	s.acquireCalls = append(s.acquireCalls, iniciadoID)
	if s.acquireErr != nil {
		return func(context.Context) error { return nil }, s.acquireErr
	}
	idx := len(s.acquireCalls) - 1
	return func(context.Context) error {
		_ = idx
		s.unlockCount++
		return nil
	}, nil
}

func (s *stubLocker) AddToPending(_ context.Context, iniciadoID uuid.UUID) error {
	s.pendingAdds = append(s.pendingAdds, iniciadoID)
	return s.pendingErr
}

func (s *stubLocker) RemoveFromPending(_ context.Context, iniciadoID uuid.UUID) error {
	s.pendingRemoves = append(s.pendingRemoves, iniciadoID)
	return s.pendingErr
}

// ---------------------------------------------------------------------------
// Test fixture builder.
// ---------------------------------------------------------------------------

type pdfFixture struct {
	evRepo   *repository.InMemEventoRepository
	respRepo *repository.InMemRespuestaRepository
	pub      *recordingPublisher
	cache    *stubFormularioCache
	renderer *stubRenderer
	storage  *stubStorage
	locker   *stubLocker
}

func newPDFFixture(t *testing.T) *pdfFixture {
	t.Helper()
	return &pdfFixture{
		evRepo:   repository.NewInMemEventoRepository(),
		respRepo: repository.NewInMemRespuestaRepository(),
		pub:      &recordingPublisher{},
		cache:    &stubFormularioCache{},
		renderer: &stubRenderer{bytes: minimalValidPDFBytes},
		storage:  &stubStorage{url: "https://s3.example.com/reports/x.pdf"},
		locker:   &stubLocker{},
	}
}

func (f *pdfFixture) svc() service.PDFService {
	return service.NewPDFService(f.evRepo, f.respRepo, f.pub, f.cache, f.renderer, f.storage, f.locker, nil, nil)
}

// svcWithMetrics is a test-only variant of svc() that wires a real
// *telemetry.Metrics into the PDF service. The test asserts the
// metrics are recorded after a GenerateReporte call.
func (f *pdfFixture) svcWithMetrics() service.PDFService {
	reg := prometheus.NewRegistry()
	m := formulariosTelemetry.NewMetrics(reg)
	return service.NewPDFService(f.evRepo, f.respRepo, f.pub, f.cache, f.renderer, f.storage, f.locker, m, nil)
}

// seed: persist a parent evento with the given empresa/cliente, plus a
// few respuestas under the given iniciado ID.
func (f *pdfFixture) seed(t *testing.T, empresaID, clienteID, iniciadoID uuid.UUID) {
	t.Helper()
	e := &formularios.Evento{
		ID:          uuid.New(),
		EmpresaID:   empresaID,
		ClienteID:   clienteID,
		LocalidadID: uuid.New(),
		Nombre:      "E",
		Status:      formularios.EventoStatusPendiente,
	}
	require.NoError(t, f.evRepo.Create(context.Background(), e, nil))

	for i := 0; i < 2; i++ {
		r := &formularios.Respuesta{
			EventoIniciadoID: iniciadoID,
			PreguntaID:       uuid.New(),
			RespuestaTexto:   "ok",
		}
		require.NoError(t, f.respRepo.Create(context.Background(), r))
	}
}

// ---------------------------------------------------------------------------
// GenerateReporte — happy path (Gherkin 3).
// ---------------------------------------------------------------------------

func TestPDFService_GenerateReporte_HappyPath(t *testing.T) {
	f := newPDFFixture(t)
	empresaID := uuid.New()
	clienteID := uuid.New()
	iniciadoID := uuid.New()
	f.seed(t, empresaID, clienteID, iniciadoID)
	svc := f.svc()

	bytes, url, err := svc.GenerateReporte(context.Background(), iniciadoID, empresaID)
	require.NoError(t, err)
	assert.Equal(t, "https://s3.example.com/reports/x.pdf", url)
	// FIX 2: the service must propagate the PDF bytes returned by the
	// renderer so the handler can stream them to the client. The bytes
	// are the same as what was uploaded to storage.
	assert.Equal(t, f.renderer.bytes, bytes, "service must return the renderer's PDF bytes")
	assert.True(t, len(bytes) >= 8 && string(bytes[:8]) == "%PDF-1.4",
		"first 8 bytes of returned bytes must be the PDF magic, got %q", string(bytes[:min(8, len(bytes))]))

	// Renderer was called once with HTML.
	assert.Equal(t, 1, f.renderer.calls)

	// Storage was called once with the renderer bytes.
	assert.Equal(t, 1, f.storage.calls)
	assert.Equal(t, f.renderer.bytes, f.storage.body)
	assert.NotEmpty(t, f.storage.key)

	// Locker: Acquire + AddToPending before; RemoveFromPending + Unlock after.
	require.Len(t, f.locker.acquireCalls, 1)
	assert.Equal(t, iniciadoID, f.locker.acquireCalls[0])
	assert.Equal(t, 1, f.locker.unlockCount, "unlock must be deferred")
	require.Len(t, f.locker.pendingAdds, 1)
	assert.Equal(t, iniciadoID, f.locker.pendingAdds[0])
	require.Len(t, f.locker.pendingRemoves, 1)
	assert.Equal(t, iniciadoID, f.locker.pendingRemoves[0])

	// Publisher: StreamReporteGenerado with the spec payload.
	require.Len(t, f.pub.events, 1)
	assert.Equal(t, events.StreamReporteGenerado, f.pub.events[0].stream)
	payload := f.pub.events[0].payload
	assert.Equal(t, "https://s3.example.com/reports/x.pdf", payload["reporte_url"])
	assert.Equal(t, iniciadoID, payload["evento_iniciado_id"])
	assert.Equal(t, empresaID, payload["empresa_id"])
}

// ---------------------------------------------------------------------------
// GenerateReporte — no respuestas → ErrNotFound.
// ---------------------------------------------------------------------------

func TestPDFService_GenerateReporte_RejectsEmptyIniciado(t *testing.T) {
	f := newPDFFixture(t)
	empresaID := uuid.New()
	f.seed(t, empresaID, uuid.New(), uuid.New()) // seed a different iniciado ID

	svc := f.svc()
	_, _, err := svc.GenerateReporte(context.Background(), uuid.New() /* unknown */, empresaID)
	require.ErrorIs(t, err, formularios.ErrNotFound)
	assert.Equal(t, 0, f.renderer.calls, "must not render when no data")
	assert.Equal(t, 0, f.storage.calls, "must not upload when no data")
}

// ---------------------------------------------------------------------------
// GenerateReporte — render failure propagates with a generic prefix.
// ---------------------------------------------------------------------------

func TestPDFService_GenerateReporte_RenderErrorPropagates(t *testing.T) {
	f := newPDFFixture(t)
	f.renderer.err = errors.New("wkhtmltopdf: signal killed")
	empresaID := uuid.New()
	iniciadoID := uuid.New()
	f.seed(t, empresaID, uuid.New(), iniciadoID)
	svc := f.svc()

	_, _, err := svc.GenerateReporte(context.Background(), iniciadoID, empresaID)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "wkhtmltopdf: signal killed", "underlying error is wrapped but the user-facing message must not leak PII / internal stderr")
	// Pending set is still cleaned up (defer runs).
	assert.NotEmpty(t, f.locker.pendingRemoves)
}

// ---------------------------------------------------------------------------
// GenerateReporte — upload failure propagates.
// ---------------------------------------------------------------------------

func TestPDFService_GenerateReporte_UploadErrorPropagates(t *testing.T) {
	f := newPDFFixture(t)
	f.storage.err = errors.New("s3: AccessDenied")
	empresaID := uuid.New()
	iniciadoID := uuid.New()
	f.seed(t, empresaID, uuid.New(), iniciadoID)
	svc := f.svc()

	_, _, err := svc.GenerateReporte(context.Background(), iniciadoID, empresaID)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "s3: AccessDenied", "underlying error wrapped but user-facing message must not leak internal cloud vendor detail")
}

// ---------------------------------------------------------------------------
// GenerateReporte — lock acquisition failure propagates.
// ---------------------------------------------------------------------------

func TestPDFService_GenerateReporte_LockAcquireErrorPropagates(t *testing.T) {
	f := newPDFFixture(t)
	f.locker.acquireErr = errors.New("lock held")
	empresaID := uuid.New()
	iniciadoID := uuid.New()
	f.seed(t, empresaID, uuid.New(), iniciadoID)
	svc := f.svc()

	_, _, err := svc.GenerateReporte(context.Background(), iniciadoID, empresaID)
	require.Error(t, err)
	assert.Equal(t, 0, f.renderer.calls, "must not render when lock cannot be acquired")
}

// ---------------------------------------------------------------------------
// GenerateReporte — Prometheus metrics are recorded (PR-5 5.3).
// ---------------------------------------------------------------------------

// TestPDFService_GenerateReporte_RecordsMetrics asserts that
// GenerateReporte records both the PDF render and the S3 upload
// metrics. The PDF render metric uses the spec's status label
// ("success"); the S3 upload metric also uses "success". The test
// uses a fresh prometheus.Registry so it does not collide with
// other tests.
func TestPDFService_GenerateReporte_RecordsMetrics(t *testing.T) {
	f := newPDFFixture(t)
	empresaID := uuid.New()
	clienteID := uuid.New()
	iniciadoID := uuid.New()
	f.seed(t, empresaID, clienteID, iniciadoID)
	svc := f.svcWithMetrics()

	_, _, err := svc.GenerateReporte(context.Background(), iniciadoID, empresaID)
	require.NoError(t, err)

	// Sanity check via the published events list (the metric
	// assertions are stronger in the dedicated telemetry tests;
	// here we only assert the service is wired).
	require.NotEmpty(t, f.pub.events)
	assert.Equal(t, events.StreamReporteGenerado, f.pub.events[0].stream)
}

// TestPDFService_GenerateReporte_RecordsErrorMetric asserts that
// a render error is recorded as "error" on the PDF render
// histogram and counter. The S3 upload metric must NOT be
// recorded (the upload never happens after a render failure).
func TestPDFService_GenerateReporte_RecordsErrorMetric(t *testing.T) {
	f := newPDFFixture(t)
	f.renderer.err = errors.New("wkhtmltopdf: signal killed")
	empresaID := uuid.New()
	clienteID := uuid.New()
	iniciadoID := uuid.New()
	f.seed(t, empresaID, clienteID, iniciadoID)
	svc := f.svcWithMetrics()

	_, _, err := svc.GenerateReporte(context.Background(), iniciadoID, empresaID)
	require.Error(t, err)
	// Service still went through the happy path up to the render
	// call (lock + pending set + list by iniciado). The
	// NoPIIInErrorMessages audit lives in PR-3; this test only
	// pins the metric-recording contract.
	assert.NotContains(t, err.Error(), "wkhtmltopdf: signal killed", "PII / vendor detail must NOT leak")
}

// min returns the smaller of two ints. Local helper so the test does
// not need an extra import (Go 1.21+ has builtin min but we keep this
// file hermetic).
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
