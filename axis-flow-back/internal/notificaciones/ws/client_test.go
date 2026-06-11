package ws_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	notificaciones "axis-flow-back/internal/notificaciones"
	ws "axis-flow-back/internal/notificaciones/ws"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

var testUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// dialTestServer spins up an httptest.Server that upgrades the connection, then
// creates a Client + Hub and starts the pumps. Returns the client WS conn and
// a channel that is closed when the server-side client exits.
func dialTestServer(t *testing.T, cfg notificaciones.Config) (clientConn *websocket.Conn, done <-chan struct{}) {
	t.Helper()

	userID := uuid.New()
	doneCh := make(chan struct{})

	hub := ws.NewHub(nil) // nil Redis — no pub/sub needed for client tests
	go hub.Run(t.Context())

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := testUpgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade error: %v", err)
			return
		}
		client := ws.NewClient(hub, conn, userID, cfg)
		hub.Subscribe(client)
		go client.WritePump(cfg)
		go func() {
			client.ReadPump(cfg)
			close(doneCh)
		}()
	}))
	t.Cleanup(srv.Close)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial error: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	return conn, doneCh
}

// ---------------------------------------------------------------------------
// Test: rate limit enforcement
// ---------------------------------------------------------------------------

func TestClient_RateLimit(t *testing.T) {
	cfg := notificaciones.ConfigFromEnv()
	cfg.RateLimitWSMsgPerSec = 3
	cfg.WSPongWait = 5 * time.Second
	cfg.WSWriteWait = 5 * time.Second
	cfg.WSPingPeriod = 4 * time.Second
	cfg.WSMaxMessageSize = 4096

	clientConn, done := dialTestServer(t, cfg)

	validMsg, _ := json.Marshal(map[string]string{"event": "ping", "payload": "x"})

	// Send more messages than the rate limit allows in one second
	for i := 0; i < cfg.RateLimitWSMsgPerSec+2; i++ {
		_ = clientConn.WriteMessage(websocket.TextMessage, validMsg)
	}

	// The server should close the connection with policy violation (1008)
	// or any close frame within a reasonable window.
	clientConn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, _, err := clientConn.ReadMessage()
	if err == nil {
		// If we didn't get an error, keep reading until close
		for {
			_, _, err = clientConn.ReadMessage()
			if err != nil {
				break
			}
		}
	}

	closeErr, ok := err.(*websocket.CloseError)
	if !ok {
		// Accept any close/EOF as proof the server terminated the connection
		select {
		case <-done:
			// server-side goroutine exited — connection was terminated
		case <-time.After(3 * time.Second):
			t.Fatal("expected server to close connection after rate limit exceeded, timed out")
		}
		return
	}
	if closeErr.Code != websocket.CloseUnsupportedData && closeErr.Code != websocket.ClosePolicyViolation {
		t.Errorf("expected close code 1003 or 1008, got %d", closeErr.Code)
	}
}

// ---------------------------------------------------------------------------
// Test: write pump closes on hub unsubscribe / send channel close
// ---------------------------------------------------------------------------

func TestClient_WritePumpClosesOnSendChanClose(t *testing.T) {
	cfg := notificaciones.ConfigFromEnv()
	cfg.WSPongWait = 5 * time.Second
	cfg.WSWriteWait = 2 * time.Second
	cfg.WSPingPeriod = 4 * time.Second
	cfg.WSMaxMessageSize = 4096
	cfg.RateLimitWSMsgPerSec = 100

	userID := uuid.New()
	doneCh := make(chan struct{})

	hub := ws.NewHub(nil)
	go hub.Run(t.Context())

	var registeredClient *ws.Client

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := testUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		client := ws.NewClient(hub, conn, userID, cfg)
		registeredClient = client
		hub.Subscribe(client)
		go client.WritePump(cfg)
		go func() {
			client.ReadPump(cfg)
			close(doneCh)
		}()
	}))
	t.Cleanup(srv.Close)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial error: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	// Wait until client is registered
	time.Sleep(50 * time.Millisecond)

	// Unsubscribe — hub closes the send channel
	hub.Unsubscribe(registeredClient)

	// WritePump should close the underlying conn; the client conn should see a close
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, _, readErr := conn.ReadMessage()
	if readErr == nil {
		t.Error("expected connection to be closed by server after unsubscribe")
	}
}

// ---------------------------------------------------------------------------
// Test: ping/pong — server sends ping, we reply with pong, connection stays alive
// ---------------------------------------------------------------------------

func TestClient_PingPong(t *testing.T) {
	cfg := notificaciones.ConfigFromEnv()
	cfg.WSPongWait = 3 * time.Second
	cfg.WSWriteWait = 2 * time.Second
	cfg.WSPingPeriod = 500 * time.Millisecond // short for test
	cfg.WSMaxMessageSize = 4096
	cfg.RateLimitWSMsgPerSec = 100

	clientConn, _ := dialTestServer(t, cfg)

	// Register pong handler so the WS library replies automatically
	clientConn.SetPongHandler(func(string) error { return nil })

	// Wait long enough for at least one ping to be sent
	time.Sleep(700 * time.Millisecond)

	// Connection should still be alive — send a message and expect no error
	msg, _ := json.Marshal(map[string]string{"event": "ping", "payload": "alive"})
	if err := clientConn.WriteMessage(websocket.TextMessage, msg); err != nil {
		t.Errorf("connection closed unexpectedly after ping/pong: %v", err)
	}
}
