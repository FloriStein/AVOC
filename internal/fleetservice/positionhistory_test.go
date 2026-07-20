package fleetservice

import (
	"database/sql"
	"errors"
	"fmt"
	"testing"
)

// positionHistoryTestVehicle seeds a bare vehicle identity row (position history only needs the
// FK target, not zones/stations/tasks like the task-history fixture) and registers cleanup.
func positionHistoryTestVehicle(t *testing.T, db *sql.DB) string {
	t.Helper()
	vehicleID := "position-history-test-vehicle-" + t.Name()
	if _, err := db.Exec(`INSERT INTO vehicles (id, display_name) VALUES ($1, $1) ON CONFLICT (id) DO NOTHING`, vehicleID); err != nil {
		t.Fatalf("seed vehicle: %v", err)
	}
	t.Cleanup(func() {
		db.Exec(`DELETE FROM vehicle_position_history WHERE vehicle_id = $1`, vehicleID)
		db.Exec(`DELETE FROM vehicles WHERE id = $1`, vehicleID)
	})
	return vehicleID
}

// TestRecordPositionHistory_WritesSample proves the basic write path: a lat/lon sample lands in
// vehicle_position_history for the given vehicle.
func TestRecordPositionHistory_WritesSample(t *testing.T) {
	db, store := openTestStore(t)
	vehicleID := positionHistoryTestVehicle(t, db)
	lat, lon := 52.13, 11.64

	if err := store.RecordPositionHistory(vehicleID, &lat, &lon); err != nil {
		t.Fatalf("RecordPositionHistory: %v", err)
	}

	points, err := store.GetVehiclePositionHistory(vehicleID)
	if err != nil {
		t.Fatalf("GetVehiclePositionHistory: %v", err)
	}
	if len(points) != 1 {
		t.Fatalf("expected 1 point, got %d", len(points))
	}
	if points[0].PositionLat != lat || points[0].PositionLon != lon || points[0].VehicleID != vehicleID {
		t.Fatalf("unexpected point: %+v", points[0])
	}
}

// TestRecordPositionHistory_NilPosition_NoOp proves a status update without a known position
// (e.g. the pre-existing Direct-Teleop vehicle, which never runs the fleet simulation) doesn't
// write a malformed/zero-value row.
func TestRecordPositionHistory_NilPosition_NoOp(t *testing.T) {
	db, store := openTestStore(t)
	vehicleID := positionHistoryTestVehicle(t, db)

	if err := store.RecordPositionHistory(vehicleID, nil, nil); err != nil {
		t.Fatalf("RecordPositionHistory: %v", err)
	}

	points, err := store.GetVehiclePositionHistory(vehicleID)
	if err != nil {
		t.Fatalf("GetVehiclePositionHistory: %v", err)
	}
	if len(points) != 0 {
		t.Fatalf("expected 0 points for a nil-position update, got %d", len(points))
	}
}

// TestRecordPositionHistory_Throttled_SkipsWriteWithinMinInterval proves ADR-033's throttle: a
// second sample arriving well within positionHistoryMinInterval of the last recorded one for the
// same vehicle is dropped, not appended — otherwise vehicle-mock's 3s status cadence would write
// far more rows than the map polyline needs.
func TestRecordPositionHistory_Throttled_SkipsWriteWithinMinInterval(t *testing.T) {
	db, store := openTestStore(t)
	vehicleID := positionHistoryTestVehicle(t, db)
	lat1, lon1 := 52.13, 11.64
	lat2, lon2 := 52.14, 11.65

	if err := store.RecordPositionHistory(vehicleID, &lat1, &lon1); err != nil {
		t.Fatalf("first RecordPositionHistory: %v", err)
	}
	if err := store.RecordPositionHistory(vehicleID, &lat2, &lon2); err != nil {
		t.Fatalf("second RecordPositionHistory: %v", err)
	}

	points, err := store.GetVehiclePositionHistory(vehicleID)
	if err != nil {
		t.Fatalf("GetVehiclePositionHistory: %v", err)
	}
	if len(points) != 1 {
		t.Fatalf("expected the immediate second sample to be throttled away (1 point), got %d", len(points))
	}
	if points[0].PositionLat != lat1 {
		t.Fatalf("expected the first sample to survive, got lat=%v", points[0].PositionLat)
	}
}

// TestRecordPositionHistory_WritesAgain_AfterMinIntervalElapsed proves the throttle is a rolling
// window, not a permanent "one sample per vehicle ever" limit — a sample whose predecessor is
// already older than positionHistoryMinInterval is recorded. The predecessor's recorded_at is
// backdated directly via SQL (not a real sleep) to keep the test fast and deterministic.
func TestRecordPositionHistory_WritesAgain_AfterMinIntervalElapsed(t *testing.T) {
	db, store := openTestStore(t)
	vehicleID := positionHistoryTestVehicle(t, db)
	lat1, lon1 := 52.13, 11.64
	lat2, lon2 := 52.14, 11.65

	if err := store.RecordPositionHistory(vehicleID, &lat1, &lon1); err != nil {
		t.Fatalf("first RecordPositionHistory: %v", err)
	}
	if _, err := db.Exec(
		`UPDATE vehicle_position_history SET recorded_at = NOW() - INTERVAL '11 seconds' WHERE vehicle_id = $1`,
		vehicleID,
	); err != nil {
		t.Fatalf("backdate first sample: %v", err)
	}
	if err := store.RecordPositionHistory(vehicleID, &lat2, &lon2); err != nil {
		t.Fatalf("second RecordPositionHistory: %v", err)
	}

	points, err := store.GetVehiclePositionHistory(vehicleID)
	if err != nil {
		t.Fatalf("GetVehiclePositionHistory: %v", err)
	}
	if len(points) != 2 {
		t.Fatalf("expected both samples once the interval elapsed, got %d", len(points))
	}
	if points[0].PositionLat != lat1 || points[1].PositionLat != lat2 {
		t.Fatalf("expected chronological order [lat1, lat2], got [%v, %v]", points[0].PositionLat, points[1].PositionLat)
	}
}

