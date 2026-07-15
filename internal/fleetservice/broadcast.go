package fleetservice

import (
	"encoding/json"
	"sync"

	"github.com/gorilla/websocket"
)

// clientSendBuffer bounds how far a single slow Dashboard client can lag behind before it starts
// missing broadcasts — sized generously for a low-frequency event stream (vehicle status, alerts,
// tasks), not a high-rate telemetry feed.
const clientSendBuffer = 32

// WSEvent is the broadcast envelope for every live update pushed to Dashboard clients (FLEET-06,
// ADR-028). Type discriminates Data's shape so the frontend can dispatch without guessing from
// field presence — "vehicle_status" (fleetservice.VehicleStatus), "alert_created" (Alert),
// "alert_acknowledged" (AlertAcknowledgedEvent), "task_created" (Task).
type WSEvent struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

// wsClient wraps one Dashboard connection. Writes go through a buffered channel + dedicated
// writePump goroutine — gorilla/websocket connections do not support concurrent writers, and
// Broadcast is called from multiple goroutines (MQTT gateway callbacks, REST handlers).
type wsClient struct {
	conn *websocket.Conn
	send chan []byte
}

// Hub fans live fleet events out to every connected Dashboard client (FLEET-06 — Multi-Workstation
// Live-Updates ohne Polling). Deliberately separate from control-server's vehicle-facing
// transport.WSHandler: different connection kind (Dashboard client, not vehicle), different
// participants, different protocol (JSON broadcast fan-out, not a per-session protobuf command
// channel) — sharing that type would conflate two unrelated concepts.
type Hub struct {
	mu      sync.RWMutex
	clients map[*wsClient]struct{}
}

func NewHub() *Hub {
	return &Hub{clients: make(map[*wsClient]struct{})}
}

func (h *Hub) register(c *wsClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[c] = struct{}{}
}

func (h *Hub) unregister(c *wsClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		close(c.send)
	}
}

// Broadcast pushes an event to every connected client. A client whose send buffer is full (slow
// consumer) has this event dropped rather than blocking delivery to every other client — it
// simply misses this one update and keeps receiving subsequent broadcasts.
func (h *Hub) Broadcast(eventType string, data any) {
	payload, err := json.Marshal(WSEvent{Type: eventType, Data: data})
	if err != nil {
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		select {
		case c.send <- payload:
		default:
		}
	}
}

// ClientCount reports the number of currently connected Dashboard clients (test/observability
// helper).
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// Connect registers a newly upgraded connection and blocks until it disconnects — callers run it
// directly inside their HTTP handler, mirroring the blocking-per-connection shape of
// http.HandlerFunc. The caller owns the *websocket.Conn's lifetime (Upgrade/Close).
func (h *Hub) Connect(conn *websocket.Conn) {
	c := &wsClient{conn: conn, send: make(chan []byte, clientSendBuffer)}
	h.register(c)
	defer h.unregister(c)

	go c.writePump()
	c.readPump()
}

func (c *wsClient) writePump() {
	for msg := range c.send {
		if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			// Force the blocked reader in readPump to unblock promptly instead of waiting for
			// its own read to time out on the same broken connection.
			c.conn.Close()
			return
		}
	}
}

// readPump only exists to detect client disconnects — Dashboard clients don't send anything on
// this connection, it is push-only (FLEET-06).
func (c *wsClient) readPump() {
	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return
		}
	}
}
