package logger

// Event type constants for all structured log events (ADR-017).
// Use with Logger.Event() so Loki/Grafana can filter by event_type label.
const (
	// Session lifecycle
	EventSessionStarted = "SESSION_STARTED"
	EventSessionEnded   = "SESSION_ENDED"

	// Safety critical events — these also write to SQLite via AuditWriter (ADR-018)
	EventSafeModeEntered            = "SAFE_MODE_ENTERED"
	EventEmergencyStop              = "EMERGENCY_STOP"
	EventDeadmanTimeout             = "DEADMAN_TIMEOUT"
	EventDeadmanArmed               = "DEADMAN_ARMED"
	EventDeadmanStarted             = "DEADMAN_STARTED"
	EventDeadmanStopped             = "DEADMAN_STOPPED"
	EventAckTimeout                 = "COMMAND_ACK_TIMEOUT"
	EventVehicleACKTimeout          = "VEHICLE_ACK_TIMEOUT"
	EventSafetyBusDown              = "SAFETY_BUS_DOWN"
	EventWsDisconnect               = "WS_DISCONNECT_CRITICAL"
	EventWsConnected                = "WS_CONNECTED"
	EventWsSessionInvalid           = "WS_SESSION_NOT_FOUND"
	EventOperatorHandover           = "OPERATOR_HANDOVER_COMPLETED"
	EventAuthWatchdogTriggered      = "AUTH_WATCHDOG_TRIGGERED"      // DRIFT-K1 (2026-07-16)
	EventTelemetryWatchdogTriggered = "TELEMETRY_WATCHDOG_TRIGGERED" // DRIFT-K3-TELEMETRY (2026-07-21)

	// System events
	EventStateTransition      = "STATE_TRANSITION_SYSTEM"
	EventCommandReceived      = "COMMAND_RECEIVED"
	EventMediaStateChange     = "MEDIA_STATE_CHANGE"
	EventTelemetryStateChange = "TELEMETRY_STATE_CHANGE" // DRIFT-K3-TELEMETRY (2026-07-21)

	// Frontend events — received via POST /log (LOG-07)
	EventFEEmergencyStop = "FE_EMERGENCY_STOP_CLICKED"
	EventFEDeadmanHold   = "FE_DEADMAN_HOLD"
	EventFEWebRTCState   = "FE_WEBRTC_STATE_CHANGE"
	EventFEWSReconnect   = "FE_WS_RECONNECT"
	EventFEWSConnected   = "FE_WS_CONNECTED"
	EventFEOperatorAck   = "FE_OPERATOR_ACK_CLICKED"
)
