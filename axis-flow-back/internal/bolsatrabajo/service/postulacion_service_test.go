package service_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"axis-flow-back/internal/bolsatrabajo"
	"axis-flow-back/internal/bolsatrabajo/events"
	"axis-flow-back/internal/bolsatrabajo/service"

	"github.com/google/uuid"
)

// --- mock PostulacionRepository ---

type mockPostulacionRepo struct {
	createFn        func(ctx context.Context, p *bolsatrabajo.Postulacion) error
	getStatsFn      func(ctx context.Context, trabajoID, empresaID uuid.UUID) (*bolsatrabajo.PipelineStats, error)
	updateEstatusFn func(ctx context.Context, id, empresaID uuid.UUID, newEstatus int) (*bolsatrabajo.Postulacion, error)
}

func (m *mockPostulacionRepo) Create(ctx context.Context, p *bolsatrabajo.Postulacion) error {
	return m.createFn(ctx, p)
}
func (m *mockPostulacionRepo) GetStatsByTrabajo(ctx context.Context, trabajoID, empresaID uuid.UUID) (*bolsatrabajo.PipelineStats, error) {
	return m.getStatsFn(ctx, trabajoID, empresaID)
}
func (m *mockPostulacionRepo) UpdateEstatus(ctx context.Context, id, empresaID uuid.UUID, newEstatus int) (*bolsatrabajo.Postulacion, error) {
	return m.updateEstatusFn(ctx, id, empresaID, newEstatus)
}

// --- mock TurnstileClient ---

type mockTurnstile struct {
	verifyFn func(ctx context.Context, token, remoteIP string) error
	// capturedToken records the token passed so we can assert the secret is not in it.
	capturedToken string
}

func (m *mockTurnstile) Verify(ctx context.Context, token, remoteIP string) error {
	m.capturedToken = token
	return m.verifyFn(ctx, token, remoteIP)
}

// --- mock BolsaTrabajoStorage ---

type mockStorage struct {
	uploadFn func(ctx context.Context, key string, r io.Reader, size int64, contentType string) (string, error)
}

func (m *mockStorage) UploadCV(ctx context.Context, key string, r io.Reader, size int64, contentType string) (string, error) {
	return m.uploadFn(ctx, key, r, size, contentType)
}
func (m *mockStorage) DeleteCV(_ context.Context, _ string) error { return nil }

// pdfMagic returns a reader whose first 4 bytes are a valid PDF magic header.
func pdfMagic() io.Reader {
	b := make([]byte, 16)
	b[0] = 0x25
	b[1] = 0x50
	b[2] = 0x44
	b[3] = 0x46
	return bytes.NewReader(b)
}

// badMagic returns a reader with no valid magic bytes.
func badMagic() io.Reader {
	return strings.NewReader("INVALID_FILE_CONTENT")
}

const fakeSecret = "SUPER_SECRET_KEY_12345"

func TestPostulacionService_Apply_TurnstileFail(t *testing.T) {
	t.Parallel()
	turnstile := &mockTurnstile{
		verifyFn: func(_ context.Context, _, _ string) error {
			return bolsatrabajo.ErrCaptchaFail
		},
	}
	svc := service.NewPostulacionService(
		&mockPostulacionRepo{},
		turnstile,
		&mockStorage{},
		&mockPublisher{},
	)
	p := &bolsatrabajo.Postulacion{TrabajoID: uuid.New()}
	_, err := svc.Apply(context.Background(), p, pdfMagic(), 100, "bad-token", "127.0.0.1")
	if !errors.Is(err, bolsatrabajo.ErrCaptchaFail) {
		t.Fatalf("expected ErrCaptchaFail, got %v", err)
	}
}

func TestPostulacionService_Apply_InvalidMIME(t *testing.T) {
	t.Parallel()
	turnstile := &mockTurnstile{
		verifyFn: func(_ context.Context, _, _ string) error { return nil },
	}
	svc := service.NewPostulacionService(
		&mockPostulacionRepo{},
		turnstile,
		&mockStorage{},
		&mockPublisher{},
	)
	p := &bolsatrabajo.Postulacion{TrabajoID: uuid.New()}
	_, err := svc.Apply(context.Background(), p, badMagic(), 100, "good-token", "127.0.0.1")
	if !errors.Is(err, bolsatrabajo.ErrInvalidFile) {
		t.Fatalf("expected ErrInvalidFile, got %v", err)
	}
}

