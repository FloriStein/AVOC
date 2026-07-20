package recording_test

import (
	"testing"

	"avoc/internal/recording"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryRecorder_StartSession_InitializesEmptyEntries(t *testing.T) {
	r := recording.NewMemoryRecorder()

	r.StartSession("session-1", "vehicle-1", "operator-1")

	assert.Empty(t, r.GetEntries("session-1"))
}

func TestMemoryRecorder_EndSession_DoesNotPanicWithoutStart(t *testing.T) {
	r := recording.NewMemoryRecorder()

	assert.NotPanics(t, func() {
		r.EndSession("unknown-session")
	})
}

func TestMemoryRecorder_RecordControlEvent_SetsEntryFields(t *testing.T) {
	r := recording.NewMemoryRecorder()
	r.StartSession("session-1", "vehicle-1", "operator-1")

	r.RecordControlEvent(recording.ControlEventParams{
		SessionID:   "session-1",
		EventID:     "event-1",
		VehicleID:   "vehicle-1",
		OperatorID:  "operator-1",
		CommandType: "steer",
		Value:       0.5,
	})

	entries := r.GetEntries("session-1")
	require.Len(t, entries, 1)
	e := entries[0]
	assert.Equal(t, "session-1", e.SessionID)
	assert.Equal(t, "event-1", e.EventID)
	assert.Equal(t, "vehicle-1", e.VehicleID)
	assert.Equal(t, "operator-1", e.OperatorID)
	assert.Equal(t, "control", e.EntryType)
	assert.Equal(t, "steer", e.CommandType)
	assert.Equal(t, float32(0.5), e.Value)
	assert.False(t, e.Timestamp.IsZero())
}

func TestMemoryRecorder_RecordStateSnapshot_SetsEntryFields(t *testing.T) {
	r := recording.NewMemoryRecorder()
	r.StartSession("session-1", "vehicle-1", "operator-1")

	r.RecordStateSnapshot(recording.StateSnapshotParams{
		SessionID:   "session-1",
		EventID:     "event-2",
		VehicleID:   "vehicle-1",
		OperatorID:  "operator-1",
		SystemState: "ACTIVE",
		CtrlState:   "MANUAL",
	})

	entries := r.GetEntries("session-1")
	require.Len(t, entries, 1)
	e := entries[0]
	assert.Equal(t, "session-1", e.SessionID)
	assert.Equal(t, "event-2", e.EventID)
	assert.Equal(t, "state", e.EntryType)
	assert.Equal(t, "ACTIVE", e.SystemState)
	assert.Equal(t, "MANUAL", e.CtrlState)
	assert.False(t, e.Timestamp.IsZero())
}

func TestMemoryRecorder_RecordSafetyEvent_SetsEntryFields(t *testing.T) {
	r := recording.NewMemoryRecorder()
	r.StartSession("session-1", "vehicle-1", "operator-1")

	r.RecordSafetyEvent(recording.SafetyEventParams{
		SessionID:  "session-1",
		EventID:    "event-3",
		VehicleID:  "vehicle-1",
		OperatorID: "operator-1",
		EventType:  "EMERGENCY_STOP",
		Reason:     "operator triggered",
	})

	entries := r.GetEntries("session-1")
	require.Len(t, entries, 1)
	e := entries[0]
	assert.Equal(t, "session-1", e.SessionID)
	assert.Equal(t, "event-3", e.EventID)
	assert.Equal(t, "safety", e.EntryType)
	assert.Equal(t, "EMERGENCY_STOP", e.EventType)
	assert.Equal(t, "operator triggered", e.Reason)
	assert.False(t, e.Timestamp.IsZero())
}

func TestMemoryRecorder_GetEntries_PreservesOrder(t *testing.T) {
	r := recording.NewMemoryRecorder()
	r.StartSession("session-1", "vehicle-1", "operator-1")

	r.RecordControlEvent(recording.ControlEventParams{SessionID: "session-1", EventID: "first", CommandType: "steer"})
	r.RecordStateSnapshot(recording.StateSnapshotParams{SessionID: "session-1", EventID: "second", SystemState: "ACTIVE"})
	r.RecordSafetyEvent(recording.SafetyEventParams{SessionID: "session-1", EventID: "third", EventType: "EMERGENCY_STOP"})

	entries := r.GetEntries("session-1")
	require.Len(t, entries, 3)
	assert.Equal(t, "first", entries[0].EventID)
	assert.Equal(t, "second", entries[1].EventID)
	assert.Equal(t, "third", entries[2].EventID)
}

func TestMemoryRecorder_GetEntries_ReturnsCopyNotReference(t *testing.T) {
	r := recording.NewMemoryRecorder()
	r.StartSession("session-1", "vehicle-1", "operator-1")
	r.RecordControlEvent(recording.ControlEventParams{SessionID: "session-1", EventID: "first", CommandType: "steer"})

	entries := r.GetEntries("session-1")
	require.Len(t, entries, 1)
	entries[0].EventID = "mutated"
	entries = append(entries, recording.Entry{EventID: "appended"})

	fresh := r.GetEntries("session-1")
	require.Len(t, fresh, 1)
	assert.Equal(t, "first", fresh[0].EventID)
}

func TestMemoryRecorder_SessionIsolation(t *testing.T) {
	r := recording.NewMemoryRecorder()
	r.StartSession("session-a", "vehicle-1", "operator-1")
	r.StartSession("session-b", "vehicle-2", "operator-2")

	r.RecordControlEvent(recording.ControlEventParams{SessionID: "session-a", EventID: "a-event", CommandType: "steer"})
	r.RecordControlEvent(recording.ControlEventParams{SessionID: "session-b", EventID: "b-event-1", CommandType: "steer"})
	r.RecordControlEvent(recording.ControlEventParams{SessionID: "session-b", EventID: "b-event-2", CommandType: "brake"})

	entriesA := r.GetEntries("session-a")
	entriesB := r.GetEntries("session-b")
	require.Len(t, entriesA, 1)
	require.Len(t, entriesB, 2)
	assert.Equal(t, "a-event", entriesA[0].EventID)
}

func TestMemoryRecorder_GetEntries_UnknownSessionID(t *testing.T) {
	r := recording.NewMemoryRecorder()

	var entries []recording.Entry
	assert.NotPanics(t, func() {
		entries = r.GetEntries("does-not-exist")
	})
	assert.Empty(t, entries)
}
