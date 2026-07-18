package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResolveSimulationStations_UsesRealStations_WhenTwoOrMoreGeoLocated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lat1, lon1 := 1.0, 2.0
		lat2, lon2 := 3.0, 4.0
		json.NewEncoder(w).Encode([]fleetStationDTO{
			{ID: "real-a", PositionLat: &lat1, PositionLon: &lon1},
			{ID: "real-b", PositionLat: &lat2, PositionLon: &lon2},
		})
	}))
	defer srv.Close()

	stations := resolveSimulationStations(srv.URL, "test-secret")

	if len(stations) != 2 {
		t.Fatalf("expected 2 stations, got %d", len(stations))
	}
	if stations[0].id != "real-a" || stations[1].id != "real-b" {
		t.Fatalf("expected real fleet-service stations, got %+v", stations)
	}
}

func TestResolveSimulationStations_FallsBack_WhenFleetServiceUnreachable(t *testing.T) {
	// Deliberately not a real server — dial fails, exercising the error path.
	stations := resolveSimulationStations("http://127.0.0.1:1", "test-secret")

	if len(stations) < 2 {
		t.Fatalf("expected fallback stations (>=2), got %d", len(stations))
	}
	if stations[0].id != fallbackDemoStations[0].id {
		t.Fatalf("expected fallbackDemoStations, got %+v", stations)
	}
}

func TestResolveSimulationStations_FallsBack_WhenFewerThanTwoGeoLocatedStations(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lat, lon := 1.0, 2.0
		// One geo-located station, one indoor station without lat/lon — only 1 usable.
		json.NewEncoder(w).Encode([]fleetStationDTO{
			{ID: "real-a", PositionLat: &lat, PositionLon: &lon},
			{ID: "indoor-only", PositionLat: nil, PositionLon: nil},
		})
	}))
	defer srv.Close()

	stations := resolveSimulationStations(srv.URL, "test-secret")

	if stations[0].id != fallbackDemoStations[0].id {
		t.Fatalf("expected fallbackDemoStations when <2 usable stations exist, got %+v", stations)
	}
}

func TestResolveSimulationStations_FallsBack_WhenFleetServiceReturnsEmptyList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// GET /fleet/zones|stations returns JSON null (not []) when the table is empty
		// (ADR-029 known nil-slice behavior) — verify that decodes cleanly, not as an error.
		w.Write([]byte("null"))
	}))
	defer srv.Close()

	stations := resolveSimulationStations(srv.URL, "test-secret")

	if stations[0].id != fallbackDemoStations[0].id {
		t.Fatalf("expected fallbackDemoStations for an empty station list, got %+v", stations)
	}
}

func TestResolveSimulationStations_FallsBack_WhenFleetServiceReturnsNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	stations := resolveSimulationStations(srv.URL, "test-secret")

	if stations[0].id != fallbackDemoStations[0].id {
		t.Fatalf("expected fallbackDemoStations on non-200 response, got %+v", stations)
	}
}

func TestResolveSimulationStations_FallsBack_OnMalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("{not valid json"))
	}))
	defer srv.Close()

	stations := resolveSimulationStations(srv.URL, "test-secret")

	if stations[0].id != fallbackDemoStations[0].id {
		t.Fatalf("expected fallbackDemoStations on malformed JSON, got %+v", stations)
	}
}
