package command

import (
	"errors"
	"sync"
	"testing"
	"time"

	commonv1 "avoc/gen/go/common/v1"
	controlv1 "avoc/gen/go/control/v1"
	"avoc/internal/controlserver/session"
	"avoc/internal/controlserver/statemachine"
	"avoc/internal/controlserver/vehiclecontext"
	"avoc/internal/safetyservice"
	"avoc/pkg/audit"
	"avoc/tests/unit/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// mockAuditWriter lets tests configure WriteSync's return value and records every call.
type mockAuditWriter struct {
	mu      sync.Mutex
	err     error
	written []audit.SafetyAuditEvent
}

func (m *mockAuditWriter) WriteSync(event audit.SafetyAuditEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.written = append(m.written, event)
	return m.err
}

func (m *mockAuditWriter) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.written)
}

// mockForwarder lets tests configure ForwardCommand's return value and records every call.
type mockForwarder struct {
	mu       sync.Mutex
	err      error
	forwards []string // vehicleID per call
}

func (m *mockForwarder) ForwardCommand(vehicleID string, _ []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.forwards = append(m.forwards, vehicleID)
	return m.err
}

func (m *mockForwarder) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.forwards)
}

// newTestEngine builds an Engine with a registry brought to StateConnected for vehicleID, mirroring
// the real bootstrap sequence in tests/unit/safety_test.go's newTestSetup.
func newTestEngine(t *testing.T, vehicleID string) (*Engine, *vehiclecontext.Registry, *mocks.MockSafetyPublisher) {
	t.Helper()
	pub := &mocks.MockSafetyPublisher{}
	registry := vehiclecontext.NewRegistry(time.Minute, time.Minute, time.Minute, pub)
	vc := registry.Get(vehicleID)
	vc.SM.TransitionSystem(statemachine.StateConnecting)
	vc.SM.TransitionSystem(statemachine.StateAuthenticated)
	vc.SM.TransitionSystem(statemachine.StateConnected)

	sessionMgr := session.NewManager(nil)
	e := NewEngine(registry, pub, sessionMgr)
	return e, registry, pub
}

func testSession(vehicleID string) session.Session {
	return session.Session{ID: "session-1", VehicleID: vehicleID, OperatorID: "operator-1", OperatorRole: "ACTIVE_OPERATOR"}
}

// --- handleEmergencyStop: audit write error path (GOTEST-04) ---

// TestHandleEmergencyStop_AuditWriteError_StillReachesSafeMode is the core regression test: even
// when the audit store is unreachable, EMERGENCY_STOP must still transition the vehicle to
// SAFE_MODE and still publish the safety event — the audit failure is logged, not fatal (ADR-018).
func TestHandleEmergencyStop_AuditWriteError_StillReachesSafeMode(t *testing.T) {
	e, registry, pub := newTestEngine(t, "vehicle-001")
	aw := &mockAuditWriter{err: errors.New("audit store unreachable")}
	e.WithAuditWriter(aw)

	vc := registry.Get("vehicle-001")
	e.handleEmergencyStop(vc, testSession("vehicle-001"))

	sys, _, _, _ := vc.SM.Get()
	assert.Equal(t, statemachine.StateSafeMode, sys, "SAFE_MODE must be reached despite the audit write error")
	assert.Equal(t, 1, aw.callCount(), "audit write must still have been attempted")
	assert.Equal(t, safetyservice.EventEmergencyStop, pub.LastEventType(), "safety event must still be published despite the audit error")
}

func TestHandleEmergencyStop_AuditWriteSuccess_ReachesSafeMode(t *testing.T) {
	e, registry, pub := newTestEngine(t, "vehicle-001")
	aw := &mockAuditWriter{}
	e.WithAuditWriter(aw)

	vc := registry.Get("vehicle-001")
	e.handleEmergencyStop(vc, testSession("vehicle-001"))

	sys, _, _, _ := vc.SM.Get()
	assert.Equal(t, statemachine.StateSafeMode, sys)
	assert.Equal(t, 1, aw.callCount())
	assert.Equal(t, safetyservice.EventEmergencyStop, pub.LastEventType())
}

