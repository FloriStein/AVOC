package fleetservice_test

import (
	"database/sql"
	"os"
	"testing"

	_ "github.com/lib/pq"

	"avoc/internal/fleetservice"
)

// TestVehicleLifecycle_AutonomyFirstFlow drives the full ADR-028 scenario through the data
// layer: a vehicle operates autonomously, detects a problem it cannot resolve itself, raises a
// vehicle-initiated alert, an operator takes over (autonomy_mode -> teleoperated), resolves the
// task, and hands control back to autonomy. This is the E2E precursor at the store level — real
// HTTP/WS E2E needs the API added in FLEET-02/05, but this proves the schema actually supports
// the documented workflow end-to-end, not just isolated CRUD per table.
func TestVehicleLifecycle_AutonomyFirstFlow(t *testing.T) {
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

	const vehicleID = "lifecycle-lastenzug-01"
	t.Cleanup(func() {
		db.Exec(`DELETE FROM alerts WHERE vehicle_id = $1`, vehicleID)
		db.Exec(`DELETE FROM vehicle_status WHERE vehicle_id = $1`, vehicleID)
		db.Exec(`DELETE FROM tasks WHERE vehicle_id = $1`, vehicleID)
		db.Exec(`DELETE FROM stations WHERE zone_id = 'lifecycle-zone'`)
		db.Exec(`DELETE FROM zones WHERE id = 'lifecycle-zone'`)
		db.Exec(`DELETE FROM vehicles WHERE id = $1`, vehicleID)
	})

	// 1. Vehicle registers (control-server auto-register on first WS connect, ADR-021) and
	//    fleet-service assigns its type (ADR-029).
	if _, err := db.Exec(`INSERT INTO vehicles (id, display_name) VALUES ($1, 'Lastenzug 01')`, vehicleID); err != nil {
		t.Fatalf("register vehicle: %v", err)
	}
	if err := store.SetVehicleType(vehicleID, "lastenzug"); err != nil {
		t.Fatalf("SetVehicleType: %v", err)
	}

	// 2. Admin sets up zone + two stations (AP3 "Räumliche Zonenzuweisung").
	if err := store.AddZone(fleetservice.Zone{ID: "lifecycle-zone", Name: "Halle 1", Environment: "indoor"}); err != nil {
		t.Fatalf("AddZone: %v", err)
	}
	if err := store.AddStation(fleetservice.Station{ID: "lifecycle-station-a", ZoneID: "lifecycle-zone", Name: "Wareneingang"}); err != nil {
		t.Fatalf("AddStation A: %v", err)
	}
	if err := store.AddStation(fleetservice.Station{ID: "lifecycle-station-b", ZoneID: "lifecycle-zone", Name: "Warenausgang"}); err != nil {
		t.Fatalf("AddStation B: %v", err)
	}

	// 3. A task is dispatched — vehicle moves from A to B (task model from requirements.md).
	task, err := store.CreateTask(fleetservice.Task{VehicleID: vehicleID, FromStationID: "lifecycle-station-a", ToStationID: "lifecycle-station-b", Priority: 1})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if task.Status != "pending" {
		t.Fatalf("expected new task status=pending, got %s", task.Status)
	}

	// 4. Vehicle drives autonomously — normal operating state (ADR-028: autonomy is the default).
	taskID := task.ID
	if err := store.UpsertVehicleStatus(fleetservice.VehicleStatus{
		VehicleID: vehicleID, AutonomyMode: "autonomous", CurrentTaskID: &taskID,
	}); err != nil {
		t.Fatalf("UpsertVehicleStatus (autonomous): %v", err)
	}

	// 5. Vehicle detects a problem it cannot resolve itself and raises an alert
	//    (vehicle-initiated, ADR-028 — not a fleet-service threshold alert).
	alert, err := store.CreateAlert(fleetservice.Alert{
		VehicleID: vehicleID, Severity: "critical", Message: "Hindernis auf der Strecke — Eingriff erforderlich",
	})
	if err != nil {
		t.Fatalf("CreateAlert: %v", err)
	}

	// 6. Operator sees the alert, selects the vehicle from the dropdown, and takes over
	//    (ADR-028 Notfall-Trigger-Modell — autonomy_mode flips to teleoperated).
	if err := store.UpsertVehicleStatus(fleetservice.VehicleStatus{
		VehicleID: vehicleID, AutonomyMode: "teleoperated", CurrentTaskID: &taskID,
	}); err != nil {
		t.Fatalf("UpsertVehicleStatus (teleoperated): %v", err)
	}
	if err := store.AcknowledgeAlert(alert.ID, "operator-1"); err != nil {
		t.Fatalf("AcknowledgeAlert: %v", err)
	}

	// 7. Operator resolves the problem (navigates around the obstacle — existing Direct-Teleop
	//    UI, out of scope here) and ends the session — endSession() suffices for now (ADR-028,
	//    handshake-based handover is FLEET-01 in backlog.md, deliberately deferred).
	if err := store.UpsertVehicleStatus(fleetservice.VehicleStatus{
		VehicleID: vehicleID, AutonomyMode: "autonomous", CurrentTaskID: &taskID,
	}); err != nil {
		t.Fatalf("UpsertVehicleStatus (back to autonomous): %v", err)
	}

	// 8. Task completes.
	if _, err := db.Exec(`UPDATE tasks SET status = 'completed', completed_at = NOW() WHERE id = $1`, taskID); err != nil {
		t.Fatalf("complete task: %v", err)
	}

	// Assertions: final state reflects the whole journey correctly.
	statuses, err := store.ListVehicleStatus()
	if err != nil {
		t.Fatalf("ListVehicleStatus: %v", err)
	}
	var final *fleetservice.VehicleStatus
	for i := range statuses {
		if statuses[i].VehicleID == vehicleID {
			final = &statuses[i]
		}
	}
	if final == nil {
		t.Fatal("vehicle status not found at end of lifecycle")
	}
	if final.AutonomyMode != "autonomous" {
		t.Fatalf("expected vehicle back in autonomous mode after handover, got %s", final.AutonomyMode)
	}

	tasks, err := store.ListTasks()
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	found := false
	for _, tsk := range tasks {
		if tsk.ID == taskID {
			found = true
			if tsk.Status != "completed" || tsk.CompletedAt == nil {
				t.Fatalf("expected task completed with timestamp, got status=%s completedAt=%v", tsk.Status, tsk.CompletedAt)
			}
		}
	}
	if !found {
		t.Fatal("task not found at end of lifecycle")
	}

	alerts, err := store.ListAlerts()
	if err != nil {
		t.Fatalf("ListAlerts: %v", err)
	}
	found = false
	for _, a := range alerts {
		if a.ID == alert.ID {
			found = true
			if a.AcknowledgedAt == nil || a.AcknowledgedBy == nil || *a.AcknowledgedBy != "operator-1" {
				t.Fatalf("expected alert acknowledged by operator-1, got %+v", a)
			}
		}
	}
	if !found {
		t.Fatal("alert not found at end of lifecycle")
	}
}
