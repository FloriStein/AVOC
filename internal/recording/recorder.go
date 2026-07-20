// Package recording implements Session Recording (BE-07, ADR-005/016).
// Storage backend is MemoryRecorder for now, replaceable via a future ADR
// (ADR-005 Folge: DB/Files/Object Storage) once a second implementation exists (GOSTYLE-IF-01).
package recording

import "time"

// Entry is a single recorded event within a session (ADR-005).
type Entry struct {
	SessionID   string
	EventID     string
	VehicleID   string
	OperatorID  string
	Timestamp   time.Time
	EntryType   string // "control", "safety", "state"
	CommandType string // for control entries
	Value       float32
	SystemState string // for state entries
	CtrlState   string
	EventType   string // for safety entries
	Reason      string
}

// ControlEventParams bundles RecordControlEvent's inputs (Rule 2.3 — more than 4 params).
type ControlEventParams struct {
	SessionID   string
	EventID     string
	VehicleID   string
	OperatorID  string
	CommandType string
	Value       float32
}

// StateSnapshotParams bundles RecordStateSnapshot's inputs (Rule 2.3 — more than 4 params).
type StateSnapshotParams struct {
	SessionID   string
	EventID     string
	VehicleID   string
	OperatorID  string
	SystemState string
	CtrlState   string
}

// SafetyEventParams bundles RecordSafetyEvent's inputs (Rule 2.3 — more than 4 params).
type SafetyEventParams struct {
	SessionID  string
	EventID    string
	VehicleID  string
	OperatorID string
	EventType  string
	Reason     string
}
