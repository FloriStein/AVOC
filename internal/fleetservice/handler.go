package fleetservice

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"

	"avoc/internal/fleetgateway"
)

var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(_ *http.Request) bool { return true },
}

// Dispatcher is the subset of fleetgateway.FleetGateway the HTTP layer needs — narrow interface
// so Handler doesn't depend on the whole gateway (Subscribe* is main.go's concern, not the
// REST API's).
type Dispatcher interface {
	DispatchTask(task fleetgateway.TaskAssignment) error
}

// Handler is fleet-service's REST API (AP2 Web-Dashboard, first slice of AP3 Admin-Konsole
// CRUD) — mirrors internal/authservice.Handler's shape (secret + store). hub fans live updates
// out to Dashboard clients connected via ServeWS (FLEET-06).
type Handler struct {
	secret []byte
	store  *PostgresFleetStore
	gw     Dispatcher
	hub    *Hub
}

func NewHandler(secret string, store *PostgresFleetStore, gw Dispatcher, hub *Hub) *Handler {
	return &Handler{secret: []byte(secret), store: store, gw: gw, hub: hub}
}

// RequireAuth gates all /fleet/* endpoints behind a valid JWT (issued by auth-service, same
// shared secret as control-server/auth-service — ADR-004). Fleet data (positions, alerts,
// tasks) is operator-facing Leitstelle data, not public, unlike control-server's historically
// open GET /vehicles (ADR-014-era decision predating the Admin-Konsole's role/rights model).
func (h *Handler) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if !h.validateToken(strings.TrimPrefix(authHeader, "Bearer ")) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func (h *Handler) validateToken(tokenStr string) bool {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return h.secret, nil
	})
	return err == nil && token.Valid
}

// ServeWS upgrades to a Dashboard-facing WebSocket and streams live fleet events (FLEET-06,
// ADR-028: alerts are the signal an operator reacts to, so every connected workstation must
// receive state changes without polling). Can't reuse RequireAuth as-is — browser WebSocket
// clients cannot set a custom Authorization header on the handshake request, so the token may
// also arrive as a query parameter (mirrors control-server's vehicle-facing
// transport.extractToken, kept as a separate copy here since the two WS layers deliberately
// don't share code — different connection kind, different participants).
func (h *Handler) ServeWS(w http.ResponseWriter, r *http.Request) {
	tokenStr := wsToken(r)
	if tokenStr == "" || !h.validateToken(tokenStr) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	h.hub.Connect(conn)
}

func wsToken(r *http.Request) string {
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return r.URL.Query().Get("token")
}

// ─── Vehicles ───────────────────────────────────────────────────────────────