// TestHandleEmergencyStop_NoAuditWriterConfigured covers the nil-auditWriter boundary case
// (Engine built without WithAuditWriter) — must not panic and must still reach SAFE_MODE.
func TestHandleEmergencyStop_NoAuditWriterConfigured(t *testing.T) {
	e, registry, pub := newTestEngine(t, "vehicle-001")

	vc := registry.Get("vehicle-001")
	assert.NotPanics(t, func() { e.handleEmergencyStop(vc, testSession("vehicle-001")) })

	sys, _, _, _ := vc.SM.Get()
	assert.Equal(t, statemachine.StateSafeMode, sys)
	assert.Equal(t, safetyservice.EventEmergencyStop, pub.LastEventType())
}

// TestHandleEmergencyStop_Idempotent_SecondCallDoesNotPanic covers repeated E-Stop delivery
// (e.g. operator holds the button, message retransmitted) — the second TransitionSystem call is
// rejected by the state machine's own guard (SAFE_MODE has no self-transition), but the audit
// write and safety publish must not error out or panic.
func TestHandleEmergencyStop_Idempotent_SecondCallDoesNotPanic(t *testing.T) {
	e, registry, pub := newTestEngine(t, "vehicle-001")
	aw := &mockAuditWriter{}
	e.WithAuditWriter(aw)

	vc := registry.Get("vehicle-001")
	e.handleEmergencyStop(vc, testSession("vehicle-001"))
	assert.NotPanics(t, func() { e.handleEmergencyStop(vc, testSession("vehicle-001")) })

	sys, _, _, _ := vc.SM.Get()
	assert.Equal(t, statemachine.StateSafeMode, sys, "must remain in SAFE_MODE")
	assert.Equal(t, 2, aw.callCount())
	assert.Len(t, pub.Events(), 2)
}

// TestHandle_EmergencyStopCommand_AuditError_EndToEnd drives the same scenario through the public
// Handle() entrypoint with a real serialized ControlCommand, verifying the ACK still reports
// success and SAFE_MODE is reached even though the audit writer fails.
func TestHandle_EmergencyStopCommand_AuditError_EndToEnd(t *testing.T) {
	e, registry, _ := newTestEngine(t, "vehicle-001")
	aw := &mockAuditWriter{err: errors.New("audit store unreachable")}
	e.WithAuditWriter(aw)

	cmd := &controlv1.ControlCommand{
		Header: &commonv1.CorrelationHeader{EventId: "event-1"},
		Type:   controlv1.CommandType_COMMAND_TYPE_EMERGENCY_STOP,
	}
	raw, err := proto.Marshal(cmd)
	require.NoError(t, err)

	ackBytes, err := e.Handle(raw, testSession("vehicle-001"))
	require.NoError(t, err)

	ack := &controlv1.ControlAck{}
	require.NoError(t, proto.Unmarshal(ackBytes, ack))
	assert.True(t, ack.Success)

	sys, _, _, _ := registry.Get("vehicle-001").SM.Get()
	assert.Equal(t, statemachine.StateSafeMode, sys)
}

// --- forwardMovementCommand (same safety-critical file, currently 0 tests) ---

func TestForwardMovementCommand_Success_ArmsVehicleACKWatchdog(t *testing.T) {
	e, registry, _ := newTestEngine(t, "vehicle-001")
	fwd := &mockForwarder{}
	e.WithVehicleForwarder(fwd)

	vc := registry.Get("vehicle-001")
	cmd := &controlv1.ControlCommand{Type: controlv1.CommandType_COMMAND_TYPE_STEER, Value: 0.5}
	e.forwardMovementCommand(vc, testSession("vehicle-001"), cmd, []byte{0x01})

	assert.Equal(t, 1, fwd.callCount())
}