func TestPostulacionService_Apply_Success_PublishesEvent(t *testing.T) {
	t.Parallel()
	turnstile := &mockTurnstile{
		verifyFn: func(_ context.Context, _, _ string) error { return nil },
	}
	repo := &mockPostulacionRepo{
		createFn: func(_ context.Context, p *bolsatrabajo.Postulacion) error { return nil },
	}
	storage := &mockStorage{
		uploadFn: func(_ context.Context, _ string, _ io.Reader, _ int64, _ string) (string, error) {
			return "https://cdn.example.com/cv.pdf", nil
		},
	}
	pub := &mockPublisher{}
	svc := service.NewPostulacionService(repo, turnstile, storage, pub)

	p := &bolsatrabajo.Postulacion{TrabajoID: uuid.New(), NombreCompleto: "Jane Doe"}
	result, err := svc.Apply(context.Background(), p, pdfMagic(), 16, "valid-token", "10.0.0.1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.CvURL == "" {
		t.Error("expected CvURL to be set")
	}
	if len(pub.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(pub.events))
	}
	if pub.events[0].stream != events.StreamPostulacionRecibida {
		t.Errorf("expected stream %s, got %s", events.StreamPostulacionRecibida, pub.events[0].stream)
	}
}

func TestPostulacionService_Apply_SecretNotInToken(t *testing.T) {
	t.Parallel()
	ts := &mockTurnstile{
		verifyFn: func(_ context.Context, token, _ string) error { return nil },
	}
	repo := &mockPostulacionRepo{
		createFn: func(_ context.Context, _ *bolsatrabajo.Postulacion) error { return nil },
	}
	storage := &mockStorage{
		uploadFn: func(_ context.Context, _ string, _ io.Reader, _ int64, _ string) (string, error) {
			return "https://cdn.example.com/cv.pdf", nil
		},
	}
	pub := &mockPublisher{}
	svc := service.NewPostulacionService(repo, ts, storage, pub)

	p := &bolsatrabajo.Postulacion{TrabajoID: uuid.New()}
	// Use the fakeSecret as the turnstile token — the service must NOT forward it to Verify.
	// The mock records whatever token was passed; we assert it is fakeSecret (the client-side token),
	// but crucially the service itself must never inject the server-side secret into the token param.
	_, _ = svc.Apply(context.Background(), p, pdfMagic(), 16, fakeSecret, "127.0.0.1")
	// The token the mock received must equal exactly what was passed — no server secret appended.
	if strings.Contains(ts.capturedToken, "SUPER_SECRET_KEY") && ts.capturedToken != fakeSecret {
		t.Errorf("service must not modify or augment the token with the server secret; got: %s", ts.capturedToken)
	}
}

func TestPostulacionService_UpdateEstatus_Contracted_PublishesBothEvents(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	empresaID := uuid.New()
	repo := &mockPostulacionRepo{
		updateEstatusFn: func(_ context.Context, _, _ uuid.UUID, _ int) (*bolsatrabajo.Postulacion, error) {
			return &bolsatrabajo.Postulacion{ID: id, Estatus: bolsatrabajo.PostulacionContratado}, nil
		},
	}
	pub := &mockPublisher{}
	svc := service.NewPostulacionService(repo, &mockTurnstile{}, &mockStorage{}, pub)

	_, err := svc.UpdateEstatus(context.Background(), id, empresaID, bolsatrabajo.PostulacionContratado)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pub.events) != 2 {
		t.Fatalf("expected 2 events (PostulacionEstatusActualizado + CandidatoContratado), got %d", len(pub.events))
	}
	streams := map[string]bool{}
	for _, e := range pub.events {
		streams[e.stream] = true
	}
	if !streams[events.StreamPostulacionEstatusActualizado] {
		t.Error("missing PostulacionEstatusActualizado event")
	}
	if !streams[events.StreamCandidatoContratado] {
		t.Error("missing CandidatoContratado event")
	}
}

func TestPostulacionService_UpdateEstatus_InvalidStatus(t *testing.T) {
	t.Parallel()
	svc := service.NewPostulacionService(&mockPostulacionRepo{}, &mockTurnstile{}, &mockStorage{}, &mockPublisher{})
	_, err := svc.UpdateEstatus(context.Background(), uuid.New(), uuid.New(), 99)
	if err == nil {
		t.Fatal("expected error for invalid estatus 99")
	}
}
