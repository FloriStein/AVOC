package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// fleetStationsFetchTimeout bounds the one-shot startup call to fleet-service — this must not
// hang vehicle-mock's boot if fleet-service is slow/unreachable (e.g. still waiting on Postgres).
const fleetStationsFetchTimeout = 5 * time.Second

// fleetStationDTO mirrors the subset of internal/fleetservice.Station's JSON shape this package
// needs (FLEET-04) — a local copy instead of importing internal/fleetservice, consistent with
// vehicle-mock's existing pattern of not depending on other services' internal packages.
type fleetStationDTO struct {
	ID          string   `json:"id"`
	PositionLat *float64 `json:"position_lat"`
	PositionLon *float64 `json:"position_lon"`
}

// resolveSimulationStations returns the stations a simulated fleet vehicle should drive between:
// real data from fleet-service's GET /fleet/stations (FLEET-04) if at least 2 stations there have
// a geo-position set, otherwise fallbackDemoStations. Never returns fewer than 2 stations — a
// fetch/parse error or an insufficiently-seeded dev DB falls back rather than leaving the
// simulator with too few points to drive between.
func resolveSimulationStations(fleetServiceURL, jwtSecret string) []fleetStation {
	stations, err := fetchRealStations(fleetServiceURL, jwtSecret)
	if err != nil {
		log.Info("fleet simulation: using fallback demo stations", "reason", err)
		return fallbackDemoStations
	}
	if len(stations) < 2 {
		log.Info("fleet simulation: using fallback demo stations", "reason", "fewer than 2 geo-located stations in fleet-service", "found", len(stations))
		return fallbackDemoStations
	}
	log.Info("fleet simulation: using real fleet-service stations", "count", len(stations))
	return stations[:2]
}

// fetchRealStations calls fleet-service's GET /fleet/stations and returns only the stations with
// both position_lat and position_lon set (indoor stations, which only carry position_x/y, are not
// usable here — the simulator drives on lat/lon). Auth reuses the same shared JWT secret vehicles
// already sign telemetry-side tokens with (ADR-004) — fleet-service's RequireAuth only checks the
// signature, not the role claim, so a VEHICLE-role token is accepted like any other.
func fetchRealStations(fleetServiceURL, jwtSecret string) ([]fleetStation, error) {
	token, err := makeVehicleJWT("vehicle-mock-fleet-sim", jwtSecret)
	if err != nil {
		return nil, fmt.Errorf("mint token: %w", err)
	}

	req, err := http.NewRequest(http.MethodGet, fleetServiceURL+"/fleet/stations", nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: fleetStationsFetchTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request fleet-service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fleet-service returned %d", resp.StatusCode)
	}

	var dtos []fleetStationDTO
	if err := json.NewDecoder(resp.Body).Decode(&dtos); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	stations := make([]fleetStation, 0, len(dtos))
	for _, d := range dtos {
		if d.PositionLat == nil || d.PositionLon == nil {
			continue
		}
		stations = append(stations, fleetStation{id: d.ID, lat: *d.PositionLat, lon: *d.PositionLon})
	}
	return stations, nil
}