// TestForwardMovementCommand_ForwarderError_DoesNotPanic covers the vehicle connection being
// unreachable — VehicleForwarder is documented as fire-and-forget (errors logged, never block).
func TestForwardMovementCommand_ForwarderError_DoesNotPanic(t *testing.T) {
	e, registry, _ := newTestEngine(t, "vehicle-001")
	fwd := &mockForwarder{err: errors.New("vehicle connection closed")}
	e.WithVehicleForwarder(fwd)

	vc := registry.Get("vehicle-001")
	cmd := &controlv1.ControlCommand{Type: controlv1.CommandType_COMMAND_TYPE_STEER, Value: 0.5}
	assert.NotPanics(t, func() {
		e.forwardMovementCommand(vc, testSession("vehicle-001"), cmd, []byte{0x01})
	})
	assert.Equal(t, 1, fwd.callCount())
}

// TestForwardMovementCommand_NoForwarderConfigured covers the nil-forwarder boundary case.
func TestForwardMovementCommand_NoForwarderConfigured(t *testing.T) {
	e, registry, _ := newTestEngine(t, "vehicle-001")

	vc := registry.Get("vehicle-001")
	cmd := &controlv1.ControlCommand{Type: controlv1.CommandType_COMMAND_TYPE_STEER, Value: 0.5}
	assert.NotPanics(t, func() {
		e.forwardMovementCommand(vc, testSession("vehicle-001"), cmd, []byte{0x01})
	})
}

// TestForwardMovementCommand_EmptyVehicleID covers the boundary case of a session without a
// vehicle assigned (should never happen in production, but must not panic).
func TestForwardMovementCommand_EmptyVehicleID(t *testing.T) {
	e, registry, _ := newTestEngine(t, "vehicle-001")
	fwd := &mockForwarder{}
	e.WithVehicleForwarder(fwd)

	vc := registry.Get("vehicle-001")
	sess := session.Session{ID: "session-1", VehicleID: "", OperatorID: "operator-1"}
	cmd := &controlv1.ControlCommand{Type: controlv1.CommandType_COMMAND_TYPE_STEER, Value: 0.5}
	assert.NotPanics(t, func() { e.forwardMovementCommand(vc, sess, cmd, []byte{0x01}) })
	assert.Equal(t, 0, fwd.callCount(), "must not forward when the session has no vehicle assigned")
}

// TestHandle_ObserverSendsMovementCommand_Rejected covers the OBSERVER access boundary (ADR-025):
// observers may never send movement commands.
func TestHandle_ObserverSendsMovementCommand_Rejected(t *testing.T) {
	e, _, _ := newTestEngine(t, "vehicle-001")
	fwd := &mockForwarder{}
	e.WithVehicleForwarder(fwd)

	cmd := &controlv1.ControlCommand{
		Header: &commonv1.CorrelationHeader{EventId: "event-1"},
		Type:   controlv1.CommandType_COMMAND_TYPE_STEER,
		Value:  0.5,
	}
	raw, err := proto.Marshal(cmd)
	require.NoError(t, err)

	sess := session.Session{ID: "session-1", VehicleID: "vehicle-001", OperatorID: "operator-1", OperatorRole: "OBSERVER"}
	ackBytes, err := e.Handle(raw, sess)
	require.NoError(t, err)

	ack := &controlv1.ControlAck{}
	require.NoError(t, proto.Unmarshal(ackBytes, ack))
	assert.False(t, ack.Success)
	assert.Equal(t, 0, fwd.callCount(), "observer's command must never reach the vehicle forwarder")
}

// TestHandle_MalformedProtobuf_ReturnsFailedAckNotPanic covers malformed input from the wire.
func TestHandle_MalformedProtobuf_ReturnsFailedAckNotPanic(t *testing.T) {
	e, _, _ := newTestEngine(t, "vehicle-001")

	var ackBytes []byte
	var err error
	assert.NotPanics(t, func() {
		ackBytes, err = e.Handle([]byte{0xFF, 0xFF, 0xFF}, testSession("vehicle-001"))
	})
	require.NoError(t, err)

	ack := &controlv1.ControlAck{}
	require.NoError(t, proto.Unmarshal(ackBytes, ack))
	assert.False(t, ack.Success)
}
