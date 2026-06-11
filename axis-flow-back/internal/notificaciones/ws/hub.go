// Package ws implements the WebSocket hub, client, and HTTP upgrade handler
// for the Notificaciones module.
package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// redisBroadcastChannel is the Redis Pub/Sub channel used to fan-out messages
// across multiple hub instances (horizontal scaling).
const redisBroadcastChannel = "notificaciones:broadcast"

// Message is the envelope broadcast through the hub and over Redis Pub/Sub.
type Message struct {
	UserID  uuid.UUID `json:"user_id"`
	Event   string    `json:"event"`
	Payload []byte    `json:"payload"`
}

// Hub manages all active WebSocket clients for the Notificaciones module.
// A single Hub instance is created at startup and shared across handlers.
//
// Scaling note: when rdb is non-nil the Hub subscribes to redisBroadcastChannel
// and re-broadcasts incoming messages to locally connected clients. This allows
// multiple Hub instances (different pods/nodes) to deliver notifications without
// a shared in-process state.
type Hub struct {
	// clients maps userID → slice of active *Client connections.
	// A user may have multiple open connections (e.g. two browser tabs).
	clients map[uuid.UUID][]*Client

	// closed tracks clients whose send channel has already been closed, preventing
	// double-close panics when ReadPump's deferred Unsubscribe races with an
	// explicit Unsubscribe from the test or handler.
	closed map[*Client]bool

	subscribe   chan *Client
	unsubscribe chan *Client
	broadcast   chan Message

	rdb redis.UniversalClient // nil → no Redis fan-out (single-node / test mode)
	mu  sync.RWMutex
}

// NewHub allocates a Hub. Pass nil for rdb in tests or single-node deployments.
func NewHub(rdb redis.UniversalClient) *Hub {
	return &Hub{
		clients:     make(map[uuid.UUID][]*Client),
		closed:      make(map[*Client]bool),
		subscribe:   make(chan *Client, 64),
		unsubscribe: make(chan *Client, 64),
		broadcast:   make(chan Message, 256),
		rdb:         rdb,
	}
}

// Subscribe enqueues a client for registration with the hub.
func (h *Hub) Subscribe(c *Client) {
	h.subscribe <- c
}

// Unsubscribe enqueues a client for removal from the hub.
func (h *Hub) Unsubscribe(c *Client) {
	h.unsubscribe <- c
}

// Broadcast enqueues a message for delivery to all local clients matching
// userID and, when Redis is configured, publishes it on the shared channel so
// other hub instances can deliver it too.
func (h *Hub) Broadcast(userID uuid.UUID, event string, payload []byte) {
	msg := Message{UserID: userID, Event: event, Payload: payload}
	h.broadcast <- msg

	if h.rdb != nil {
		data, err := json.Marshal(msg)
		if err != nil {
			slog.Error("ws.Hub: failed to marshal message for Redis publish", "error", err)
			return
		}
		// Fire-and-forget — publish errors are logged but not fatal.
		go func() {
			if err := h.rdb.Publish(context.Background(), redisBroadcastChannel, data).Err(); err != nil {
				slog.Error("ws.Hub: Redis PUBLISH error", "error", err)
			}
		}()
	}
}

// Run is the hub's main event loop. It must be started in its own goroutine.
// It exits when ctx is cancelled.
func (h *Hub) Run(ctx context.Context) {
	var pubsub *redis.PubSub
	redisCh := make(<-chan *redis.Message) // nil channel by default (blocks forever)

	if h.rdb != nil {
		pubsub = h.rdb.Subscribe(ctx, redisBroadcastChannel)
		redisCh = pubsub.Channel()
		defer pubsub.Close()
	}

	for {
		select {
		case <-ctx.Done():
			return

		case c := <-h.subscribe:
			h.mu.Lock()
			h.clients[c.userID] = append(h.clients[c.userID], c)
			h.mu.Unlock()

		case c := <-h.unsubscribe:
			h.mu.Lock()
			h.removeClient(c)
			h.closeSend(c)
			h.mu.Unlock()

		case msg := <-h.broadcast:
			h.mu.RLock()
			targets := h.clients[msg.UserID]
			h.mu.RUnlock()
			h.deliverToClients(msg, targets)

		case redisMsg := <-redisCh:
			var msg Message
			if err := json.Unmarshal([]byte(redisMsg.Payload), &msg); err != nil {
				slog.Error("ws.Hub: failed to unmarshal Redis message", "error", err)
				continue
			}
			// Only deliver to locally connected clients (avoid double delivery
			// to the originating node — the local broadcast channel handles that).
			h.mu.RLock()
			targets := h.clients[msg.UserID]
			h.mu.RUnlock()
			h.deliverToClients(msg, targets)
		}
	}
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// closeSend closes c.send exactly once (caller must hold mu.Lock).
// Test-managed clients own their send channel; the hub must not close it.
func (h *Hub) closeSend(c *Client) {
	if c.testManaged {
		return
	}
	if !h.closed[c] {
		h.closed[c] = true
		close(c.send)
	}
}

// removeClient removes c from h.clients without locking (caller must hold mu.Lock).
func (h *Hub) removeClient(c *Client) {
	list := h.clients[c.userID]
	updated := list[:0]
	for _, existing := range list {
		if existing != c {
			updated = append(updated, existing)
		}
	}
	if len(updated) == 0 {
		delete(h.clients, c.userID)
	} else {
		h.clients[c.userID] = updated
	}
}

// deliverToClients encodes msg as JSON and attempts non-blocking delivery to
// each target client's send channel. Slow clients that cannot receive are
// unsubscribed and their connection closed.
func (h *Hub) deliverToClients(msg Message, targets []*Client) {
	data, err := json.Marshal(msg)
	if err != nil {
		slog.Error("ws.Hub: failed to marshal message for delivery", "error", err)
		return
	}

	for _, c := range targets {
		select {
		case c.send <- data:
		default:
			// Client send buffer full — drop and disconnect.
			h.mu.Lock()
			h.removeClient(c)
			h.closeSend(c)
			h.mu.Unlock()
		}
	}
}