func (h *Handler) ListVehicles(w http.ResponseWriter, _ *http.Request) {
	vehicles, err := h.store.ListVehiclesWithStatus()
	if err != nil {
		http.Error(w, "store error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, vehicles)
}

// ─── Zones ──────────────────────────────────────────────────────────────────

func (h *Handler) ListZones(w http.ResponseWriter, _ *http.Request) {
	zones, err := h.store.ListZones()
	if err != nil {
		http.Error(w, "store error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, zones)
}

func (h *Handler) CreateZone(w http.ResponseWriter, r *http.Request) {
	var z Zone
	if err := json.NewDecoder(r.Body).Decode(&z); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if z.ID == "" || z.Name == "" || (z.Environment != "indoor" && z.Environment != "outdoor") {
		http.Error(w, "id, name and environment ('indoor'|'outdoor') required", http.StatusBadRequest)
		return
	}
	if err := h.store.AddZone(z); err != nil {
		http.Error(w, "store error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, z)
}

// ─── Stations ───────────────────────────────────────────────────────────────

func (h *Handler) ListStations(w http.ResponseWriter, _ *http.Request) {
	stations, err := h.store.ListStations()
	if err != nil {
		http.Error(w, "store error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, stations)
}

func (h *Handler) CreateStation(w http.ResponseWriter, r *http.Request) {
	var st Station
	if err := json.NewDecoder(r.Body).Decode(&st); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if st.ID == "" || st.ZoneID == "" || st.Name == "" {
		http.Error(w, "id, zone_id and name required", http.StatusBadRequest)
		return
	}
	if err := h.store.AddStation(st); err != nil {
		http.Error(w, "store error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, st)
}

// ─── Tasks ──────────────────────────────────────────────────────────────────

func (h *Handler) ListTasks(w http.ResponseWriter, _ *http.Request) {
	tasks, err := h.store.ListTasks()
	if err != nil {
		http.Error(w, "store error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, tasks)
}

// CreateTask persists the task and dispatches it to the vehicle via the FleetGateway
// (fire-and-forget — ADR-027, real ack semantics unknown until the AP1 workshop). Persistence
// succeeds even if dispatch fails; the task remains visible/retriable via the Dashboard.
func (h *Handler) CreateTask(w http.ResponseWriter, r *http.Request) {
	var t Task
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if t.VehicleID == "" || t.FromStationID == "" || t.ToStationID == "" {
		http.Error(w, "vehicle_id, from_station_id and to_station_id required", http.StatusBadRequest)
		return
	}
	created, err := h.store.CreateTask(t)
	if err != nil {
		http.Error(w, "store error", http.StatusInternalServerError)
		return
	}

	// Dispatch error intentionally not surfaced as a request failure — the task is already
	// persisted and visible/retriable via the Dashboard regardless (ADR-027: real dispatch ack
	// semantics unknown until the AP1 workshop). internal/ packages don't log (cmd/ does); a
	// dispatch failure here is silent by design for this first slice.
	_ = h.gw.DispatchTask(fleetgateway.TaskAssignment{
		TaskID: created.ID, VehicleID: created.VehicleID,
		FromStationID: created.FromStationID, ToStationID: created.ToStationID, Priority: created.Priority,
	})

	h.hub.Broadcast("task_created", created)

	w.WriteHeader(http.StatusCreated)
	writeJSON(w, created)
}

// UpdateTaskStatus handles the ADR-030 manual status-transition endpoint. Unlike CreateTask/
// AcknowledgeAlert (which only ever produce one kind of failure), a PATCH here can fail two
// distinct ways that need distinct HTTP codes: unknown id (404) vs. a structurally valid but
// currently-disallowed transition (409) — store.UpdateTaskStatus's ErrTaskNotFound/
// ErrInvalidTransition sentinels carry that distinction through.
func (h *Handler) UpdateTaskStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Status    string `json:"status"`
		ChangedBy string `json:"changed_by"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Status == "" || req.ChangedBy == "" {
		http.Error(w, "status and changed_by required", http.StatusBadRequest)
		return
	}

	updated, err := h.store.UpdateTaskStatus(id, req.Status, req.ChangedBy)
	switch {
	case errors.Is(err, ErrTaskNotFound):
		http.Error(w, "not found", http.StatusNotFound)
		return
	case errors.Is(err, ErrInvalidTransition):
		http.Error(w, "invalid status transition", http.StatusConflict)
		return
	case err != nil:
		http.Error(w, "store error", http.StatusInternalServerError)
		return
	}

	h.hub.Broadcast("task_status_changed", TaskStatusChangedEvent{
		ID:          updated.ID,
		Status:      updated.Status,
		CompletedAt: updated.CompletedAt,
		ChangedBy:   req.ChangedBy,
	})

	writeJSON(w, updated)
}

// TaskStatusChangedEvent is the FLEET-06-style broadcast payload for a status transition —
// smaller than the full Task row (mirrors AlertAcknowledgedEvent's rationale: GET /fleet/tasks
// remains the authoritative source for the full row).
type TaskStatusChangedEvent struct {
	ID          string     `json:"id"`
	Status      string     `json:"status"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	ChangedBy   string     `json:"status_changed_by"`
}

// ─── Alerts ─────────────────────────────────────────────────────────────────

func (h *Handler) ListAlerts(w http.ResponseWriter, _ *http.Request) {
	alerts, err := h.store.ListAlerts()
	if err != nil {
		http.Error(w, "store error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, alerts)
}

func (h *Handler) AcknowledgeAlert(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		AcknowledgedBy string `json:"acknowledged_by"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.AcknowledgedBy == "" {
		http.Error(w, "acknowledged_by required", http.StatusBadRequest)
		return
	}
	if err := h.store.AcknowledgeAlert(id, req.AcknowledgedBy); err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	h.hub.Broadcast("alert_acknowledged", AlertAcknowledgedEvent{
		ID:             id,
		AcknowledgedBy: req.AcknowledgedBy,
		AcknowledgedAt: time.Now(),
	})
	w.WriteHeader(http.StatusNoContent)
}

// AlertAcknowledgedEvent is the FLEET-06 broadcast payload for an acknowledgement — smaller than
// the full Alert row (store.AcknowledgeAlert only reports success/not-found, not the row itself;
// GET /fleet/alerts remains the authoritative source for the exact server-side timestamp).
type AlertAcknowledgedEvent struct {
	ID             string    `json:"id"`
	AcknowledgedBy string    `json:"acknowledged_by"`
	AcknowledgedAt time.Time `json:"acknowledged_at"`
}

// ─── Health ─────────────────────────────────────────────────────────────────

func (h *Handler) Health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]string{"status": "ok", "service": "fleet-service"})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
