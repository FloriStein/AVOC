package fleetservice

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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

// TestListTasks_ReturnsEmptySliceNotNull_WhenNoTasksExist is a regression test for a real crash
// found via manual browser verification (TASK-14 follow-up): ListTasks used `var tasks []Task`,
// which stays a nil slice when the query returns zero rows — encoding/json marshals a nil slice
// as `null`, not `[]`. The frontend (FleetTaskPanel.tsx) called `tasks.length` on that `null` and
// crashed the entire Fleet Overview page (no error boundary). A freshly created, guaranteed-empty
// schema (same isolation pattern as TestNewPostgresFleetStore_SucceedsWhenVehiclesTableDoesNotExistYet
// in integration_test.go) is required here — the shared `public` schema used by other tests in
// this package always has pre-existing tasks, which would hide this exact bug.
func TestListTasks_ReturnsEmptySliceNotNull_WhenNoTasksExist(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set — skipping Postgres integration test")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })

	const schemaName = "fleet_list_tasks_empty_test"
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

	store, err := NewPostgresFleetStore(db)
	if err != nil {
		t.Fatalf("NewPostgresFleetStore: %v", err)
	}

	tasks, err := store.ListTasks()
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if tasks == nil {
		t.Fatal("ListTasks must return a non-nil empty slice when no tasks exist, got nil — this marshals to JSON `null` and crashes FleetTaskPanel.tsx's tasks.length check")
	}
	if len(tasks) != 0 {
		t.Fatalf("expected 0 tasks in freshly created schema, got %d", len(tasks))
	}

	body, err := json.Marshal(tasks)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if string(body) != "[]" {
		t.Fatalf("expected JSON `[]` for empty task list, got %q (this is exactly what crashes the frontend)", body)
	}
}

// TestUpdateTaskStatus_ConcurrentTransitions_ExactlyOneSucceeds is the race-safety proof ADR-030
// requires (Teststandard §17): N goroutines race the same pending->in_progress transition on one
// task. The atomic `WHERE id=$1 AND status = ANY($2)` UPDATE (not a read-then-write) must let
// exactly one caller "win" — any read-then-write implementation could let multiple callers read
// "pending" before any of them writes, corrupting the state machine's single-transition guarantee.
// Run with -race (like this package's other concurrency tests, e.g. alertengine_test.go).
func TestUpdateTaskStatus_ConcurrentTransitions_ExactlyOneSucceeds(t *testing.T) {
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
	if _, err := db.Exec(`INSERT INTO vehicles (id, display_name) VALUES ('race-test-vehicle', 'Race Test Vehicle')
		ON CONFLICT (id) DO NOTHING`); err != nil {
		t.Fatalf("seed vehicle: %v", err)
	}

	store, err := NewPostgresFleetStore(db)
	if err != nil {
		t.Fatalf("NewPostgresFleetStore: %v", err)
	}
	if err := store.AddZone(Zone{ID: "race-test-zone", Name: "Race Zone", Environment: "indoor"}); err != nil {
		t.Fatalf("AddZone: %v", err)
	}
	if err := store.AddStation(Station{ID: "race-test-station-a", ZoneID: "race-test-zone", Name: "A"}); err != nil {
		t.Fatalf("AddStation A: %v", err)
	}
	if err := store.AddStation(Station{ID: "race-test-station-b", ZoneID: "race-test-zone", Name: "B"}); err != nil {
		t.Fatalf("AddStation B: %v", err)
	}
	task, err := store.CreateTask(Task{VehicleID: "race-test-vehicle", FromStationID: "race-test-station-a", ToStationID: "race-test-station-b"})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	t.Cleanup(func() {
		db.Exec(`DELETE FROM tasks WHERE vehicle_id = 'race-test-vehicle'`)
		db.Exec(`DELETE FROM stations WHERE zone_id = 'race-test-zone'`)
		db.Exec(`DELETE FROM zones WHERE id = 'race-test-zone'`)
		db.Exec(`DELETE FROM vehicles WHERE id = 'race-test-vehicle'`)
	})

	const attempts = 20
	var successes, conflicts int32
	var wg sync.WaitGroup
	wg.Add(attempts)
	for i := 0; i < attempts; i++ {
		go func(i int) {
			defer wg.Done()
			_, err := store.UpdateTaskStatus(task.ID, "in_progress", fmt.Sprintf("operator-%d", i))
			switch {
			case err == nil:
				atomic.AddInt32(&successes, 1)
			case errors.Is(err, ErrInvalidTransition):
				atomic.AddInt32(&conflicts, 1)
			default:
				t.Errorf("unexpected error from goroutine %d: %v", i, err)
			}
		}(i)
	}
	wg.Wait()

	if successes != 1 {
		t.Fatalf("expected exactly 1 successful transition among %d concurrent attempts, got %d (conflicts=%d)", attempts, successes, conflicts)
	}
	if conflicts != attempts-1 {
		t.Fatalf("expected %d conflicts, got %d", attempts-1, conflicts)
	}

	tasks, err := store.ListTasks()
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	for _, tk := range tasks {
		if tk.ID == task.ID && tk.Status != "in_progress" {
			t.Fatalf("expected final status in_progress, got %q", tk.Status)
		}
	}
}

