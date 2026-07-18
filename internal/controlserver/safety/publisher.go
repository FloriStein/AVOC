// Package safety implements the Control Server's Safety Decision Module (ADR-009).
// It classifies failures as CRITICAL/DEGRADED and enforces the three system invariants.
package safety

import "avoc/internal/safetyservice"

// Publisher sends safety events to the external Safety Event Bus service (ADR-002). Deliberately
// PublishEvent-only (GOSTYLE-IF-06) — every consumer that holds a Publisher (DeadmanWatchdog,
// ACKTimeoutWatcher, VehicleACKWatchdog, BusWatchdog, command.Engine, vehiclecontext.Registry)
// only ever calls PublishEvent. TriggerEmergencyStop is called directly on the concrete
// *HTTPPublisher in cmd/control-server/main.go, never through this interface, so it stays a
// concrete method on HTTPPublisher rather than being abstracted here.
// Implemented by HTTPPublisher in production and MockSafetyPublisher in tests.
type Publisher interface {
	PublishEvent(event safetyservice.SafetyEvent)
}
