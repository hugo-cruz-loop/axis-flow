package ws_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	ws "axis-flow-back/internal/notificaciones/ws"
)

// ---------------------------------------------------------------------------
// Test: Subscribe adds a client and Broadcast delivers to that client
// ---------------------------------------------------------------------------

func TestHub_SubscribeAndBroadcast(t *testing.T) {
	hub := ws.NewHub(nil)
	ctx := t.Context()
	go hub.Run(ctx)

	userID := uuid.New()
	sendCh := make(chan []byte, 8)
	client := ws.NewTestClient(hub, userID, sendCh)
	hub.Subscribe(client)

	// Give the hub goroutine time to process subscribe
	time.Sleep(20 * time.Millisecond)

	payload := []byte(`{"msg":"hello"}`)
	hub.Broadcast(userID, "test_event", payload)

	select {
	case got := <-sendCh:
		if string(got) == "" {
			t.Error("expected non-empty message on send channel")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for broadcast to reach client")
	}
}

// ---------------------------------------------------------------------------
// Test: Unsubscribe removes client — subsequent broadcast does not deliver
// ---------------------------------------------------------------------------

func TestHub_Unsubscribe(t *testing.T) {
	hub := ws.NewHub(nil)
	ctx := t.Context()
	go hub.Run(ctx)

	userID := uuid.New()
	sendCh := make(chan []byte, 8)
	client := ws.NewTestClient(hub, userID, sendCh)
	hub.Subscribe(client)
	time.Sleep(20 * time.Millisecond)

	hub.Unsubscribe(client)
	time.Sleep(20 * time.Millisecond)

	hub.Broadcast(userID, "test_event", []byte("should not arrive"))
	time.Sleep(100 * time.Millisecond)

	select {
	case msg := <-sendCh:
		t.Errorf("expected no message after unsubscribe, got: %s", msg)
	default:
		// correct — nothing delivered
	}
}

// ---------------------------------------------------------------------------
// Test: Broadcast to user A does not deliver to user B
// ---------------------------------------------------------------------------

func TestHub_BroadcastIsolation(t *testing.T) {
	hub := ws.NewHub(nil)
	ctx := t.Context()
	go hub.Run(ctx)

	userA := uuid.New()
	userB := uuid.New()

	sendA := make(chan []byte, 8)
	sendB := make(chan []byte, 8)

	clientA := ws.NewTestClient(hub, userA, sendA)
	clientB := ws.NewTestClient(hub, userB, sendB)

	hub.Subscribe(clientA)
	hub.Subscribe(clientB)
	time.Sleep(20 * time.Millisecond)

	hub.Broadcast(userA, "only_for_a", []byte("secret"))

	select {
	case <-sendA:
		// correct
	case <-time.After(300 * time.Millisecond):
		t.Fatal("userA's client did not receive the broadcast")
	}

	select {
	case msg := <-sendB:
		t.Errorf("userB should not have received userA's message, got: %s", msg)
	default:
		// correct
	}
}

// ---------------------------------------------------------------------------
// Test: multiple clients for same user all receive the broadcast
// ---------------------------------------------------------------------------

func TestHub_MultipleClientsPerUser(t *testing.T) {
	hub := ws.NewHub(nil)
	ctx := t.Context()
	go hub.Run(ctx)

	userID := uuid.New()
	sendCh1 := make(chan []byte, 8)
	sendCh2 := make(chan []byte, 8)

	c1 := ws.NewTestClient(hub, userID, sendCh1)
	c2 := ws.NewTestClient(hub, userID, sendCh2)
	hub.Subscribe(c1)
	hub.Subscribe(c2)
	time.Sleep(20 * time.Millisecond)

	hub.Broadcast(userID, "multi", []byte("broadcast"))

	for i, ch := range []chan []byte{sendCh1, sendCh2} {
		select {
		case <-ch:
			// ok
		case <-time.After(300 * time.Millisecond):
			t.Errorf("client %d did not receive broadcast", i+1)
		}
	}
}