func TestGetVehiclePositionHistory_EmptyNonNil_ForVehicleWithNoSamplesYet(t *testing.T) {
	db, store := openTestStore(t)
	vehicleID := positionHistoryTestVehicle(t, db)

	points, err := store.GetVehiclePositionHistory(vehicleID)
	if err != nil {
		t.Fatalf("GetVehiclePositionHistory: %v", err)
	}
	if points == nil {
		t.Fatal("expected non-nil empty slice for a vehicle with no samples yet, got nil (marshals to JSON null)")
	}
	if len(points) != 0 {
		t.Fatalf("expected 0 points, got %d", len(points))
	}
}

func TestGetVehiclePositionHistory_NotFound_ForNonexistentVehicle(t *testing.T) {
	_, store := openTestStore(t)

	_, err := store.GetVehiclePositionHistory("does-not-exist-" + t.Name())
	if !errors.Is(err, ErrVehicleNotFound) {
		t.Fatalf("expected ErrVehicleNotFound, got %v", err)
	}
}

// TestVehiclePositionHistory_CascadesOnVehicleDelete guards against the exact regression ADR-032
// found for task_status_history: an FK without ON DELETE CASCADE breaks any future
// vehicle-deletion path (test cleanup today, potential admin tooling later) once a history row
// exists, failing on a RESTRICT constraint instead of cascading.
func TestVehiclePositionHistory_CascadesOnVehicleDelete(t *testing.T) {
	db, store := openTestStore(t)
	vehicleID := "cascade-test-vehicle-" + t.Name()
	if _, err := db.Exec(`INSERT INTO vehicles (id, display_name) VALUES ($1, $1)`, vehicleID); err != nil {
		t.Fatalf("seed vehicle: %v", err)
	}
	lat, lon := 52.13, 11.64
	if err := store.RecordPositionHistory(vehicleID, &lat, &lon); err != nil {
		t.Fatalf("RecordPositionHistory: %v", err)
	}

	if _, err := db.Exec(`DELETE FROM vehicles WHERE id = $1`, vehicleID); err != nil {
		t.Fatalf("expected DELETE to cascade via ON DELETE CASCADE, got error: %v", err)
	}

	var remaining int
	if err := db.QueryRow(`SELECT COUNT(*) FROM vehicle_position_history WHERE vehicle_id = $1`, vehicleID).Scan(&remaining); err != nil {
		t.Fatalf("count remaining history rows: %v", err)
	}
	if remaining != 0 {
		t.Fatalf("expected cascade delete to remove history rows, %d remain", remaining)
	}
}

// TestPruneVehiclePositionHistory_RemovesOnlySamplesOlderThanRetention proves ADR-033's retention
// cleanup is selective — a sample just past the retention window is removed, one just inside it
// survives. Both recorded_at values are set directly via SQL relative to
// positionHistoryRetention so the test doesn't depend on the constant's exact value.
func TestPruneVehiclePositionHistory_RemovesOnlySamplesOlderThanRetention(t *testing.T) {
	db, store := openTestStore(t)
	vehicleID := positionHistoryTestVehicle(t, db)

	retentionSeconds := int(positionHistoryRetention.Seconds())
	if _, err := db.Exec(
		fmt.Sprintf(
			`INSERT INTO vehicle_position_history (id, vehicle_id, position_lat, position_lon, recorded_at)
			 VALUES ('old-' || $1, $1, 1, 1, NOW() - INTERVAL '%d seconds' - INTERVAL '1 hour')`,
			retentionSeconds,
		),
		vehicleID,
	); err != nil {
		t.Fatalf("seed old sample: %v", err)
	}
	if _, err := db.Exec(
		fmt.Sprintf(
			`INSERT INTO vehicle_position_history (id, vehicle_id, position_lat, position_lon, recorded_at)
			 VALUES ('recent-' || $1, $1, 2, 2, NOW() - INTERVAL '%d seconds' + INTERVAL '1 hour')`,
			retentionSeconds,
		),
		vehicleID,
	); err != nil {
		t.Fatalf("seed recent sample: %v", err)
	}

	if err := store.PruneVehiclePositionHistory(); err != nil {
		t.Fatalf("PruneVehiclePositionHistory: %v", err)
	}

	points, err := store.GetVehiclePositionHistory(vehicleID)
	if err != nil {
		t.Fatalf("GetVehiclePositionHistory: %v", err)
	}
	if len(points) != 1 {
		t.Fatalf("expected exactly the recent sample to survive pruning, got %d points", len(points))
	}
	if points[0].PositionLat != 2 {
		t.Fatalf("expected the surviving sample to be the recent one (lat=2), got lat=%v", points[0].PositionLat)
	}
}
