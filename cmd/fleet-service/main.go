package main

import (
	"net/http"
	"time"

	"avoc/internal/fleetgateway"
	"avoc/internal/fleetservice"
	pkgdb "avoc/pkg/db"
	"avoc/pkg/env"
	"avoc/pkg/logger"
)

var log = logger.New("fleet-service")

func main() {
	port := env.OptionalOr("FLEET_PORT", "8085")
	mqttBroker := env.OptionalOr("MQTT_BROKER", "mosquitto:1883")
	mqttUsername := env.OptionalOr("MQTT_USERNAME", "")
	mqttPassword := env.OptionalOr("MQTT_PASSWORD", "")

	databaseURL := env.Require("DATABASE_URL", log)
	jwtSecret := env.Require("JWT_SECRET", log)

	db := pkgdb.OpenAndWait(databaseURL, log, "database not reachable after retries — proceeding anyway")
	defer db.Close()

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
	gw, err := connectGatewayWithRetry(mqttBroker, mqttUsername, mqttPassword)
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

	subscribeVehicleStatus(gw, store, hub, alertEngine)
	subscribeVehicleAlerts(gw, store, hub)
	startPositionHistoryRetentionLoop(store)

	handler := fleetservice.NewHandler(jwtSecret, store, gw, hub)
	mux := newFleetMux(handler)

	log.Info("Fleet Service starting", "port", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal("Fleet Service failed", "error", err)
	}
}

// subscribeVehicleStatus persists vehicle-reported status (FLEET-06), broadcasts it to connected
// Dashboard clients, and raises a threshold alert via alertEngine if warranted (FLEET-07). Takes
// the fleetgateway.FleetGateway interface (ADR-027), not the concrete MQTTGateway — it only needs
// SubscribeVehicleStatus, so it stays agnostic to which gateway implementation main() wires up
// (GOSTYLE-IF-02).
func subscribeVehicleStatus(gw fleetgateway.FleetGateway, store *fleetservice.PostgresFleetStore, hub *fleetservice.Hub, alertEngine *fleetservice.AlertEngine) {
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
		// ADR-033 "gefahrene Route" — throttled inside RecordPositionHistory itself, not here; a
		// failure here must not block the (already-persisted) live status update above, so it's
		// only logged.
		if err := store.RecordPositionHistory(e.VehicleID, e.PositionLat, e.PositionLon); err != nil {
			log.Warn("failed to record position history", "vehicle_id", e.VehicleID, "error", err)
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
}

// subscribeVehicleAlerts persists vehicle-initiated alerts — the second alert source from
// ADR-028, distinct from the threshold alerts subscribeVehicleStatus raises. Takes the
// fleetgateway.FleetGateway interface, same reasoning as subscribeVehicleStatus (GOSTYLE-IF-02).
func subscribeVehicleAlerts(gw fleetgateway.FleetGateway, store *fleetservice.PostgresFleetStore, hub *fleetservice.Hub) {
	gw.SubscribeVehicleAlerts(func(e fleetgateway.VehicleAlertEvent) {
		// Same FK gap as subscribeVehicleStatus above — a vehicle-initiated alert can in
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
}

// positionHistoryPruneInterval is how often startPositionHistoryRetentionLoop re-runs the
// ADR-033 retention cleanup while the process keeps running, on top of the one-shot prune
// NewPostgresFleetStore already does at startup.
const positionHistoryPruneInterval = 24 * time.Hour

// startPositionHistoryRetentionLoop periodically prunes vehicle_position_history (ADR-033) so a
// long-running fleet-service instance doesn't rely on a restart to shed rows past the retention
// window — NewPostgresFleetStore's startup prune alone only covers the moment of process start.
func startPositionHistoryRetentionLoop(store *fleetservice.PostgresFleetStore) {
	go func() {
		ticker := time.NewTicker(positionHistoryPruneInterval)
		defer ticker.Stop()
		for range ticker.C {
			if err := store.PruneVehiclePositionHistory(); err != nil {
				log.Warn("failed to prune vehicle_position_history", "error", err)
			}
		}
	}()
}

func newFleetMux(handler *fleetservice.Handler) *http.ServeMux {
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
	mux.HandleFunc("GET /fleet/tasks/{id}/history", handler.RequireAuth(handler.GetTaskStatusHistory))
	mux.HandleFunc("GET /fleet/vehicles/{id}/history", handler.RequireAuth(handler.GetVehiclePositionHistory))
	mux.HandleFunc("GET /fleet/alerts", handler.RequireAuth(handler.ListAlerts))
	mux.HandleFunc("POST /fleet/alerts/{id}/acknowledge", handler.RequireAuth(handler.AcknowledgeAlert))
	mux.HandleFunc("GET /fleet/ws", handler.ServeWS)
	return mux
}

// connectGatewayWithRetry mirrors pkgdb.WaitForReady's retry shape for the MQTT broker
// dependency — mosquitto has no depends_on ordering guarantee relative to fleet-service either.
func connectGatewayWithRetry(broker, username, password string) (*fleetgateway.MQTTGateway, error) {
	var lastErr error
	for i := 0; i < pkgdb.DefaultConnectRetries; i++ {
		gw, err := fleetgateway.NewMQTTGateway("tcp://"+broker, username, password)
		if err == nil {
			return gw, nil
		}
		lastErr = err
		log.Warn("MQTT broker not reachable yet, retrying", "broker", broker, "error", err)
		time.Sleep(pkgdb.DefaultConnectRetryDelay)
	}
	return nil, lastErr
}