// ─── TaskStatusHistory (ADR-032, Postgres required) ────────────────────────────

// taskStatusHistoryTestFixture creates a fresh vehicle/zone/2 stations/task for one test's
// exclusive use — same rationale as newTaskStatusTestFixture in handler_test.go (this file is
// package fleetservice, so it can't reuse that unexported-field-free but still test-local helper).
func taskStatusHistoryTestFixture(t *testing.T, db *sql.DB, store *PostgresFleetStore) Task {
	t.Helper()
	suffix := t.Name()
	vehicleID := "history-test-vehicle-" + suffix
	zoneID := "history-test-zone-" + suffix
	stationAID := "history-test-station-a-" + suffix
	stationBID := "history-test-station-b-" + suffix

	if _, err := db.Exec(`INSERT INTO vehicles (id, display_name) VALUES ($1, $1) ON CONFLICT (id) DO NOTHING`, vehicleID); err != nil {
		t.Fatalf("seed vehicle: %v", err)
	}
	if err := store.AddZone(Zone{ID: zoneID, Name: "Zone", Environment: "indoor"}); err != nil {
		t.Fatalf("AddZone: %v", err)
	}
	if err := store.AddStation(Station{ID: stationAID, ZoneID: zoneID, Name: "A"}); err != nil {
		t.Fatalf("AddStation A: %v", err)
	}
	if err := store.AddStation(Station{ID: stationBID, ZoneID: zoneID, Name: "B"}); err != nil {
		t.Fatalf("AddStation B: %v", err)
	}
	task, err := store.CreateTask(Task{VehicleID: vehicleID, FromStationID: stationAID, ToStationID: stationBID})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	t.Cleanup(func() {
		db.Exec(`DELETE FROM task_status_history WHERE task_id = $1`, task.ID)
		db.Exec(`DELETE FROM tasks WHERE id = $1`, task.ID)
		db.Exec(`DELETE FROM stations WHERE zone_id = $1`, zoneID)
		db.Exec(`DELETE FROM zones WHERE id = $1`, zoneID)
		db.Exec(`DELETE FROM vehicles WHERE id = $1`, vehicleID)
	})
	return task
}

