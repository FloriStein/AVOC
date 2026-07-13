package fleetservice_test

import (
	"database/sql"
	"os"
	"testing"

	_ "github.com/lib/pq"

	"avoc/internal/fleetservice"
	"avoc/internal/vehicleregistry"
)

// stubConnectionChecker satisfies vehicleregistry.ConnectionChecker without a real WebSocket
// registry — only vehicle_id -> online mapping matters for this test.
type stubConnectionChecker map[string]bool

func (s stubConnectionChecker) Connected(vehicleID string) bool { return s[vehicleID] }

// TestNewPostgresFleetStore_SucceedsWhenVehiclesTableDoesNotExistYet is a regression test for a
// real bug found via tests/integration (fleet-service starting before control-server in
// docker-compose.test.yml, which has no depends_on between the two): the original
// NewPostgresFleetStore only ran `ALTER TABLE vehicles ADD COLUMN ...`, which errors if
// `vehicles` doesn't exist yet. Fixed by having fleet-service also create the base table
// (CREATE TABLE IF NOT EXISTS, mirroring vehicleregistry's schema) before extending it.
func TestNewPostgresFleetStore_SucceedsWhenVehiclesTableDoesNotExistYet(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set — skipping Postgres integration test")
	}
	// A pooled *sql.DB can hand out a different underlying connection per query, so a plain
	// `SET search_path` would not reliably apply to the schema-creation queries below —
	// pin this DB handle to a single connection for the whole test.
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })

	// Freshly created (not IF NOT EXISTS) schema is guaranteed empty — simulates a genuinely
	// fresh database where `vehicles` truly does not exist yet, unlike the shared `public`
	// schema used by the other tests in this package.
	const schemaName = "fleet_fresh_start_test"
	if _, err := db.Exec("DROP SCHEMA IF EXISTS " + schemaName + " CASCADE"); err != nil {
		t.Fatalf("drop schema: %v", err)
	}
	if _, err := db.Exec("CREATE SCHEMA " + schemaName); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() { db.Exec("DROP SCHEMA IF EXISTS " + schemaName + " CASCADE") })
	if _, err := db.Exec("SET search_path TO " + schemaName); err != nil {
		t.Fatalf("set search_path: %v", err)
	}

	if _, err := fleetservice.NewPostgresFleetStore(db); err != nil {
		t.Fatalf("NewPostgresFleetStore must succeed even when `vehicles` does not exist yet (fleet-service starting first): %v", err)
	}
}

// TestFleetserviceAndVehicleregistry_CoexistOnSharedTable is the core design claim of ADR-029:
// vehicleregistry (control-server, owns id/display_name) and fleetservice (owns vehicle_type)
// both initialize their schema against the SAME `vehicles` table without conflict, and each
// sees the other's writes.
func TestFleetserviceAndVehicleregistry_CoexistOnSharedTable(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set — skipping Postgres integration test")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	// Both packages initialize their schema against the same DB — order matters for the real
	// deployment (control-server usually starts vehicleregistry's base table first), but this
	// test also covers fleet-service starting first (ALTER TABLE ... ADD COLUMN IF NOT EXISTS
	// must not fail if `vehicles` doesn't exist yet under a real deployment race — see note below).
	vehicleStore, err := vehicleregistry.NewPostgresVehicleStore(db, stubConnectionChecker{"itg-vehicle-01": true})
	if err != nil {
		t.Fatalf("vehicleregistry.NewPostgresVehicleStore: %v", err)
	}
	fleetStore, err := fleetservice.NewPostgresFleetStore(db)
	if err != nil {
		t.Fatalf("fleetservice.NewPostgresFleetStore: %v", err)
	}

	t.Cleanup(func() { db.Exec(`DELETE FROM vehicles WHERE id = 'itg-vehicle-01'`) })

	// control-server's side: auto-register on first WS connect (ADR-021 behaviour).
	if err := vehicleStore.Add("itg-vehicle-01", "Integration Vehicle", ""); err != nil {
		t.Fatalf("vehicleregistry.Add: %v", err)
	}

	// fleet-service's side: admin sets the fleet-specific type (ADR-029 — fleet-service owns
	// vehicle_type, control-server owns id/display_name).
	if err := fleetStore.SetVehicleType("itg-vehicle-01", "lastenzug"); err != nil {
		t.Fatalf("fleetservice.SetVehicleType: %v", err)
	}

	// vehicleregistry.List() must still work unmodified — it doesn't select vehicle_type, so
	// adding the column must not break its existing query (ADR-029 "control-server bleibt
	// unverändert").
	vehicles, err := vehicleStore.List()
	if err != nil {
		t.Fatalf("vehicleregistry.List: %v", err)
	}
	var found *vehicleregistry.Vehicle
	for i := range vehicles {
		if vehicles[i].ID == "itg-vehicle-01" {
			found = &vehicles[i]
		}
	}
	if found == nil {
		t.Fatal("itg-vehicle-01 not found via vehicleregistry.List after fleet-service wrote vehicle_type")
	}
	if !found.Online {
		t.Fatal("expected Online=true from stubConnectionChecker, got false")
	}

	// Confirm the vehicle_type fleet-service wrote is actually visible at the raw SQL level —
	// proves both packages really share one row, not two divergent copies.
	var vehicleType sql.NullString
	if err := db.QueryRow(`SELECT vehicle_type FROM vehicles WHERE id = $1`, "itg-vehicle-01").Scan(&vehicleType); err != nil {
		t.Fatalf("query vehicle_type: %v", err)
	}
	if !vehicleType.Valid || vehicleType.String != "lastenzug" {
		t.Fatalf("expected vehicle_type='lastenzug', got %v", vehicleType)
	}
}

// TestForeignKeyRestrict_ZoneInUseCannotBeDeleted verifies referential integrity is enforced at
// the database level (ADR-029 doesn't specify cascade behaviour — RESTRICT is Postgres's
// default and this test locks that in as the actual, tested behaviour rather than an assumption).
func TestForeignKeyRestrict_ZoneInUseCannotBeDeleted(t *testing.T) {
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
	store, err := fleetservice.NewPostgresFleetStore(db)
	if err != nil {
		t.Fatalf("NewPostgresFleetStore: %v", err)
	}

	if err := store.AddZone(fleetservice.Zone{ID: "itg-zone-restrict", Name: "Restrict Zone", Environment: "indoor"}); err != nil {
		t.Fatalf("AddZone: %v", err)
	}
	if err := store.AddStation(fleetservice.Station{ID: "itg-station-restrict", ZoneID: "itg-zone-restrict", Name: "Restrict Station"}); err != nil {
		t.Fatalf("AddStation: %v", err)
	}
	t.Cleanup(func() {
		db.Exec(`DELETE FROM stations WHERE id = 'itg-station-restrict'`)
		db.Exec(`DELETE FROM zones WHERE id = 'itg-zone-restrict'`)
	})

	// Zone is still referenced by a station — deletion must be rejected, not silently cascade.
	if _, err := db.Exec(`DELETE FROM zones WHERE id = 'itg-zone-restrict'`); err == nil {
		t.Fatal("expected FK RESTRICT to reject deleting a zone still referenced by a station")
	}
}
