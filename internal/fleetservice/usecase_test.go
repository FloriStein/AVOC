package fleetservice

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"avoc/internal/fleetgateway"
)

// ─── Stub Dispatcher (internal-package copy — fleetservice_test's stubDispatcher in
// handler_test.go is unexported to that external package and unreachable from here) ───────────

type ucStubDispatcher struct {
	lastTask   fleetgateway.TaskAssignment
	dispatched bool
	err        error
}

func (d *ucStubDispatcher) DispatchTask(task fleetgateway.TaskAssignment) error {
	d.lastTask = task
	d.dispatched = true
	return d.err
}

func newUCFakeStore(t *testing.T) *FakeFleetStore {
	t.Helper()
	store := NewFakeFleetStore()
	if err := store.EnsureVehicleExists("uc-test-vehicle"); err != nil {
		t.Fatalf("EnsureVehicleExists: %v", err)
	}
	if err := store.AddZone(Zone{ID: "uc-test-zone", Name: "Zone", Environment: "indoor"}); err != nil {
		t.Fatalf("AddZone: %v", err)
	}
	if err := store.AddStation(Station{ID: "uc-test-station-a", ZoneID: "uc-test-zone", Name: "A"}); err != nil {
		t.Fatalf("AddStation A: %v", err)
	}
	if err := store.AddStation(Station{ID: "uc-test-station-b", ZoneID: "uc-test-zone", Name: "B"}); err != nil {
		t.Fatalf("AddStation B: %v", err)
	}
	return store
}

// readOneEvent dials nothing itself — it reads exactly one already-broadcast message off an
// existing connected test client and decodes it as a WSEvent.
func readOneEvent(t *testing.T, client interface {
	ReadMessage() (int, []byte, error)
}) WSEvent {
	t.Helper()
	_, raw, err := client.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	var evt WSEvent
	if err := json.Unmarshal(raw, &evt); err != nil {
		t.Fatalf("unmarshal WSEvent: %v", err)
	}
	return evt
}

// ─── createAndDispatchTask ──────────────────────────────────────────────────────

