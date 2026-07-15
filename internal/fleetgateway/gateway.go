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
// stable even if the persistence schema changes. JSON tags double as the MQTT wire format
// (FLEET-04/05) — the concrete stand-in for the still-unspecified ROS2/DDS transport.
type VehicleStatusEvent struct {
	VehicleID      string    `json:"vehicle_id"`
	BatteryPct     *float64  `json:"battery_pct,omitempty"`
	Speed          *float64  `json:"speed,omitempty"`
	PositionLat    *float64  `json:"position_lat,omitempty"`
	PositionLon    *float64  `json:"position_lon,omitempty"`
	PositionZoneID *string   `json:"position_zone_id,omitempty"`
	AutonomyMode   string    `json:"autonomy_mode"` // "autonomous" | "teleoperated" | "manual"
	CurrentTaskID  *string   `json:"current_task_id,omitempty"`
	Timestamp      time.Time `json:"timestamp"`
}

// VehicleAlertEvent is a problem the vehicle detected on its own and cannot resolve without
// operator intervention (ADR-028 Notfall-Trigger-Modell) — distinct from threshold-based alerts
// that fleet-service computes itself from VehicleStatusEvent (FLEET-07).
type VehicleAlertEvent struct {
	VehicleID string    `json:"vehicle_id"`
	Severity  string    `json:"severity"` // "info" | "warning" | "critical"
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// TaskAssignment is dispatched to a vehicle when an operator or fleet-service assigns it a new
// route between two stations — outbound direction (Leitstelle -> vehicle).
type TaskAssignment struct {
	TaskID        string `json:"task_id"`
	VehicleID     string `json:"vehicle_id"`
	FromStationID string `json:"from_station_id"`
	ToStationID   string `json:"to_station_id"`
	Priority      int    `json:"priority"`
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
