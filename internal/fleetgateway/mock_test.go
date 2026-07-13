package fleetgateway

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Compile-time check: MockGateway must satisfy the FleetGateway interface.
var _ FleetGateway = (*MockGateway)(nil)

func TestMockGateway_SubscribeVehicleStatus_ReceivesEvent(t *testing.T) {
	g := NewMockGateway()

	var received VehicleStatusEvent
	var got bool
	g.SubscribeVehicleStatus(func(e VehicleStatusEvent) {
		received = e
		got = true
	})

	battery := 55.0
	g.SimulateVehicleStatus(VehicleStatusEvent{VehicleID: "v1", BatteryPct: &battery, AutonomyMode: "autonomous"})

	if !got {
		t.Fatal("subscriber was not called")
	}
	if received.VehicleID != "v1" || received.BatteryPct == nil || *received.BatteryPct != 55.0 {
		t.Fatalf("unexpected event: %+v", received)
	}
}

func TestMockGateway_SubscribeVehicleAlerts_ReceivesEvent(t *testing.T) {
	g := NewMockGateway()

	var received VehicleAlertEvent
	g.SubscribeVehicleAlerts(func(e VehicleAlertEvent) { received = e })

	g.SimulateVehicleAlert(VehicleAlertEvent{VehicleID: "v1", Severity: "critical", Message: "Hindernis erkannt"})

	if received.VehicleID != "v1" || received.Severity != "critical" || received.Message != "Hindernis erkannt" {
		t.Fatalf("unexpected event: %+v", received)
	}
}

func TestMockGateway_MultipleSubscribers_AllReceiveEvent(t *testing.T) {
	g := NewMockGateway()

	var count int32
	for i := 0; i < 3; i++ {
		g.SubscribeVehicleStatus(func(VehicleStatusEvent) { atomic.AddInt32(&count, 1) })
	}

	g.SimulateVehicleStatus(VehicleStatusEvent{VehicleID: "v1", AutonomyMode: "autonomous"})

	if count != 3 {
		t.Fatalf("expected all 3 subscribers to receive the event, got %d calls", count)
	}
}

func TestMockGateway_NoSubscribers_DoesNotPanic(t *testing.T) {
	g := NewMockGateway()
	// No subscribers registered at all — must not panic.
	g.SimulateVehicleStatus(VehicleStatusEvent{VehicleID: "v1"})
	g.SimulateVehicleAlert(VehicleAlertEvent{VehicleID: "v1"})
}

func TestMockGateway_SimulateVehicleStatus_FillsZeroTimestamp(t *testing.T) {
	g := NewMockGateway()

	var received VehicleStatusEvent
	g.SubscribeVehicleStatus(func(e VehicleStatusEvent) { received = e })

	before := time.Now()
	g.SimulateVehicleStatus(VehicleStatusEvent{VehicleID: "v1"}) // Timestamp left zero-value
	after := time.Now()

	if received.Timestamp.Before(before) || received.Timestamp.After(after) {
		t.Fatalf("expected auto-filled timestamp between %v and %v, got %v", before, after, received.Timestamp)
	}
}

func TestMockGateway_SimulateVehicleStatus_PreservesExplicitTimestamp(t *testing.T) {
	g := NewMockGateway()
	explicit := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	var received VehicleStatusEvent
	g.SubscribeVehicleStatus(func(e VehicleStatusEvent) { received = e })
	g.SimulateVehicleStatus(VehicleStatusEvent{VehicleID: "v1", Timestamp: explicit})

	if !received.Timestamp.Equal(explicit) {
		t.Fatalf("expected explicit timestamp %v preserved, got %v", explicit, received.Timestamp)
	}
}

func TestMockGateway_DispatchTask_ReturnsNilError(t *testing.T) {
	g := NewMockGateway()
	err := g.DispatchTask(TaskAssignment{TaskID: "t1", VehicleID: "v1", FromStationID: "a", ToStationID: "b"})
	if err != nil {
		t.Fatalf("expected nil error from mock DispatchTask, got %v", err)
	}
}

// TestMockGateway_ConcurrentSubscribeAndSimulate_NoRace exercises Subscribe* and Simulate*
// concurrently — run with `go test -race` to catch data races on the subscriber slices.
func TestMockGateway_ConcurrentSubscribeAndSimulate_NoRace(t *testing.T) {
	g := NewMockGateway()
	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			g.SubscribeVehicleStatus(func(VehicleStatusEvent) {})
		}()
		go func() {
			defer wg.Done()
			g.SimulateVehicleStatus(VehicleStatusEvent{VehicleID: "v1"})
		}()
	}
	wg.Wait()
}

func TestMockGateway_StartSimulation_ProducesEventsForEachVehicle(t *testing.T) {
	g := NewMockGateway()

	seen := map[string]int{}
	var mu sync.Mutex
	g.SubscribeVehicleStatus(func(e VehicleStatusEvent) {
		mu.Lock()
		seen[e.VehicleID]++
		mu.Unlock()
	})

	g.StartSimulation([]string{"sim-v1", "sim-v2"}, 52.13, 11.64, 10*time.Millisecond)
	defer g.Stop()

	deadline := time.After(2 * time.Second)
	for {
		mu.Lock()
		count1, count2 := seen["sim-v1"], seen["sim-v2"]
		mu.Unlock()
		if count1 >= 2 && count2 >= 2 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for simulated events, got seen=%v", seen)
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func TestMockGateway_StartSimulation_BatteryDrainsOverTime(t *testing.T) {
	g := NewMockGateway()

	var mu sync.Mutex
	var readings []float64
	g.SubscribeVehicleStatus(func(e VehicleStatusEvent) {
		if e.BatteryPct == nil {
			return
		}
		mu.Lock()
		readings = append(readings, *e.BatteryPct)
		mu.Unlock()
	})

	g.StartSimulation([]string{"drain-v1"}, 0, 0, 5*time.Millisecond)
	defer g.Stop()

	deadline := time.After(2 * time.Second)
	for {
		mu.Lock()
		n := len(readings)
		mu.Unlock()
		if n >= 10 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for battery readings, got %d", n)
		case <-time.After(10 * time.Millisecond):
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if readings[0] < readings[len(readings)-1] {
		t.Fatalf("expected battery to drain (non-increasing trend), first=%.2f last=%.2f", readings[0], readings[len(readings)-1])
	}
	for _, r := range readings {
		if r < 0 || r > 100 {
			t.Fatalf("battery reading out of bounds: %.2f", r)
		}
	}
}

func TestMockGateway_Stop_StopsProducingEvents(t *testing.T) {
	g := NewMockGateway()

	var count int32
	g.SubscribeVehicleStatus(func(VehicleStatusEvent) { atomic.AddInt32(&count, 1) })

	g.StartSimulation([]string{"stop-v1"}, 0, 0, 5*time.Millisecond)
	time.Sleep(50 * time.Millisecond)
	g.Stop()

	countAtStop := atomic.LoadInt32(&count)
	time.Sleep(100 * time.Millisecond) // would accumulate more events if the loop were still running
	countAfterWait := atomic.LoadInt32(&count)

	if countAfterWait != countAtStop {
		t.Fatalf("expected no new events after Stop(), got %d before wait and %d after", countAtStop, countAfterWait)
	}
}

func TestMockGateway_Stop_WithoutStart_DoesNotPanic(t *testing.T) {
	g := NewMockGateway()
	g.Stop() // no simulation ever started — must not panic on nil channel
}
