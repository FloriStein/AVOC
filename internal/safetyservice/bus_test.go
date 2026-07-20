package safetyservice_test

import (
	"sync"
	"testing"
	"time"

	"avoc/internal/safetyservice"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBus_PublishSafetyEvent_UpdatesState(t *testing.T) {
	b := safetyservice.NewBus()
	before := time.Now()

	b.PublishSafetyEvent(safetyservice.SafetyEvent{
		SessionID: "session-1",
		VehicleID: "vehicle-1",
		Type:      safetyservice.EventDeadmanTimeout,
		Reason:    "no ack",
	})

	state := b.GetSafetyState()
	assert.True(t, state.SafeMode)
	assert.Equal(t, safetyservice.EventDeadmanTimeout, state.LastEvent)
	assert.False(t, state.UpdatedAt.Before(before))
}

func TestBus_TriggerEmergencyStop_SetsSafeModeWithEmergencyStopType(t *testing.T) {
	b := safetyservice.NewBus()

	b.TriggerEmergencyStop("session-1", "vehicle-1", "operator triggered")

	state := b.GetSafetyState()
	assert.True(t, state.SafeMode)
	assert.Equal(t, safetyservice.EventEmergencyStop, state.LastEvent)
}

func TestBus_GetSafetyState_ZeroValueBeforeAnyEvent(t *testing.T) {
	b := safetyservice.NewBus()

	state := b.GetSafetyState()
	assert.False(t, state.SafeMode)
	assert.Empty(t, state.LastEvent)
	assert.True(t, state.UpdatedAt.IsZero())
}

func TestBus_Subscribe_NotifiesMultipleHandlers(t *testing.T) {
	b := safetyservice.NewBus()

	var wg sync.WaitGroup
	wg.Add(2)

	received := make(chan safetyservice.SafetyEvent, 2)
	handler := func(event safetyservice.SafetyEvent) {
		received <- event
		wg.Done()
	}
	b.Subscribe(handler)
	b.Subscribe(handler)

	event := safetyservice.SafetyEvent{SessionID: "session-1", Type: safetyservice.EventACKTimeout, Reason: "ack timeout"}
	b.PublishSafetyEvent(event)

	waitOrTimeout(t, &wg, time.Second)
	close(received)

	count := 0
	for got := range received {
		assert.Equal(t, event.SessionID, got.SessionID)
		assert.Equal(t, event.Type, got.Type)
		count++
	}
	assert.Equal(t, 2, count)
}

func TestBus_Reset_ClearsStateToZeroValue(t *testing.T) {
	b := safetyservice.NewBus()
	b.TriggerEmergencyStop("session-1", "vehicle-1", "reason")
	require.True(t, b.GetSafetyState().SafeMode)

	b.Reset()

	state := b.GetSafetyState()
	assert.False(t, state.SafeMode)
	assert.Empty(t, state.LastEvent)
	assert.True(t, state.UpdatedAt.IsZero())
}

func TestBus_ConcurrentPublishAndRead(t *testing.T) {
	b := safetyservice.NewBus()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			b.PublishSafetyEvent(safetyservice.SafetyEvent{
				SessionID: "session-1",
				Type:      safetyservice.EventWSDisconnect,
			})
		}()
		go func() {
			defer wg.Done()
			_ = b.GetSafetyState()
		}()
	}
	wg.Wait()

	state := b.GetSafetyState()
	assert.True(t, state.SafeMode)
}

func waitOrTimeout(t *testing.T, wg *sync.WaitGroup, timeout time.Duration) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
		t.Fatal("timed out waiting for handlers to be notified")
	}
}
