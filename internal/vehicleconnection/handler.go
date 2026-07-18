// Package vehicleconnection manages WebSocket connections from vehicles (ADR-015, BE-06).
// Vehicle disconnect triggers SAFE_MODE — kein Session-State-Ownership hier (GSA = Control Server).
// Commands are forwarded to the vehicle via Registry.ForwardCommand (ADR-021).
package vehicleconnection

import (
	"net/http"
	"strings"
	"time"

	vehiclev1 "avoc/gen/go/vehicle/v1"
	"avoc/internal/controlserver/statemachine"
	"avoc/internal/controlserver/vehiclecontext"
	"avoc/internal/safetyservice"
	"avoc/pkg/logger"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
)

var svcLog = logger.New("control-server")

var upgrader = websocket.Upgrader{
	CheckOrigin: func(_ *http.Request) bool { return true },
}

type vehicleClaims struct {
	jwt.RegisteredClaims
	Role string `json:"role"`
}

type safetyPublisher interface {
	PublishEvent(event safetyservice.SafetyEvent)
}

// VehicleAdder auto-registers unknown vehicles in the fleet DB on first connect.
type VehicleAdder interface {
	Add(id, displayName, description string) error
}

// Handler manages incoming vehicle WebSocket connections. Looks up the
// per-vehicle State Machine and VehicleACKWatchdog via the Registry using the
// vehicle's own JWT subject as the key (ADR-026) — no single injected
// sm/vehicleACKWatchdog, since two vehicles must never share safety state.
type Handler struct {
	jwtSecret       []byte
	vehicleContexts *vehiclecontext.Registry
	publisher       safetyPublisher
	registry        *Registry
	ackStore        *AckStore
	vehicleAdder    VehicleAdder
}

// HandlerOptions bundles NewHandler's inputs (Rule 2.3 — more than 4 params).
type HandlerOptions struct {
	JWTSecret       string
	VehicleContexts *vehiclecontext.Registry
	Publisher       safetyPublisher
	Registry        *Registry
	AckStore        *AckStore
}

func NewHandler(opts HandlerOptions) *Handler {
	return &Handler{
		jwtSecret:       []byte(opts.JWTSecret),
		vehicleContexts: opts.VehicleContexts,
		publisher:       opts.Publisher,
		registry:        opts.Registry,
		ackStore:        opts.AckStore,
	}
}

// WithVehicleAdder enables auto-registration of unknown vehicles on first connect.
func (h *Handler) WithVehicleAdder(a VehicleAdder) *Handler {
	h.vehicleAdder = a
	return h
}

// ServeWS upgrades the HTTP connection to WebSocket and validates the vehicle JWT.
// Registers the connection in the Registry for command forwarding.
// Disconnect triggers WS_DISCONNECT → SAFE_MODE (ADR-009/010).
func (h *Handler) ServeWS(w http.ResponseWriter, r *http.Request) {
	claims, err := h.validateJWT(extractToken(r))
	if err != nil || claims.Role != "VEHICLE" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		svcLog.Error("vehicle WebSocket upgrade failed", "error", err)
		return
	}
	defer conn.Close()

	vehicleID := claims.Subject
	// Auto-register vehicle in the fleet DB on first connect (idempotent).
	if h.vehicleAdder != nil {
		if err := h.vehicleAdder.Add(vehicleID, vehicleID, ""); err != nil {
			svcLog.Info("vehicle auto-register skipped (already exists)", "vehicle_id", vehicleID)
		}
	}
	h.registry.register(vehicleID, conn)
	defer h.registry.unregister(vehicleID)

	svcLog.Info("vehicle connected", "vehicle_id", vehicleID)

	go h.heartbeat(conn)
	h.readLoop(conn, claims)
}

func (h *Handler) heartbeat(conn *websocket.Conn) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
			return
		}
	}
}

func (h *Handler) readLoop(conn *websocket.Conn, claims *vehicleClaims) {
	vehicleID := claims.Subject
	vc := h.vehicleContexts.Get(vehicleID)
	defer func() {
		sys, _, _, _ := vc.SM.Get()
		if sys != statemachine.StateSafeMode {
			vc.SM.TransitionSystem(statemachine.StateSafeMode)
			h.publisher.PublishEvent(safetyservice.SafetyEvent{
				Type:      safetyservice.EventWSDisconnect,
				Reason:    "vehicle WebSocket disconnected",
				VehicleID: vehicleID,
				Timestamp: time.Now(),
			})
			svcLog.Event(logger.EventWsDisconnect,
				"vehicle disconnected → SAFE_MODE", "vehicle_id", vehicleID)
		}
	}()

	conn.SetPongHandler(func(_ string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		// Decode VehicleCommandAck and store it (ADR-021).
		ack := &vehiclev1.VehicleCommandAck{}
		if err := proto.Unmarshal(data, ack); err == nil && ack.Received {
			h.ackStore.Store(vehicleID, ack)
			vc.VehicleACKWatchdog.ACKReceived()
			svcLog.Info("vehicle ACK received",
				"vehicle_id", vehicleID,
				"command_event_id", ack.CommandEventId,
			)
		}
	}
}

func (h *Handler) validateJWT(tokenStr string) (*vehicleClaims, error) {
	claims := &vehicleClaims{}
	_, err := jwt.ParseWithClaims(tokenStr, claims, func(_ *jwt.Token) (any, error) {
		return h.jwtSecret, nil
	})
	return claims, err
}

func extractToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return r.URL.Query().Get("token")
}
