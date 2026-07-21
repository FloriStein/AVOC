package fleetgateway

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// MockGateway is an in-memory FleetGateway (ADR-027) — pub/sub for status/alert events, plus a
// built-in simulation loop so it produces plausible vehicle data on its own (no dependency on
// FLEET-04's vehicle-mock extension to be useful/testable in isolation). SimulateVehicleStatus/
// SimulateVehicleAlert are also exported so FLEET-04/05 can drive it from outside instead.
type MockGateway struct {
	mu         sync.RWMutex
	statusSubs []func(VehicleStatusEvent)
	alertSubs  []func(VehicleAlertEvent)

	simMu    sync.Mutex
	simState map[string]*simulatedVehicle
	stopSim  chan struct{}
	simDone  chan struct{} // closed once the simulation goroutine has actually returned
}

type simulatedVehicle struct {
	batteryPct   float64
	lat, lon     float64
	autonomyMode string
}

func NewMockGateway() *MockGateway {
	return &MockGateway{simState: make(map[string]*simulatedVehicle)}
}

func (g *MockGateway) SubscribeVehicleStatus(fn func(VehicleStatusEvent)) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.statusSubs = append(g.statusSubs, fn)
}

func (g *MockGateway) SubscribeVehicleAlerts(fn func(VehicleAlertEvent)) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.alertSubs = append(g.alertSubs, fn)
}

// DispatchTask is a no-op in the mock (logged via the returned nil — no real vehicle to
// dispatch to). Kept side-effect-free rather than panicking so callers can exercise the full
// dispatch path in tests without a real backend.
func (g *MockGateway) DispatchTask(_ TaskAssignment) error {
	return nil
}

// SimulateVehicleStatus injects a status event as if it came from the external backend — lets
// FLEET-04's vehicle-mock extension (or a test) drive this gateway directly instead of relying
// on the built-in simulation loop.
func (g *MockGateway) SimulateVehicleStatus(e VehicleStatusEvent) {
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	for _, fn := range g.statusSubs {
		fn(e)
	}
}

// SimulateVehicleAlert injects a vehicle-initiated alert (ADR-028 Notfall-Trigger-Modell).
func (g *MockGateway) SimulateVehicleAlert(e VehicleAlertEvent) {
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	for _, fn := range g.alertSubs {
		fn(e)
	}
}

// StartSimulation begins a built-in simulation loop for the given vehicle IDs — random-walk
// position around (startLat, startLon), slowly draining battery, occasionally raising a
// vehicle-initiated alert. Satisfies "Mock liefert simulierte Fahrzeugdaten" (FLEET-03) without
// requiring FLEET-04's vehicle-mock extension. Call Stop to end it.
func (g *MockGateway) StartSimulation(vehicleIDs []string, startLat, startLon float64, tick time.Duration) {
	g.simMu.Lock()
	defer g.simMu.Unlock()

	for _, id := range vehicleIDs {
		g.simState[id] = &simulatedVehicle{batteryPct: 90, lat: startLat, lon: startLon, autonomyMode: "autonomous"}
	}
	g.stopSim = make(chan struct{})
	stop := g.stopSim
	g.simDone = make(chan struct{})
	done := g.simDone

	go func() {
		defer close(done)
		ticker := time.NewTicker(tick)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				g.simulateTick()
			}
		}
	}()
}

// Stop ends the simulation loop started by StartSimulation and blocks until the loop's goroutine
// has actually returned — closing stopSim alone would let a tick already selected concurrently
// with Stop() still emit one more event after Stop() returns (a real, reproducible race: the
// ticker and the caller's shutdown timing are independent, and a plain "signal and return
// immediately" Stop() gives no guarantee the goroutine reacted before the caller moves on).
// Safe to call even if no simulation is running.
func (g *MockGateway) Stop() {
	g.simMu.Lock()
	stop := g.stopSim
	done := g.simDone
	g.stopSim = nil
	g.simDone = nil
	g.simMu.Unlock()

	if stop == nil {
		return
	}
	close(stop)
	<-done
}

func (g *MockGateway) simulateTick() {
	g.simMu.Lock()
	type vehicleSnapshot struct {
		id    string
		state simulatedVehicle
	}
	var snapshots []vehicleSnapshot
	for id, v := range g.simState {
		// Small random walk — plausible movement, not a real path-following simulator
		// (FLEET-04 adds station-to-station movement for actual task demos).
		v.lat += (rand.Float64() - 0.5) * 0.0005
		v.lon += (rand.Float64() - 0.5) * 0.0005
		v.batteryPct -= rand.Float64() * 0.2
		if v.batteryPct < 0 {
			v.batteryPct = 0
		}
		snapshots = append(snapshots, vehicleSnapshot{id: id, state: *v})
	}
	g.simMu.Unlock()

	for _, s := range snapshots {
		lat, lon, battery := s.state.lat, s.state.lon, s.state.batteryPct
		g.SimulateVehicleStatus(VehicleStatusEvent{
			VehicleID:    s.id,
			BatteryPct:   &battery,
			PositionLat:  &lat,
			PositionLon:  &lon,
			AutonomyMode: s.state.autonomyMode,
		})

		// Rare vehicle-initiated alert — models "detected a problem it cannot resolve itself"
		// (ADR-028), not battery (that's a fleet-service threshold computation, FLEET-07).
		if rand.Float64() < 0.01 {
			g.SimulateVehicleAlert(VehicleAlertEvent{
				VehicleID: s.id,
				Severity:  "critical",
				Message:   fmt.Sprintf("Hindernis auf der Strecke erkannt — Fahrzeug %s kann nicht selbstständig ausweichen", s.id),
			})
		}
	}
}
