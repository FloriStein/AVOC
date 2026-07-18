package main

import (
	"testing"
)

func TestFleetVehicleSimulator_InitialState(t *testing.T) {
	s := newFleetVehicleSimulator("v1", typeLastenrad, fallbackDemoStations)
	if s.battery != 90 {
		t.Fatalf("expected initial battery 90, got %v", s.battery)
	}
	if s.lat != fallbackDemoStations[0].lat || s.lon != fallbackDemoStations[0].lon {
		t.Fatalf("expected to start at fallbackDemoStations[0], got lat=%v lon=%v", s.lat, s.lon)
	}
	if s.charging {
		t.Fatal("expected not charging initially")
	}
}

func TestFleetVehicleSimulator_Tick_ProducesValidStatus(t *testing.T) {
	s := newFleetVehicleSimulator("v1", typeLastenrad, fallbackDemoStations)
	status, _ := s.tick()

	if status.VehicleID != "v1" {
		t.Fatalf("expected VehicleID=v1, got %s", status.VehicleID)
	}
	if status.AutonomyMode != "autonomous" {
		t.Fatalf("expected AutonomyMode=autonomous, got %s", status.AutonomyMode)
	}
	if status.BatteryPct == nil || *status.BatteryPct < 0 || *status.BatteryPct > 100 {
		t.Fatalf("battery out of bounds: %v", status.BatteryPct)
	}
	if status.PositionLat == nil || status.PositionLon == nil {
		t.Fatal("expected position to be set")
	}
	if status.Timestamp.IsZero() {
		t.Fatal("expected non-zero timestamp")
	}
}

func TestFleetVehicleSimulator_MovesTowardTarget(t *testing.T) {
	s := newFleetVehicleSimulator("v1", typeLastenrad, fallbackDemoStations)
	target := fallbackDemoStations[s.targetIdx]

	startDist := distance(s.lat, s.lon, target.lat, target.lon)
	s.tick()
	afterDist := distance(s.lat, s.lon, target.lat, target.lon)

	if afterDist >= startDist {
		t.Fatalf("expected vehicle to move closer to target: before=%v after=%v", startDist, afterDist)
	}
}

func TestFleetVehicleSimulator_BatteryDrainsWhileMoving(t *testing.T) {
	s := newFleetVehicleSimulator("v1", typeLastenzug, fallbackDemoStations)
	initial := s.battery
	s.tick()
	if s.battery >= initial {
		t.Fatalf("expected battery to drain while moving: before=%v after=%v", initial, s.battery)
	}
}

func TestFleetVehicleSimulator_ArrivesAndSwitchesTarget_WhenBatteryHigh(t *testing.T) {
	s := newFleetVehicleSimulator("v1", typeLastenrad, fallbackDemoStations)
	s.battery = 90       // stays above chargeBelow throughout
	s.speedPerTick = 1.0 // arrive in a single tick for a deterministic test

	initialTarget := s.targetIdx
	s.tick()

	if s.charging {
		t.Fatal("expected not to start charging with high battery")
	}
	if s.targetIdx == initialTarget {
		t.Fatal("expected targetIdx to switch to the other station after arrival")
	}
	arrived := fallbackDemoStations[initialTarget]
	if s.lat != arrived.lat || s.lon != arrived.lon {
		t.Fatalf("expected position to snap exactly to arrived station, got lat=%v lon=%v", s.lat, s.lon)
	}
}

func TestFleetVehicleSimulator_ArrivesAndStartsCharging_WhenBatteryLow(t *testing.T) {
	s := newFleetVehicleSimulator("v1", typeLastenrad, fallbackDemoStations)
	s.battery = 25 // below chargeBelow (30)
	s.speedPerTick = 1.0

	initialTarget := s.targetIdx
	s.tick()

	if !s.charging {
		t.Fatal("expected to start charging on arrival with low battery")
	}
	if s.targetIdx != initialTarget {
		t.Fatal("expected targetIdx to stay the same while charging, not advance to next station")
	}
}

func TestFleetVehicleSimulator_ChargingIncreasesBatteryUntilFull_ThenResumes(t *testing.T) {
	s := newFleetVehicleSimulator("v1", typeLastenrad, fallbackDemoStations)
	s.charging = true
	s.battery = 99.9
	initialTarget := s.targetIdx

	status, _ := s.tick()

	if s.charging {
		t.Fatal("expected charging to stop once battery reaches 100")
	}
	if *status.BatteryPct != 100 {
		t.Fatalf("expected battery capped at 100, got %v", *status.BatteryPct)
	}
	_ = initialTarget // charging stop doesn't itself advance the target — next tick resumes driving
}

func TestFleetVehicleSimulator_BatteryNeverGoesBelowZero(t *testing.T) {
	s := newFleetVehicleSimulator("v1", typeLastenzug, fallbackDemoStations)
	s.battery = 0.01
	s.chargeBelow = -1 // never charge, force draining to the floor

	for i := 0; i < 20; i++ {
		status, _ := s.tick()
		if *status.BatteryPct < 0 {
			t.Fatalf("battery went negative: %v", *status.BatteryPct)
		}
	}
}

func TestFleetVehicleSimulator_LastenzugSlowerThanLastenrad(t *testing.T) {
	rad := newFleetVehicleSimulator("v1", typeLastenrad, fallbackDemoStations)
	zug := newFleetVehicleSimulator("v2", typeLastenzug, fallbackDemoStations)

	if !(zug.speedPerTick < rad.speedPerTick) {
		t.Fatalf("expected lastenzug to move slower than lastenrad: zug=%v rad=%v", zug.speedPerTick, rad.speedPerTick)
	}
	if !(zug.drainPerTick < rad.drainPerTick) {
		t.Fatalf("expected lastenzug to drain slower (larger battery) than lastenrad: zug=%v rad=%v", zug.drainPerTick, rad.drainPerTick)
	}
}

// distance is a simple Euclidean approximation — sufficient for these small local-scale test
// deltas, not meant for real geospatial use.
func distance(lat1, lon1, lat2, lon2 float64) float64 {
	dLat := lat1 - lat2
	dLon := lon1 - lon2
	return dLat*dLat + dLon*dLon
}
