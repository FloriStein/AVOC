package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"avoc/internal/safetyservice"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func postJSON(t *testing.T, mux *http.ServeMux, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	return rr
}

func TestSafetyMux_PostEvent_Accepted(t *testing.T) {
	bus := safetyservice.NewBus()
	mux := newSafetyMux(bus)

	rr := postJSON(t, mux, "/safety/event", safetyservice.SafetyEvent{
		SessionID: "session-1",
		VehicleID: "vehicle-1",
		Type:      safetyservice.EventDeadmanTimeout,
		Reason:    "no ack",
	})

	assert.Equal(t, http.StatusAccepted, rr.Code)
	state := bus.GetSafetyState()
	assert.True(t, state.SafeMode)
	assert.Equal(t, safetyservice.EventDeadmanTimeout, state.LastEvent)
}

func TestSafetyMux_PostEvent_MalformedJSON(t *testing.T) {
	bus := safetyservice.NewBus()
	mux := newSafetyMux(bus)

	req := httptest.NewRequest(http.MethodPost, "/safety/event", bytes.NewReader([]byte("{not json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestSafetyMux_PostEmergencyStop_Accepted(t *testing.T) {
	bus := safetyservice.NewBus()
	mux := newSafetyMux(bus)

	rr := postJSON(t, mux, "/safety/emergency-stop", map[string]string{
		"session_id": "session-1",
		"vehicle_id": "vehicle-1",
		"reason":     "operator triggered",
	})

	assert.Equal(t, http.StatusAccepted, rr.Code)
	state := bus.GetSafetyState()
	assert.True(t, state.SafeMode)
	assert.Equal(t, safetyservice.EventEmergencyStop, state.LastEvent)
}

func TestSafetyMux_PostEmergencyStop_MalformedJSON(t *testing.T) {
	bus := safetyservice.NewBus()
	mux := newSafetyMux(bus)

	req := httptest.NewRequest(http.MethodPost, "/safety/emergency-stop", bytes.NewReader([]byte("{not json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestSafetyMux_GetState_ReflectsBusState(t *testing.T) {
	bus := safetyservice.NewBus()
	bus.TriggerEmergencyStop("session-1", "vehicle-1", "reason")
	mux := newSafetyMux(bus)

	req := httptest.NewRequest(http.MethodGet, "/safety/state", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var state safetyservice.SafetyState
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&state))
	assert.True(t, state.SafeMode)
	assert.Equal(t, safetyservice.EventEmergencyStop, state.LastEvent)
}

func TestSafetyMux_Health(t *testing.T) {
	bus := safetyservice.NewBus()
	mux := newSafetyMux(bus)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]string
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, "ok", resp["status"])
	assert.Equal(t, "safety-service", resp["service"])
}
