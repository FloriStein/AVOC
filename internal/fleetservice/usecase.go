package fleetservice

import (
	"time"

	"avoc/internal/fleetgateway"
)

// This file holds the fleet-service use cases with real orchestration logic — coordinating a
// store write with a second side effect (dispatch, dashboard broadcast) — extracted out of
// Handler so they're testable without HTTP (ADR-031, HEX-06). Pure CRUD endpoints (ListVehicles,
// ListZones, ListStations, ListTasks, ListAlerts, GetTaskStatusHistory,
// GetVehiclePositionHistory) are 1:1 store pass-throughs with no orchestration of their own and
// deliberately stay inline in Handler — a use-case wrapper there would only add indirection.

// createAndDispatchTask persists t and dispatches it to the vehicle via gw (ADR-027,
// fire-and-forget — dispatch error intentionally not surfaced as a failure, the task is already
// persisted and remains visible/retriable via the Dashboard regardless of dispatch outcome).
func createAndDispatchTask(store FleetStore, gw Dispatcher, t Task) (Task, error) {
	created, err := store.CreateTask(t)
	if err != nil {
		return Task{}, err
	}
	_ = gw.DispatchTask(fleetgateway.TaskAssignment{
		TaskID: created.ID, VehicleID: created.VehicleID,
		FromStationID: created.FromStationID, ToStationID: created.ToStationID, Priority: created.Priority,
	})
	return created, nil
}

// transitionTaskStatus applies the status change (store.UpdateTaskStatus carries the ADR-030
// transition validation) and, on success, notifies connected Dashboard clients (FLEET-06) —
// bundled here because a status transition without the broadcast is an incomplete operation:
// every caller of this use case needs both effects.
func transitionTaskStatus(store FleetStore, hub *Hub, id, newStatus, changedBy string) (Task, error) {
	updated, err := store.UpdateTaskStatus(id, newStatus, changedBy)
	if err != nil {
		return Task{}, err
	}
	hub.Broadcast("task_status_changed", TaskStatusChangedEvent{
		ID:          updated.ID,
		Status:      updated.Status,
		CompletedAt: updated.CompletedAt,
		ChangedBy:   changedBy,
	})
	return updated, nil
}

// acknowledgeAlert records the acknowledgement and notifies connected Dashboard clients
// (FLEET-06), same rationale as transitionTaskStatus.
func acknowledgeAlert(store FleetStore, hub *Hub, id, ackBy string) error {
	if err := store.AcknowledgeAlert(id, ackBy); err != nil {
		return err
	}
	hub.Broadcast("alert_acknowledged", AlertAcknowledgedEvent{
		ID:             id,
		AcknowledgedBy: ackBy,
		AcknowledgedAt: time.Now(),
	})
	return nil
}
