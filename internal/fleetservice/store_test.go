package fleetservice

import (
	"database/sql"
	"os"
	"testing"

	_ "github.com/lib/pq"
)

// TestPostgresFleetStore_SchemaAndCRUD verifies the FLEET-01 schema against a real Postgres —
// requires DATABASE_URL (skipped otherwise, matching the project's integration-test convention).
func TestPostgresFleetStore_SchemaAndCRUD(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set — skipping Postgres integration test")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	// Registered before the data-cleanup below so it runs LAST (t.Cleanup is LIFO) —
	// a plain `defer db.Close()` would run before t.Cleanup callbacks and break the
	// DELETE statements below with a "closed connection" error.
	t.Cleanup(func() { db.Close() })

	// Base vehicles table must exist first — normally created by vehicleregistry
	// (control-server). Recreated here minimally so this package is independently testable.
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS vehicles (
		id TEXT PRIMARY KEY, display_name TEXT NOT NULL, description TEXT NOT NULL DEFAULT '',
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`); err != nil {
		t.Fatalf("create base vehicles table: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO vehicles (id, display_name) VALUES ('test-vehicle-01', 'Test Vehicle')
		ON CONFLICT (id) DO NOTHING`); err != nil {
		t.Fatalf("seed vehicle: %v", err)
	}

	store, err := NewPostgresFleetStore(db)
	if err != nil {
		t.Fatalf("NewPostgresFleetStore: %v", err)
	}

	t.Run("SetVehicleType", func(t *testing.T) {
		if err := store.SetVehicleType("test-vehicle-01", "lastenrad"); err != nil {
			t.Fatalf("SetVehicleType: %v", err)
		}
	})

	t.Run("Zone + Station + Task round-trip", func(t *testing.T) {
		if err := store.AddZone(Zone{ID: "zone-test", Name: "Test Zone", Environment: "indoor"}); err != nil {
			t.Fatalf("AddZone: %v", err)
		}
		zones, err := store.ListZones()
		if err != nil || len(zones) == 0 {
			t.Fatalf("ListZones: %v (got %d zones)", err, len(zones))
		}

		x, y := 1.0, 2.0
		if err := store.AddStation(Station{ID: "station-a", ZoneID: "zone-test", Name: "Station A", PositionX: &x, PositionY: &y}); err != nil {
			t.Fatalf("AddStation A: %v", err)
		}
		if err := store.AddStation(Station{ID: "station-b", ZoneID: "zone-test", Name: "Station B", PositionX: &x, PositionY: &y}); err != nil {
			t.Fatalf("AddStation B: %v", err)
		}
		stations, err := store.ListStations()
		if err != nil || len(stations) < 2 {
			t.Fatalf("ListStations: %v (got %d stations)", err, len(stations))
		}

		task, err := store.CreateTask(Task{VehicleID: "test-vehicle-01", FromStationID: "station-a", ToStationID: "station-b", Priority: 5})
		if err != nil {
			t.Fatalf("CreateTask: %v", err)
		}
		if task.ID == "" || task.Status != "pending" {
			t.Fatalf("unexpected task: %+v", task)
		}
		tasks, err := store.ListTasks()
		if err != nil || len(tasks) == 0 {
			t.Fatalf("ListTasks: %v (got %d tasks)", err, len(tasks))
		}
	})

	t.Run("VehicleStatus upsert", func(t *testing.T) {
		battery := 87.5
		if err := store.UpsertVehicleStatus(VehicleStatus{VehicleID: "test-vehicle-01", BatteryPct: &battery, AutonomyMode: "autonomous"}); err != nil {
			t.Fatalf("UpsertVehicleStatus: %v", err)
		}
		// Upsert again to verify ON CONFLICT path (no duplicate row).
		battery2 := 90.0
		if err := store.UpsertVehicleStatus(VehicleStatus{VehicleID: "test-vehicle-01", BatteryPct: &battery2, AutonomyMode: "teleoperated"}); err != nil {
			t.Fatalf("UpsertVehicleStatus (update): %v", err)
		}
		statuses, err := store.ListVehicleStatus()
		if err != nil {
			t.Fatalf("ListVehicleStatus: %v", err)
		}
		found := false
		for _, s := range statuses {
			if s.VehicleID == "test-vehicle-01" {
				found = true
				if s.AutonomyMode != "teleoperated" || s.BatteryPct == nil || *s.BatteryPct != 90.0 {
					t.Fatalf("expected updated status, got %+v", s)
				}
			}
		}
		if !found {
			t.Fatal("test-vehicle-01 status not found after upsert")
		}
	})

	t.Run("Alert create + acknowledge", func(t *testing.T) {
		alert, err := store.CreateAlert(Alert{VehicleID: "test-vehicle-01", Severity: "warning", Message: "Low battery"})
		if err != nil {
			t.Fatalf("CreateAlert: %v", err)
		}
		if alert.ID == "" {
			t.Fatal("expected generated alert ID")
		}
		if err := store.AcknowledgeAlert(alert.ID, "operator-1"); err != nil {
			t.Fatalf("AcknowledgeAlert: %v", err)
		}
		alerts, err := store.ListAlerts()
		if err != nil || len(alerts) == 0 {
			t.Fatalf("ListAlerts: %v (got %d alerts)", err, len(alerts))
		}
	})

	// Cleanup — keep the shared dev DB tidy for subsequent runs/other services.
	t.Cleanup(func() {
		db.Exec(`DELETE FROM alerts WHERE vehicle_id = 'test-vehicle-01'`)
		db.Exec(`DELETE FROM vehicle_status WHERE vehicle_id = 'test-vehicle-01'`)
		db.Exec(`DELETE FROM tasks WHERE vehicle_id = 'test-vehicle-01'`)
		db.Exec(`DELETE FROM stations WHERE zone_id = 'zone-test'`)
		db.Exec(`DELETE FROM zones WHERE id = 'zone-test'`)
		db.Exec(`DELETE FROM vehicles WHERE id = 'test-vehicle-01'`)
	})
}
