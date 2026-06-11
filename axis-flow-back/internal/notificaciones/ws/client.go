package ws

import (
	"encoding/json"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	notificaciones "axis-flow-back/internal/notificaciones"
)

// knownEvents is the set of event names accepted from clients.
// Unknown events are rejected with close code 1003 (Unsupported Data).
var knownEvents = map[string]bool{
	"ping":         true,
	"subscribe":    true,
	"unsubscribe":  true,
	"chat_message": true,
}

// Client represents one active WebSocket connection for a specific user.
type Client struct {
	hub    *Hub
	conn   *websocket.Conn // nil for test clients
	userID uuid.UUID
	send   chan []byte

	// testManaged signals that the send channel is owned by the test harness and
	// must NOT be closed by the hub. Only NewTestClient sets this.
	testManaged bool

	msgCount int64 // atomic — messages received in the current rate-limit window
}

// NewClient allocates a Client.
func NewClient(hub *Hub, conn *websocket.Conn, userID uuid.UUID, cfg notificaciones.Config) *Client {
	_ = cfg
	return &Client{
		hub:    hub,
		conn:   conn,
		userID: userID,
		send:   make(chan []byte, 256),
	}
}

// NewTestClient creates a Client with an injected send channel and no real
// WebSocket connection. Intended for hub unit tests only.
// The hub will NOT close the send channel for test-managed clients.
func NewTestClient(hub *Hub, userID uuid.UUID, sendCh chan []byte) *Client {
	return &Client{
		hub:         hub,
		conn:        nil,
		userID:      userID,
		send:        sendCh,
		testManaged: true,
	}
}

// ---------------------------------------------------------------------------
// ReadPump
// ---------------------------------------------------------------------------

// ReadPump reads messages from the WebSocket and applies rate limiting.
// It blocks until the connection is closed or an error occurs.
func (c *Client) ReadPump(cfg notificaciones.Config) {
	defer func() {
		c.hub.Unsubscribe(c)
		if c.conn != nil {
			c.conn.Close()
		}
	}()

	if c.conn == nil {
		return
	}

	c.conn.SetReadLimit(cfg.WSMaxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(cfg.WSPongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(cfg.WSPongWait))
		return nil
	})

	// Rate-limit window: reset msgCount every second.
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		msgType, data, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err,
				websocket.CloseGoingAway,
				websocket.CloseNormalClosure,
				websocket.CloseNoStatusReceived,
			) {
				slog.Info("ws.Client: read error", "user_id", c.userID, "error", err)
			}
			return
		}

		// Drain ticker without blocking; reset count on tick.
		select {
		case <-ticker.C:
			atomic.StoreInt64(&c.msgCount, 0)
		default:
		}

		if msgType != websocket.TextMessage {
			continue
		}

		// Rate limit check.
		count := atomic.AddInt64(&c.msgCount, 1)
		if int(count) > cfg.RateLimitWSMsgPerSec {
			slog.Info("ws.Client: rate limit exceeded, closing connection", "user_id", c.userID)
			c.conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "rate limit exceeded"))
			return
		}

		// Validate event field.
		var envelope struct {
			Event string `json:"event"`
		}
		if err := json.Unmarshal(data, &envelope); err != nil || !knownEvents[envelope.Event] {
			c.conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseUnsupportedData, "unknown event type"))
			return
		}

		// Forward the message as a broadcast from this user.
		c.hub.Broadcast(c.userID, envelope.Event, data)
	}
}

// ---------------------------------------------------------------------------
// WritePump
// ---------------------------------------------------------------------------

// WritePump writes queued messages to the WebSocket and sends periodic pings.
// It blocks until the send channel is closed or a write error occurs.
func (c *Client) WritePump(cfg notificaciones.Config) {
	ticker := time.NewTicker(cfg.WSPingPeriod)
	defer func() {
		ticker.Stop()
		if c.conn != nil {
			c.conn.Close()
		}
	}()

	for {
		select {
		case msg, ok := <-c.send:
			if c.conn == nil {
				if !ok {
					return
				}
				continue
			}
			c.conn.SetWriteDeadline(time.Now().Add(cfg.WSWriteWait))
			if !ok {
				// Hub closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}

		case <-ticker.C:
			if c.conn == nil {
				continue
			}
			c.conn.SetWriteDeadline(time.Now().Add(cfg.WSWriteWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
