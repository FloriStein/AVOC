package main

import (
	"encoding/json"
	"net/http"
	"os"

	"avoc/internal/fleetservice"
	pkgdb "avoc/pkg/db"
	"avoc/pkg/logger"
)

var log = logger.New("fleet-service")

func main() {
	port := os.Getenv("FLEET_PORT")
	if port == "" {
		port = "8085"
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL environment variable is required")
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
	_ = store // REST API wiring (GET /fleet/vehicles etc.) is FLEET-05

	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "fleet-service"})
	})

	log.Info("Fleet Service starting", "port", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal("Fleet Service failed", "error", err)
	}
}
