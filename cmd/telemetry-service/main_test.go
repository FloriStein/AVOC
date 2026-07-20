package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"avoc/internal/telemetryservice"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTelemetryMux_GetLatest_MissingVehicleID(t *testing.T) {
	client := telemetryservice.NewClient("test-broker:1883", "", "", "")
	mux := newTelemetryMux(client)

	req := httptest.NewRequest(http.MethodGet, "/telemetry/latest/", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestTelemetryMux_GetLatest_UnknownVehicle_NotFound(t *testing.T) {
	client := telemetryservice.NewClient("test-broker:1883", "", "", "")
	mux := newTelemetryMux(client)

	req := httptest.NewRequest(http.MethodGet, "/telemetry/latest/unknown-vehicle", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestTelemetryMux_Health(t *testing.T) {
	client := telemetryservice.NewClient("test-broker:1883", "", "", "")
	mux := newTelemetryMux(client)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]string
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, "ok", resp["status"])
	assert.Equal(t, "telemetry-service", resp["service"])
}
