package fleetservice

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// waitForClientCount polls Hub.ClientCount() until it matches want or the timeout expires —
// register/unregister happen in a goroutine relative to the test (Connect blocks the connection's
// own goroutine), so a plain assertion right after dialing would race.
func waitForClientCount(t *testing.T, hub *Hub, want int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if hub.ClientCount() == want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for ClientCount() == %d, got %d", want, hub.ClientCount())
}

func newTestHubServer(t *testing.T, hub *Hub) (wsURL string, cleanup func()) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := wsUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		hub.Connect(conn)
	}))
	wsURL = "ws" + srv.URL[len("http"):]
	return wsURL, srv.Close
}

func dialTestClient(t *testing.T, wsURL string) *websocket.Conn {
	t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func TestHub_Broadcast_DeliversToConnectedClient(t *testing.T) {
	hub := NewHub()
	wsURL, cleanup := newTestHubServer(t, hub)
	defer cleanup()

	client := dialTestClient(t, wsURL)
	waitForClientCount(t, hub, 1, time.Second)

	hub.Broadcast("vehicle_status", map[string]string{"vehicle_id": "v1"})

	client.SetReadDeadline(time.Now().Add(time.Second))
	_, msg, err := client.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	var evt WSEvent
	if err := json.Unmarshal(msg, &evt); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if evt.Type != "vehicle_status" {
		t.Fatalf("expected type vehicle_status, got %q", evt.Type)
	}
}

func TestHub_Broadcast_DeliversToMultipleClients(t *testing.T) {
	hub := NewHub()
	wsURL, cleanup := newTestHubServer(t, hub)
	defer cleanup()

	clientA := dialTestClient(t, wsURL)
	clientB := dialTestClient(t, wsURL)
	waitForClientCount(t, hub, 2, time.Second)

	hub.Broadcast("alert_created", map[string]string{"id": "a1"})

	for _, c := range []*websocket.Conn{clientA, clientB} {
		c.SetReadDeadline(time.Now().Add(time.Second))
		_, msg, err := c.ReadMessage()
		if err != nil {
			t.Fatalf("ReadMessage: %v", err)
		}
		var evt WSEvent
		if err := json.Unmarshal(msg, &evt); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if evt.Type != "alert_created" {
			t.Fatalf("expected type alert_created, got %q", evt.Type)
		}
	}
}

func TestHub_ClientDisconnect_Unregisters(t *testing.T) {
	hub := NewHub()
	wsURL, cleanup := newTestHubServer(t, hub)
	defer cleanup()

	client := dialTestClient(t, wsURL)
	waitForClientCount(t, hub, 1, time.Second)

	client.Close()
	waitForClientCount(t, hub, 0, time.Second)
}

// TestHub_Broadcast_SlowConsumerDropsWithoutBlockingOthers exercises the drop-on-full-buffer path
// directly (bypassing the real network connection — Broadcast only ever touches wsClient.send, not
// .conn, so this is a faithful test of the actual code path exercised in production when a
// Dashboard client stalls). The key property: one full client must never block delivery to others.
func TestHub_Broadcast_SlowConsumerDropsWithoutBlockingOthers(t *testing.T) {
	hub := NewHub()

	slow := &wsClient{send: make(chan []byte, 1)}
	hub.register(slow)
	hub.Broadcast("fill", nil) // fills slow's 1-slot buffer

	fast := &wsClient{send: make(chan []byte, clientSendBuffer)}
	hub.register(fast)

	done := make(chan struct{})
	go func() {
		hub.Broadcast("vehicle_status", map[string]string{"vehicle_id": "v1"})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Broadcast blocked despite a full slow-consumer buffer")
	}

	select {
	case msg := <-fast.send:
		var evt WSEvent
		if err := json.Unmarshal(msg, &evt); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if evt.Type != "vehicle_status" {
			t.Fatalf("unexpected type: %s", evt.Type)
		}
	default:
		t.Fatal("fast client did not receive the broadcast")
	}

	if len(slow.send) != 1 {
		t.Fatalf("expected slow client's buffer to stay at 1 (second event dropped), got %d", len(slow.send))
	}
}

func TestHub_ClientCount_ReflectsRegisterAndUnregister(t *testing.T) {
	hub := NewHub()
	if got := hub.ClientCount(); got != 0 {
		t.Fatalf("expected 0 clients initially, got %d", got)
	}

	c := &wsClient{send: make(chan []byte, 1)}
	hub.register(c)
	if got := hub.ClientCount(); got != 1 {
		t.Fatalf("expected 1 client after register, got %d", got)
	}

	hub.unregister(c)
	if got := hub.ClientCount(); got != 0 {
		t.Fatalf("expected 0 clients after unregister, got %d", got)
	}

	// unregistering an already-unregistered client must not panic (double-close guard).
	hub.unregister(c)
}
