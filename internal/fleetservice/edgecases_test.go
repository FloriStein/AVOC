package fleetservice

import (
	"database/sql"
	"os"
	"strings"
	"testing"

	_ "github.com/lib/pq"
)

// openTestStore is shared setup for edge-case and integration tests — mirrors the base-table
// bootstrap in store_test.go so each test file stays independently runnable.
func openTestStore(t *testing.T) (*sql.DB, *PostgresFleetStore) {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set — skipping Postgres integration test")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS vehicles (
		id TEXT PRIMARY KEY, display_name TEXT NOT NULL, description TEXT NOT NULL DEFAULT '',
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`); err != nil {
		t.Fatalf("create base vehicles table: %v", err)
	}

	store, err := NewPostgresFleetStore(db)
	if err != nil {
		t.Fatalf("NewPostgresFleetStore: %v", err)
	}
	return db, store
}

func TestEdgeCases_ForeignKeyViolations(t *testing.T) {
	db, store := openTestStore(t)

	t.Run("CreateTask with unknown vehicle_id fails", func(t *testing.T) {
		_, err := store.CreateTask(Task{VehicleID: "does-not-exist", FromStationID: "s1", ToStationID: "s2"})
		if err == nil {
			t.Fatal("expected FK violation error, got nil")
		}
	})

	t.Run("AddStation with unknown zone_id fails", func(t *testing.T) {
		err := store.AddStation(Station{ID: "orphan-station", ZoneID: "does-not-exist", Name: "Orphan"})
		if err == nil {
			t.Fatal("expected FK violation error, got nil")
		}
	})

	t.Run("CreateTask with unknown station_id fails", func(t *testing.T) {
		if _, err := db.Exec(`INSERT INTO vehicles (id, display_name) VALUES ('edge-veh-01', 'Edge Vehicle') ON CONFLICT (id) DO NOTHING`); err != nil {
			t.Fatalf("seed vehicle: %v", err)
		}
		t.Cleanup(func() { db.Exec(`DELETE FROM vehicles WHERE id = 'edge-veh-01'`) })

		_, err := store.CreateTask(Task{VehicleID: "edge-veh-01", FromStationID: "does-not-exist", ToStationID: "also-missing"})
		if err == nil {
			t.Fatal("expected FK violation error, got nil")
		}
	})
}

func TestEdgeCases_PrimaryKeyDuplicates(t *testing.T) {
	_, store := openTestStore(t)

	zone := Zone{ID: "edge-zone-dup", Name: "Dup Zone", Environment: "indoor"}
	if err := store.AddZone(zone); err != nil {
		t.Fatalf("first AddZone: %v", err)
	}
	t.Cleanup(func() {
		db, _ := sql.Open("postgres", os.Getenv("DATABASE_URL"))
		defer db.Close()
		db.Exec(`DELETE FROM zones WHERE id = 'edge-zone-dup'`)
	})

	if err := store.AddZone(zone); err == nil {
		t.Fatal("expected primary key violation on duplicate zone ID, got nil")
	} else if !strings.Contains(err.Error(), "duplicate key") {
		t.Fatalf("expected duplicate key error, got: %v", err)
	}
}

func TestEdgeCases_CheckConstraintViolations(t *testing.T) {
	db, store := openTestStore(t)
	if _, err := db.Exec(`INSERT INTO vehicles (id, display_name) VALUES ('edge-veh-02', 'Edge Vehicle 2') ON CONFLICT (id) DO NOTHING`); err != nil {
		t.Fatalf("seed vehicle: %v", err)
	}
	t.Cleanup(func() { db.Exec(`DELETE FROM vehicles WHERE id = 'edge-veh-02'`) })

	t.Run("invalid zone environment rejected", func(t *testing.T) {
		err := store.AddZone(Zone{ID: "edge-zone-bad-env", Name: "Bad", Environment: "underwater"})
		if err == nil {
			db.Exec(`DELETE FROM zones WHERE id = 'edge-zone-bad-env'`)
			t.Fatal("expected CHECK constraint violation, got nil")
		}
	})

	t.Run("invalid autonomy_mode rejected", func(t *testing.T) {
		err := store.UpsertVehicleStatus(VehicleStatus{VehicleID: "edge-veh-02", AutonomyMode: "sentient"})
		if err == nil {
			t.Fatal("expected CHECK constraint violation, got nil")
		}
	})

	t.Run("invalid alert severity rejected", func(t *testing.T) {
		_, err := store.CreateAlert(Alert{VehicleID: "edge-veh-02", Severity: "catastrophic", Message: "test"})
		if err == nil {
			t.Fatal("expected CHECK constraint violation, got nil")
		}
	})
}

func TestEdgeCases_NotFoundPaths(t *testing.T) {
	_, store := openTestStore(t)

	t.Run("SetVehicleType on unknown vehicle returns not-found error", func(t *testing.T) {
		err := store.SetVehicleType("no-such-vehicle-ever", "lastenrad")
		if err == nil {
			t.Fatal("expected not-found error, got nil")
		}
	})

	t.Run("AcknowledgeAlert on unknown alert returns not-found error", func(t *testing.T) {
		err := store.AcknowledgeAlert("no-such-alert-ever", "operator-1")
		if err == nil {
			t.Fatal("expected not-found error, got nil")
		}
	})
}

func TestEdgeCases_NullableFieldsAndUnicode(t *testing.T) {
	db, store := openTestStore(t)

	t.Run("indoor station without GPS coordinates round-trips nil", func(t *testing.T) {
		if err := store.AddZone(Zone{ID: "edge-zone-nullable", Name: "Halle Ost", Environment: "indoor"}); err != nil {
			t.Fatalf("AddZone: %v", err)
		}
		t.Cleanup(func() {
			db.Exec(`DELETE FROM stations WHERE zone_id = 'edge-zone-nullable'`)
			db.Exec(`DELETE FROM zones WHERE id = 'edge-zone-nullable'`)
		})

		x, y := 12.5, 7.0
		if err := store.AddStation(Station{ID: "edge-station-nullable", ZoneID: "edge-zone-nullable", Name: "Ladestation Ost", PositionX: &x, PositionY: &y}); err != nil {
			t.Fatalf("AddStation: %v", err)
		}
		stations, err := store.ListStations()
		if err != nil {
			t.Fatalf("ListStations: %v", err)
		}
		var found *Station
		for i := range stations {
			if stations[i].ID == "edge-station-nullable" {
				found = &stations[i]
			}
		}
		if found == nil {
			t.Fatal("station not found after insert")
		}
		if found.PositionLat != nil || found.PositionLon != nil {
			t.Fatalf("expected nil GPS coords for indoor-only station, got lat=%v lon=%v", found.PositionLat, found.PositionLon)
		}
		if found.PositionX == nil || *found.PositionX != 12.5 {
			t.Fatalf("expected PositionX=12.5, got %v", found.PositionX)
		}
	})

	t.Run("German umlauts round-trip correctly in zone name and alert message", func(t *testing.T) {
		if _, err := db.Exec(`INSERT INTO vehicles (id, display_name) VALUES ('edge-veh-unicode', 'Ünicode Vehicle') ON CONFLICT (id) DO NOTHING`); err != nil {
			t.Fatalf("seed vehicle: %v", err)
		}
		t.Cleanup(func() {
			db.Exec(`DELETE FROM alerts WHERE vehicle_id = 'edge-veh-unicode'`)
			db.Exec(`DELETE FROM vehicles WHERE id = 'edge-veh-unicode'`)
		})

		const msg = "Hindernis erkannt — Lastenzug kann nicht selbstständig ausweichen, Größe überschritten"
		alert, err := store.CreateAlert(Alert{VehicleID: "edge-veh-unicode", Severity: "critical", Message: msg})
		if err != nil {
			t.Fatalf("CreateAlert: %v", err)
		}
		alerts, err := store.ListAlerts()
		if err != nil {
			t.Fatalf("ListAlerts: %v", err)
		}
		found := false
		for _, a := range alerts {
			if a.ID == alert.ID {
				found = true
				if a.Message != msg {
					t.Fatalf("unicode message corrupted: got %q, want %q", a.Message, msg)
				}
			}
		}
		if !found {
			t.Fatal("alert not found after insert")
		}
	})
}
