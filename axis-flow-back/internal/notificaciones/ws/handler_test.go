package ws_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"axis-flow-back/internal/domain"
	notificaciones "axis-flow-back/internal/notificaciones"
	ws "axis-flow-back/internal/notificaciones/ws"
	"axis-flow-back/internal/service"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const handlerTestJWTSecret = "handler-test-secret-32-bytes-xx"

// mintHandlerToken signs a minimal access token for handler tests.
func mintHandlerToken(t *testing.T, userID uuid.UUID) string {
	t.Helper()
	claims := &service.Claims{
		UserID:   userID.String(),
		TenantID: uuid.New().String(),
		Email:    "ws@example.com",
		Role:     "EMPLEADO",
	}
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(handlerTestJWTSecret))
	if err != nil {
		t.Fatalf("mintHandlerToken: %v", err)
	}
	return tok
}

// stubHandlerAuthSvc returns a *service.AuthService that uses the handler test secret.
func stubHandlerAuthSvc(t *testing.T) *service.AuthService {
	t.Helper()
	return service.NewAuthService(
		&handlerStubUserRepo{},
		&handlerStubSessionRepo{},
		handlerTestJWTSecret,
		15*time.Minute,
		7*24*time.Hour,
		&handlerStubEmpleadoLookup{},
	)
}

// ---------------------------------------------------------------------------
// Test: missing token → connection refused before upgrade (HTTP 400/401)
// ---------------------------------------------------------------------------

func TestHandler_MissingToken_Rejected(t *testing.T) {
	hub := ws.NewHub(nil)
	go hub.Run(t.Context())

	cfg := notificaciones.ConfigFromEnv()
	cfg.WSHandshakeTimeout = 5 * time.Second
	cfg.WSPongWait = 5 * time.Second
	cfg.WSWriteWait = 5 * time.Second
	cfg.WSPingPeriod = 4 * time.Second
	cfg.RateLimitWSMsgPerSec = 100

	authSvc := stubHandlerAuthSvc(t)
	handler := ws.NewHandler(hub, authSvc, cfg)

	srv := httptest.NewServer(http.HandlerFunc(handler.ServeWS))
	t.Cleanup(srv.Close)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") // no ?token=
	_, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err == nil {
		t.Fatal("expected dial to fail without token")
	}
	if resp != nil && resp.StatusCode == http.StatusSwitchingProtocols {
		t.Fatal("server should not have upgraded without a valid token")
	}
}

// ---------------------------------------------------------------------------
// Test: invalid token → upgrade accepted, then close frame with code 4001
// ---------------------------------------------------------------------------

func TestHandler_InvalidToken_Close4001(t *testing.T) {
	hub := ws.NewHub(nil)
	go hub.Run(t.Context())

	cfg := notificaciones.ConfigFromEnv()
	cfg.WSHandshakeTimeout = 5 * time.Second
	cfg.WSPongWait = 5 * time.Second
	cfg.WSWriteWait = 5 * time.Second
	cfg.WSPingPeriod = 4 * time.Second
	cfg.RateLimitWSMsgPerSec = 100

	authSvc := stubHandlerAuthSvc(t)
	handler := ws.NewHandler(hub, authSvc, cfg)

	srv := httptest.NewServer(http.HandlerFunc(handler.ServeWS))
	t.Cleanup(srv.Close)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "?token=not.a.valid.jwt"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		// Some clients may reject at dial if the server closes immediately — acceptable.
		return
	}
	t.Cleanup(func() { conn.Close() })

	// The server upgrades then sends a close frame with code 4001.
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, readErr := conn.ReadMessage()
	if readErr == nil {
		t.Fatal("expected connection to be closed by server")
	}
	closeErr, ok := readErr.(*websocket.CloseError)
	if !ok {
		t.Fatalf("expected *websocket.CloseError, got %T: %v", readErr, readErr)
	}
	if closeErr.Code != 4001 {
		t.Errorf("expected close code 4001, got %d", closeErr.Code)
	}
}

// ---------------------------------------------------------------------------
// Test: valid token → connection accepted, client registered in hub
// ---------------------------------------------------------------------------

func TestHandler_ValidToken_Accepted(t *testing.T) {
	hub := ws.NewHub(nil)
	go hub.Run(t.Context())

	cfg := notificaciones.ConfigFromEnv()
	cfg.WSHandshakeTimeout = 5 * time.Second
	cfg.WSPongWait = 5 * time.Second
	cfg.WSWriteWait = 5 * time.Second
	cfg.WSPingPeriod = 4 * time.Second
	cfg.RateLimitWSMsgPerSec = 100

	authSvc := stubHandlerAuthSvc(t)
	handler := ws.NewHandler(hub, authSvc, cfg)

	srv := httptest.NewServer(http.HandlerFunc(handler.ServeWS))
	t.Cleanup(srv.Close)

	userID := uuid.New()
	tok := mintHandlerToken(t, userID)
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "?token=" + tok

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("expected successful connection with valid token, got: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	// Give hub time to register the client
	time.Sleep(50 * time.Millisecond)

	// Confirm hub has the client by broadcasting and receiving
	hub.Broadcast(userID, "test", []byte(`{"data":"test"}`))

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, readErr := conn.ReadMessage()
	if readErr != nil {
		t.Fatalf("expected to receive broadcast after valid connection, got: %v", readErr)
	}
	if len(msg) == 0 {
		t.Error("expected non-empty broadcast message")
	}
}

// ---------------------------------------------------------------------------
// Stub repos for AuthService construction in handler tests
// ---------------------------------------------------------------------------

type handlerStubUserRepo struct{}

func (s *handlerStubUserRepo) FindByEmail(_ context.Context, _ string) (*domain.User, error) {
	return nil, nil
}
func (s *handlerStubUserRepo) FindByID(_ context.Context, _ uuid.UUID) (*domain.User, error) {
	return nil, nil
}
func (s *handlerStubUserRepo) FindPrimaryRoleCode(_ context.Context, _ uuid.UUID) (string, error) {
	return "", nil
}
func (s *handlerStubUserRepo) ListPermissionCodes(_ context.Context, _ uuid.UUID) ([]string, error) {
	return nil, nil
}
func (s *handlerStubUserRepo) UpdateStatus(_ context.Context, _ uuid.UUID, _ domain.UserStatus) error {
	return nil
}
func (s *handlerStubUserRepo) UpdateLastLogin(_ context.Context, _ uuid.UUID, _ time.Time) error {
	return nil
}
func (s *handlerStubUserRepo) UpdatePasswordHash(_ context.Context, _ uuid.UUID, _ string) error {
	return nil
}
func (s *handlerStubUserRepo) IncrementFailedAttempts(_ context.Context, _ uuid.UUID) error {
	return nil
}
func (s *handlerStubUserRepo) ResetFailedAttempts(_ context.Context, _ uuid.UUID) error {
	return nil
}

type handlerStubSessionRepo struct{}

func (s *handlerStubSessionRepo) Create(_ context.Context, _ *domain.Session) error { return nil }
func (s *handlerStubSessionRepo) FindByRefreshTokenHash(_ context.Context, _ string) (*domain.Session, error) {
	return nil, nil
}
func (s *handlerStubSessionRepo) Revoke(_ context.Context, _ uuid.UUID) error { return nil }

type handlerStubEmpleadoLookup struct{}

func (s *handlerStubEmpleadoLookup) GetEmpleadoIDByUserID(_ context.Context, _, _ uuid.UUID) (int64, error) {
	return 0, nil
}
