package main

import (
	"net/http"
	"os"
	"time"

	"avoc/internal/fleetgateway"
	"avoc/internal/fleetservice"
	pkgdb "avoc/pkg/db"
	"avoc/pkg/logger"
)

var log = logger.New("fleet-service")

func main() {
	port := envOr("FLEET_PORT", "8085")
	mqttBroker := envOr("MQTT_BROKER", "mosquitto:1883")

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET environment variable is required")
	}

	db, err := pkgdb.Open(databaseURL)
	if err != nil {
		log.Fatal("failed to open database", "error", err)
	}
	defer db.Close()

	// See auth-service/control-server main.go: Docker's `restart: unless-stopped` policy does
	// not honor `depends_on: service_healthy`, so a crash-restarted fleet-service can race
	// Postgres's own startup.
	if err := pkgdb.WaitForReady(db, pkgdb.DefaultConnectRetries, pkgdb.DefaultConnectRetryDelay); err != nil {
		log.Warn("database not reachable after retries — proceeding anyway", "error", err)
	}

	// NewPostgresFleetStore extends the vehicles table (vehicle_type) and creates the
	// zones/stations/tasks/vehicle_status/alerts tables — idempotent, safe regardless of
	// whether control-server's vehicleregistry has already created the base vehicles table
	// (ADR-029: fleet-service can start before or after control-server).
	store, err := fleetservice.NewPostgresFleetStore(db)
	if err != nil {
		log.Fatal("failed to initialize fleet store", "error", err)
	}

	// MQTTGateway is the concrete FleetGateway realization (FLEET-04/05) — same Mosquitto broker
	// vehicle-mock's fleet simulation publishes to. Retried like pkgdb.WaitForReady: mosquitto
	// and fleet-service have no depends_on between them either.
	gw, err := connectGatewayWithRetry(mqttBroker)
	if err != nil {
		log.Fatal("failed to connect to MQTT broker", "broker", mqttBroker, "error", err)
	}
	defer gw.Close()

	// hub fans every state change out to connected Dashboard clients (FLEET-06, ADR-028 —
	// multiple operators at different workstations must see alerts/status live, without
	// polling). Fed from two places below: the MQTT gateway callbacks (vehicle-initiated status/
	// alerts) and the REST handlers (task creation, alert acknowledgement).
	hub := fleetservice.NewHub()

	// alertEngine raises threshold-based alerts (FLEET-07, e.g. low battery) from a vehicle's
	// own reported status — separate from vehicle-initiated alerts below, which a vehicle
	// detects and reports about itself (ADR-028: two sources, one `alerts` table/shape).
	alertEngine := fleetservice.NewAlertEngine()

	gw.SubscribeVehicleStatus(func(e fleetgateway.VehicleStatusEvent) {
		// Fleet vehicles never establish a WS connection to control-server, so they never hit
		// its "Auto-Register bei erstem WS-Connect" path (ADR-029) — without this, every status
		// event for a not-yet-provisioned vehicle silently fails the FK constraint below.
		if err := store.EnsureVehicleExists(e.VehicleID); err != nil {
			log.Warn("failed to auto-register vehicle", "vehicle_id", e.VehicleID, "error", err)
			return
		}
		status := fleetservice.VehicleStatus{
			VehicleID:      e.VehicleID,
			BatteryPct:     e.BatteryPct,
			Speed:          e.Speed,
			PositionLat:    e.PositionLat,
			PositionLon:    e.PositionLon,
			PositionZoneID: e.PositionZoneID,
			AutonomyMode:   e.AutonomyMode,
			CurrentTaskID:  e.CurrentTaskID,
		}
		if err := store.UpsertVehicleStatus(status); err != nil {
			log.Warn("failed to persist vehicle status", "vehicle_id", e.VehicleID, "error", err)
			return
		}
		status.UpdatedAt = time.Now()
		hub.Broadcast("vehicle_status", status)

		if alert := alertEngine.Evaluate(status); alert != nil {
			created, err := store.CreateAlert(*alert)
			if err != nil {
				log.Warn("failed to persist threshold alert", "vehicle_id", e.VehicleID, "error", err)
				return
			}
			hub.Broadcast("alert_created", created)
		}
	})
	gw.SubscribeVehicleAlerts(func(e fleetgateway.VehicleAlertEvent) {
		// Same FK gap as SubscribeVehicleStatus above — a vehicle-initiated alert can in
		// principle arrive before that vehicle's first status event.
		if err := store.EnsureVehicleExists(e.VehicleID); err != nil {
			log.Warn("failed to auto-register vehicle", "vehicle_id", e.VehicleID, "error", err)
			return
		}
		created, err := store.CreateAlert(fleetservice.Alert{
			VehicleID: e.VehicleID,
			Severity:  e.Severity,
			Message:   e.Message,
		})
		if err != nil {
			log.Warn("failed to persist vehicle alert", "vehicle_id", e.VehicleID, "error", err)
			return
		}
		hub.Broadcast("alert_created", created)
	})

	handler := fleetservice.NewHandler(jwtSecret, store, gw, hub)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handler.Health)
	mux.HandleFunc("GET /fleet/vehicles", handler.RequireAuth(handler.ListVehicles))
	mux.HandleFunc("GET /fleet/zones", handler.RequireAuth(handler.ListZones))
	mux.HandleFunc("POST /fleet/zones", handler.RequireAuth(handler.CreateZone))
	mux.HandleFunc("GET /fleet/stations", handler.RequireAuth(handler.ListStations))
	mux.HandleFunc("POST /fleet/stations", handler.RequireAuth(handler.CreateStation))
	mux.HandleFunc("GET /fleet/tasks", handler.RequireAuth(handler.ListTasks))
	mux.HandleFunc("POST /fleet/tasks", handler.RequireAuth(handler.CreateTask))
	mux.HandleFunc("PATCH /fleet/tasks/{id}/status", handler.RequireAuth(handler.UpdateTaskStatus))
	mux.HandleFunc("GET /fleet/alerts", handler.RequireAuth(handler.ListAlerts))
	mux.HandleFunc("POST /fleet/alerts/{id}/acknowledge", handler.RequireAuth(handler.AcknowledgeAlert))
	mux.HandleFunc("GET /fleet/ws", handler.ServeWS)

	log.Info("Fleet Service starting", "port", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal("Fleet Service failed", "error", err)
	}
}

// connectGatewayWithRetry mirrors pkgdb.WaitForReady's retry shape for the MQTT broker
// dependency — mosquitto has no depends_on ordering guarantee relative to fleet-service either.
func connectGatewayWithRetry(broker string) (*fleetgateway.MQTTGateway, error) {
	var lastErr error
	for i := 0; i < pkgdb.DefaultConnectRetries; i++ {
		gw, err := fleetgateway.NewMQTTGateway("tcp://" + broker)
		if err == nil {
			return gw, nil
		}
		lastErr = err
		log.Warn("MQTT broker not reachable yet, retrying", "broker", broker, "error", err)
		time.Sleep(pkgdb.DefaultConnectRetryDelay)
	}
	return nil, lastErr
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
