package main

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"avoc/internal/fleetgateway"
)

// fleetSimulationTick is how often each simulated fleet vehicle publishes a status update.
// Independent of telemetryHz (Direct-Teleop) — the fleet vehicles aren't in a teleop session.
const fleetSimulationTick = 3 * time.Second

// fleetStation is placeholder demo geo data for FLEET-04 — vehicle-mock has no DB access, so
// station coordinates are hardcoded here. Once fleet-service's REST API exists (FLEET-05) this
// could fetch real Zone/Station data instead; documented as a known simplification.
type fleetStation struct {
	id       string
	lat, lon float64
}

var demoStations = []fleetStation{
	{id: "demo-station-a", lat: 52.130100, lon: 11.640100},
	{id: "demo-station-b", lat: 52.130500, lon: 11.641200},
}

// fleetVehicleType tunes simulation parameters per vehicle type (ADR-029).
type fleetVehicleType string

const (
	typeLastenrad fleetVehicleType = "lastenrad"
	typeLastenzug fleetVehicleType = "lastenzug"
)

// arrivalThresholdDegrees is how close (in decimal degrees) counts as "arrived" — well under a
// meter at this latitude, small enough to never falsely trigger given the movement step sizes
// below.
const arrivalThresholdDegrees = 0.00005

// fleetVehicleSimulator drives one simulated autonomous fleet vehicle moving back and forth
// between two demo stations, draining battery while moving and charging while parked at a
// station once low (FLEET-04). Pure logic, no I/O — testable in isolation; main.go wires tick()
// output to MQTT publish (StatusTopic/AlertTopic).
type fleetVehicleSimulator struct {
	vehicleID string
	vType     fleetVehicleType

	battery   float64
	lat, lon  float64
	targetIdx int
	charging  bool

	speedPerTick  float64 // fraction of remaining distance to target covered per tick
	drainPerTick  float64
	chargePerTick float64
	chargeBelow   float64 // start charging when battery drops below this, once arrived
}

func newFleetVehicleSimulator(vehicleID string, vType fleetVehicleType) *fleetVehicleSimulator {
	// Lastenrad: nimbler/faster, smaller battery (drains and charges faster in relative terms).
	// Lastenzug: slower, larger battery (drains and charges more slowly in relative terms).
	speed, drain, charge := 0.08, 0.15, 0.6
	if vType == typeLastenzug {
		speed, drain, charge = 0.03, 0.08, 0.3
	}
	start := demoStations[0]
	return &fleetVehicleSimulator{
		vehicleID: vehicleID, vType: vType,
		battery: 90, lat: start.lat, lon: start.lon, targetIdx: 1,
		speedPerTick: speed, drainPerTick: drain, chargePerTick: charge, chargeBelow: 30,
	}
}

// tick advances the simulation by one step and returns the status to publish, plus an alert if
// one was triggered this tick (nil otherwise — alerts are rare, per ADR-028 they represent a
// problem the vehicle cannot resolve itself, not routine telemetry).
func (s *fleetVehicleSimulator) tick() (fleetgateway.VehicleStatusEvent, *fleetgateway.VehicleAlertEvent) {
	target := demoStations[s.targetIdx]

	if s.charging {
		s.battery += s.chargePerTick
		if s.battery >= 100 {
			s.battery = 100
			s.charging = false
		}
	} else {
		s.lat += (target.lat - s.lat) * s.speedPerTick
		s.lon += (target.lon - s.lon) * s.speedPerTick
		s.battery -= s.drainPerTick
		if s.battery < 0 {
			s.battery = 0
		}

		if math.Abs(target.lat-s.lat) < arrivalThresholdDegrees && math.Abs(target.lon-s.lon) < arrivalThresholdDegrees {
			s.lat, s.lon = target.lat, target.lon
			if s.battery < s.chargeBelow {
				s.charging = true
			} else {
				s.targetIdx = (s.targetIdx + 1) % len(demoStations)
			}
		}
	}

	battery, lat, lon := s.battery, s.lat, s.lon
	status := fleetgateway.VehicleStatusEvent{
		VehicleID:    s.vehicleID,
		BatteryPct:   &battery,
		PositionLat:  &lat,
		PositionLon:  &lon,
		AutonomyMode: "autonomous",
		Timestamp:    time.Now(),
	}

	var alert *fleetgateway.VehicleAlertEvent
	if !s.charging && rand.Float64() < 0.005 {
		alert = &fleetgateway.VehicleAlertEvent{
			VehicleID: s.vehicleID,
			Severity:  "critical",
			Message:   fmt.Sprintf("Hindernis auf der Strecke erkannt — %s kann nicht selbstständig ausweichen", s.vehicleID),
			Timestamp: time.Now(),
		}
	}
	return status, alert
}

// startFleetSimulation parses spec (format: "id:type,id:type,..." — e.g.
// "lastenzug-01:lastenzug,lastenrad-01:lastenrad") and starts one goroutine per vehicle,
// publishing VehicleStatusEvent/VehicleAlertEvent as JSON to fleetgateway.StatusTopic/AlertTopic
// on the given MQTT client — the concrete transport realization of ADR-027 until the real
// ROS2/DDS interface is confirmed. No-op if spec is empty (default: no fleet vehicles simulated,
// keeps the existing single-vehicle Direct-Teleop behavior as the default docker-compose setup).
func startFleetSimulation(client mqtt.Client, spec string) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return
	}

	for _, entry := range strings.Split(spec, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		parts := strings.SplitN(entry, ":", 2)
		id := parts[0]
		vType := typeLastenzug
		if len(parts) == 2 && fleetVehicleType(parts[1]) == typeLastenrad {
			vType = typeLastenrad
		}

		sim := newFleetVehicleSimulator(id, vType)
		go runFleetVehicleSimulation(client, sim)
		log.Info("fleet vehicle simulation started", "vehicle_id", id, "type", vType)
	}
}

func runFleetVehicleSimulation(client mqtt.Client, sim *fleetVehicleSimulator) {
	ticker := time.NewTicker(fleetSimulationTick)
	defer ticker.Stop()

	for range ticker.C {
		status, alert := sim.tick()
		publishFleetJSON(client, fleetgateway.StatusTopic(sim.vehicleID), status)
		if alert != nil {
			publishFleetJSON(client, fleetgateway.AlertTopic(sim.vehicleID), *alert)
		}
	}
}

func publishFleetJSON(client mqtt.Client, topic string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		log.Warn("fleet payload marshal failed", "topic", topic, "error", err)
		return
	}
	token := client.Publish(topic, 1, false, data)
	token.Wait()
	if err := token.Error(); err != nil {
		log.Warn("fleet MQTT publish failed", "topic", topic, "error", err)
	}
}