func TestCreateAndDispatchTask_Success_PersistsAndDispatches(t *testing.T) {
	store := newUCFakeStore(t)
	dispatcher := &ucStubDispatcher{}

	created, err := createAndDispatchTask(store, dispatcher, Task{
		VehicleID: "uc-test-vehicle", FromStationID: "uc-test-station-a", ToStationID: "uc-test-station-b",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !dispatcher.dispatched {
		t.Fatal("expected DispatchTask to be called")
	}
	if dispatcher.lastTask.TaskID != created.ID || dispatcher.lastTask.VehicleID != "uc-test-vehicle" {
		t.Fatalf("dispatched task mismatch: %+v vs created %+v", dispatcher.lastTask, created)
	}
}

// ADR-027: dispatch is fire-and-forget — a Dispatcher error must not fail task creation.
func TestCreateAndDispatchTask_DispatchFails_TaskStillPersisted(t *testing.T) {
	store := newUCFakeStore(t)
	dispatcher := &ucStubDispatcher{err: errors.New("gateway unreachable")}

	created, err := createAndDispatchTask(store, dispatcher, Task{
		VehicleID: "uc-test-vehicle", FromStationID: "uc-test-station-a", ToStationID: "uc-test-station-b",
	})
	if err != nil {
		t.Fatalf("expected no error despite dispatch failure, got %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected task to be persisted with an ID")
	}
	if !dispatcher.dispatched {
		t.Fatal("expected DispatchTask to still be called")
	}
}

func TestCreateAndDispatchTask_StoreFails_DispatcherNotCalled(t *testing.T) {
	store := newUCFakeStore(t)
	dispatcher := &ucStubDispatcher{}

	// Unknown vehicle ID -> store.CreateTask fails (FK violation, mirrors Postgres behavior).
	_, err := createAndDispatchTask(store, dispatcher, Task{
		VehicleID: "does-not-exist", FromStationID: "uc-test-station-a", ToStationID: "uc-test-station-b",
	})
	if err == nil {
		t.Fatal("expected error for unknown vehicle ID")
	}
	if dispatcher.dispatched {
		t.Fatal("expected DispatchTask NOT to be called when persistence fails")
	}
}

// ─── transitionTaskStatus ───────────────────────────────────────────────────────

func TestTransitionTaskStatus_Success_BroadcastsUpdate(t *testing.T) {
	store := newUCFakeStore(t)
	created, err := store.CreateTask(Task{VehicleID: "uc-test-vehicle", FromStationID: "uc-test-station-a", ToStationID: "uc-test-station-b"})
	if err != nil {
		t.Fatalf("seed CreateTask: %v", err)
	}

	hub := NewHub()
	wsURL, cleanup := newTestHubServer(t, hub)
	defer cleanup()
	client := dialTestClient(t, wsURL)
	waitForClientCount(t, hub, 1, time.Second)

	updated, err := transitionTaskStatus(store, hub, created.ID, "in_progress", "operator-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Status != "in_progress" {
		t.Fatalf("expected status in_progress, got %q", updated.Status)
	}

	evt := readOneEvent(t, client)
	if evt.Type != "task_status_changed" {
		t.Fatalf("expected task_status_changed broadcast, got %q", evt.Type)
	}
}

func TestTransitionTaskStatus_UnknownID_ReturnsErrorAndNoBroadcast(t *testing.T) {
	store := newUCFakeStore(t)

	hub := NewHub()
	wsURL, cleanup := newTestHubServer(t, hub)
	defer cleanup()
	client := dialTestClient(t, wsURL)
	waitForClientCount(t, hub, 1, time.Second)

	_, err := transitionTaskStatus(store, hub, "does-not-exist", "in_progress", "operator-1")
	if !errors.Is(err, ErrTaskNotFound) {
		t.Fatalf("expected ErrTaskNotFound, got %v", err)
	}

	// Marker broadcast proves the error path above sent nothing — if it had, this would be the
	// second message read instead of the first.
	hub.Broadcast("marker", nil)
	evt := readOneEvent(t, client)
	if evt.Type != "marker" {
		t.Fatalf("expected only the marker broadcast, got %q first — error path broadcast unexpectedly", evt.Type)
	}
}

// ─── acknowledgeAlert ───────────────────────────────────────────────────────────

func TestAcknowledgeAlert_Success_Broadcasts(t *testing.T) {
	store := newUCFakeStore(t)
	alert, err := store.CreateAlert(Alert{VehicleID: "uc-test-vehicle", Severity: "warning", Message: "test alert"})
	if err != nil {
		t.Fatalf("seed CreateAlert: %v", err)
	}

	hub := NewHub()
	wsURL, cleanup := newTestHubServer(t, hub)
	defer cleanup()
	client := dialTestClient(t, wsURL)
	waitForClientCount(t, hub, 1, time.Second)

	if err := acknowledgeAlert(store, hub, alert.ID, "operator-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	evt := readOneEvent(t, client)
	if evt.Type != "alert_acknowledged" {
		t.Fatalf("expected alert_acknowledged broadcast, got %q", evt.Type)
	}
}

func TestAcknowledgeAlert_UnknownID_ReturnsErrorAndNoBroadcast(t *testing.T) {
	store := newUCFakeStore(t)

	hub := NewHub()
	wsURL, cleanup := newTestHubServer(t, hub)
	defer cleanup()
	client := dialTestClient(t, wsURL)
	waitForClientCount(t, hub, 1, time.Second)

	if err := acknowledgeAlert(store, hub, "does-not-exist", "operator-1"); err == nil {
		t.Fatal("expected error for unknown alert ID")
	}

	hub.Broadcast("marker", nil)
	evt := readOneEvent(t, client)
	if evt.Type != "marker" {
		t.Fatalf("expected only the marker broadcast, got %q first — error path broadcast unexpectedly", evt.Type)
	}
}
