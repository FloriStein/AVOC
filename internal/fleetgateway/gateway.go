// Package fleetgateway abstracts the external ROS2/DDS vehicle backend (ADR-027). The concrete
// interface (topic names, message format, DDS-vs-bridge) is not yet specified by the Professur
// Logistik — this package lets fleet-service (and the rest of the Leitstelle) develop against a
// stable Go interface now, with a Mock implementation, and swap in a real adapter later without
// touching callers. Consistent with the project's existing Interface-over-Implementation pattern
// (ADR-002 Safety Bus, ADR-005 Session Recording, ADR-020 MediaMTX).
package fleetgateway

import "time"

// VehicleStatusEvent is a live telemetry update from a vehicle (position, battery, autonomy
// mode) — inbound direction (vehicle -> Leitstelle). Field shape intentionally mirrors
// internal/fleetservice.VehicleStatus but is kept as a separate type: this package must stay
// stable even if the persistence schema changes.
type VehicleStatusEvent struct {
	VehicleID      string
	BatteryPct     *float64
	Speed          *float64
	PositionLat    *float64
	PositionLon    *float64
	PositionZoneID *string
	AutonomyMode   string // "autonomous" | "teleoperated" | "manual"
	CurrentTaskID  *string
	Timestamp      time.Time
}

// VehicleAlertEvent is a problem the vehicle detected on its own and cannot resolve without
// operator intervention (ADR-028 Notfall-Trigger-Modell) — distinct from threshold-based alerts
// that fleet-service computes itself from VehicleStatusEvent (FLEET-07).
type VehicleAlertEvent struct {
	VehicleID string
	Severity  string // "info" | "warning" | "critical"
	Message   string
	Timestamp time.Time
}

// TaskAssignment is dispatched to a vehicle when an operator or fleet-service assigns it a new
// route between two stations — outbound direction (Leitstelle -> vehicle).
type TaskAssignment struct {
	TaskID        string
	VehicleID     string
	FromStationID string
	ToStationID   string
	Priority      int
}

// FleetGateway is the abstraction boundary described in ADR-027. Implementations: MockGateway
// (this package, Sprint 21) now, a real ROS2/DDS adapter after the AP1 workshop with the
// Professur Logistik confirms the actual interface.
type FleetGateway interface {
	// SubscribeVehicleStatus registers fn to be called for every vehicle status update. Safe
	// for concurrent use; fn may be called from any goroutine and must not block long.
	SubscribeVehicleStatus(fn func(VehicleStatusEvent))

	// SubscribeVehicleAlerts registers fn to be called for every vehicle-initiated alert.
	SubscribeVehicleAlerts(fn func(VehicleAlertEvent))

	// DispatchTask sends a task assignment to a vehicle. The real adapter's error semantics are
	// unknown until the AP1 workshop (e.g. whether dispatch is fire-and-forget or ack-based) —
	// callers should treat a non-nil error as "not confirmed accepted by the vehicle".
	DispatchTask(task TaskAssignment) error
}
