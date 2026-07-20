// Package audit provides guaranteed-persistence storage for safety events (ADR-018).
// Safety events must NEVER be lost — they write synchronously via WriteSync()
// with fsync before the SAFE_MODE state transition fires.
package audit

import "time"

// SafetyAuditEvent is a safety-critical event written before every SAFE_MODE transition.
type SafetyAuditEvent struct {
	EventID     string
	SessionID   string
	VehicleID   string
	OperatorID  string
	EventType   string
	Reason      string
	SystemState string
	CtrlState   string
	Data        string // JSON-encoded extra data (optional)
	Timestamp   time.Time
}

// SafetyAuditWriter is the write-only view needed by safety-critical consumers
// (command.Engine, transport.WSHandler, vehiclecontext.Registry, DeadmanWatchdog,
// ACKTimeoutWatcher, VehicleACKWatchdog — GOSTYLE-IF-03). None of them query or
// close the writer, so those methods stay off this interface.
type SafetyAuditWriter interface {
	// WriteSync writes the event synchronously and fsyncs before returning.
	// Must be called BEFORE TransitionSystem(StateSafeMode).
	// A write error is logged but does NOT prevent the SAFE_MODE transition.
	WriteSync(event SafetyAuditEvent) error
}

// AuditWriter persists safety events with a durability guarantee (ADR-018) and
// additionally allows querying persisted events by session (used by the
// GET /audit/events handler in cmd/control-server/main.go). Close is
// deliberately not part of either interface (GOSTYLE-IF-03) — it is called
// exactly once, on the concrete *PostgresAuditWriter, during bootstrap
// shutdown in cmd/control-server/main.go's newAuditWriter, never through an
// interface value.
// Implementations: PostgresAuditWriter (production), NoopWriter (tests).
type AuditWriter interface {
	SafetyAuditWriter

	// QueryBySession returns all safety events for a session in timestamp order.
	QueryBySession(sessionID string) ([]SafetyAuditEvent, error)
}