// TestUpdateTaskStatus_RecordsHistoryEntry_WithCorrectFromAndToStatus proves ADR-032's core
// write path: a PATCH-driven transition appends exactly one task_status_history row with the
// true prior status (read atomically via the UPDATE's CTE, not guessed).
func TestUpdateTaskStatus_RecordsHistoryEntry_WithCorrectFromAndToStatus(t *testing.T) {
	db, store := openTestStore(t)
	task := taskStatusHistoryTestFixture(t, db, store)

	if _, err := store.UpdateTaskStatus(task.ID, "in_progress", "operator-1"); err != nil {
		t.Fatalf("UpdateTaskStatus: %v", err)
	}

	history, err := store.GetTaskStatusHistory(task.ID)
	if err != nil {
		t.Fatalf("GetTaskStatusHistory: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(history))
	}
	e := history[0]
	if e.FromStatus == nil || *e.FromStatus != "pending" {
		t.Fatalf("expected from_status=pending, got %v", e.FromStatus)
	}
	if e.ToStatus != "in_progress" || e.ChangedBy != "operator-1" || e.TaskID != task.ID {
		t.Fatalf("unexpected history entry: %+v", e)
	}
}

// TestGetTaskStatusHistory_MultipleTransitions_ChronologicalOrder proves entries accumulate (not
// overwrite, unlike tasks.status_changed_by) and are returned oldest-first for a timeline view.
func TestGetTaskStatusHistory_MultipleTransitions_ChronologicalOrder(t *testing.T) {
	db, store := openTestStore(t)
	task := taskStatusHistoryTestFixture(t, db, store)

	if _, err := store.UpdateTaskStatus(task.ID, "in_progress", "operator-1"); err != nil {
		t.Fatalf("transition 1: %v", err)
	}
	if _, err := store.UpdateTaskStatus(task.ID, "completed", "operator-2"); err != nil {
		t.Fatalf("transition 2: %v", err)
	}

	history, err := store.GetTaskStatusHistory(task.ID)
	if err != nil {
		t.Fatalf("GetTaskStatusHistory: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(history))
	}
	if history[0].ToStatus != "in_progress" || history[1].ToStatus != "completed" {
		t.Fatalf("expected chronological order [in_progress, completed], got [%s, %s]", history[0].ToStatus, history[1].ToStatus)
	}
	if history[1].FromStatus == nil || *history[1].FromStatus != "in_progress" {
		t.Fatalf("expected second entry's from_status=in_progress, got %v", history[1].FromStatus)
	}
}

// TestGetTaskStatusHistory_EmptyNonNil_ForTaskWithNoTransitionsYet mirrors the TASKUI-04
// nil-slice convention: a still-pending task (never PATCHed) has no history rows, but the
// response must still be `[]`, not `null` — the same class of frontend crash TASKUI-04 fixed for
// ListTasks/ListZones/etc.
func TestGetTaskStatusHistory_EmptyNonNil_ForTaskWithNoTransitionsYet(t *testing.T) {
	db, store := openTestStore(t)
	task := taskStatusHistoryTestFixture(t, db, store)

	history, err := store.GetTaskStatusHistory(task.ID)
	if err != nil {
		t.Fatalf("GetTaskStatusHistory: %v", err)
	}
	if history == nil {
		t.Fatal("expected non-nil empty slice for a task with no transitions yet, got nil (marshals to JSON null)")
	}
	if len(history) != 0 {
		t.Fatalf("expected 0 entries for a never-transitioned task, got %d", len(history))
	}
}

func TestGetTaskStatusHistory_NotFound_ForNonexistentTask(t *testing.T) {
	_, store := openTestStore(t)

	_, err := store.GetTaskStatusHistory("does-not-exist-" + t.Name())
	if !errors.Is(err, ErrTaskNotFound) {
		t.Fatalf("expected ErrTaskNotFound, got %v", err)
	}
}

// TestBackfillTaskStatusHistory_PopulatesLegacyTransitionsIdempotently verifies the ADR-032
// rollout migration against tasks that transitioned before task_status_history existed (rows
// inserted directly via raw SQL, bypassing CreateTask/UpdateTaskStatus, to simulate genuinely
// pre-existing data) — and that a repeated startup does not duplicate the backfilled rows.
// Isolated schema (like TestListTasks_ReturnsEmptySliceNotNull_WhenNoTasksExist) so pre-existing
// rows in the shared `public` schema from other tests can't interfere with the legacy-data setup.
func TestBackfillTaskStatusHistory_PopulatesLegacyTransitionsIdempotently(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set — skipping Postgres integration test")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })

	const schemaName = "fleet_backfill_history_test"
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

	// First startup bootstraps the (empty) schema — no legacy tasks exist yet, so this is a no-op
	// backfill run, exercising that NewPostgresFleetStore tolerates having nothing to do.
	if _, err := NewPostgresFleetStore(db); err != nil {
		t.Fatalf("bootstrap store: %v", err)
	}

	if _, err := db.Exec(`INSERT INTO vehicles (id, display_name) VALUES ('legacy-vehicle', 'Legacy Vehicle')`); err != nil {
		t.Fatalf("seed vehicle: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO zones (id, name, environment) VALUES ('legacy-zone', 'Zone', 'indoor')`); err != nil {
		t.Fatalf("seed zone: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO stations (id, zone_id, name) VALUES ('legacy-station-a', 'legacy-zone', 'A'), ('legacy-station-b', 'legacy-zone', 'B')`); err != nil {
		t.Fatalf("seed stations: %v", err)
	}

	completedAt := time.Now().Add(-2 * time.Hour).Truncate(time.Second)
	createdAt := time.Now().Add(-3 * time.Hour).Truncate(time.Second)
	if _, err := db.Exec(
		`INSERT INTO tasks (id, vehicle_id, from_station_id, to_station_id, status, status_changed_by, completed_at, created_at)
		 VALUES ('legacy-task-completed', 'legacy-vehicle', 'legacy-station-a', 'legacy-station-b', 'completed', 'legacy-operator', $1, $2)`,
		completedAt, createdAt,
	); err != nil {
		t.Fatalf("seed legacy completed task: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO tasks (id, vehicle_id, from_station_id, to_station_id, status, status_changed_by, created_at)
		 VALUES ('legacy-task-cancelled', 'legacy-vehicle', 'legacy-station-a', 'legacy-station-b', 'cancelled', 'legacy-operator', $1)`,
		createdAt,
	); err != nil {
		t.Fatalf("seed legacy cancelled task: %v", err)
	}

	// Second startup is the actual backfill under test — mirrors the real rollout (existing dev
	// DB with pre-ADR-032 tasks, service restarts once the migration ships).
	store, err := NewPostgresFleetStore(db)
	if err != nil {
		t.Fatalf("NewPostgresFleetStore (backfill run): %v", err)
	}

	completedHistory, err := store.GetTaskStatusHistory("legacy-task-completed")
	if err != nil {
		t.Fatalf("GetTaskStatusHistory (completed): %v", err)
	}
	if len(completedHistory) != 1 {
		t.Fatalf("expected 1 backfilled entry for completed task, got %d", len(completedHistory))
	}
	ce := completedHistory[0]
	if ce.FromStatus == nil || *ce.FromStatus != "in_progress" {
		t.Fatalf("expected backfilled from_status=in_progress for a completed task, got %v", ce.FromStatus)
	}
	if ce.ToStatus != "completed" || ce.ChangedBy != "legacy-operator" {
		t.Fatalf("unexpected backfilled entry: %+v", ce)
	}
	if ce.ChangedAt.Unix() != completedAt.Unix() {
		t.Fatalf("expected changed_at to use completed_at (%v), got %v", completedAt, ce.ChangedAt)
	}

	cancelledHistory, err := store.GetTaskStatusHistory("legacy-task-cancelled")
	if err != nil {
		t.Fatalf("GetTaskStatusHistory (cancelled): %v", err)
	}
	if len(cancelledHistory) != 1 {
		t.Fatalf("expected 1 backfilled entry for cancelled task, got %d", len(cancelledHistory))
	}
	xe := cancelledHistory[0]
	if xe.FromStatus != nil {
		t.Fatalf("expected from_status=nil for a backfilled cancelled task (ambiguous prior status), got %v", *xe.FromStatus)
	}
	if xe.ChangedAt.Unix() != createdAt.Unix() {
		t.Fatalf("expected changed_at to fall back to created_at (%v) for a cancelled task without completed_at, got %v", createdAt, xe.ChangedAt)
	}

	// Idempotency: a third startup (simulating another restart) must not duplicate rows.
	if _, err := NewPostgresFleetStore(db); err != nil {
		t.Fatalf("NewPostgresFleetStore (third run): %v", err)
	}
	completedHistoryAgain, err := store.GetTaskStatusHistory("legacy-task-completed")
	if err != nil {
		t.Fatalf("GetTaskStatusHistory after third run: %v", err)
	}
	if len(completedHistoryAgain) != 1 {
		t.Fatalf("expected backfill to stay idempotent (still 1 entry), got %d", len(completedHistoryAgain))
	}
}
