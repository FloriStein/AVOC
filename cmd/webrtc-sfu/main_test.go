package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"avoc/internal/webrtcsfu"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func doRequest(mux *http.ServeMux, method, path string, body []byte) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	return rr
}

func TestSFUMux_SessionEvent_Accepted(t *testing.T) {
	sfu := webrtcsfu.New()
	mux := newSFUMux(sfu)

	body, err := json.Marshal(webrtcsfu.SessionEvent{Type: webrtcsfu.EventCreated, SessionID: "session-1"})
	require.NoError(t, err)

	rr := doRequest(mux, http.MethodPost, "/session/event", body)

	assert.Equal(t, http.StatusAccepted, rr.Code)
}

func TestSFUMux_SessionEvent_MalformedJSON(t *testing.T) {
	sfu := webrtcsfu.New()
	mux := newSFUMux(sfu)

	rr := doRequest(mux, http.MethodPost, "/session/event", []byte("{not json"))

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestSFUMux_Offer_MalformedJSON(t *testing.T) {
	sfu := webrtcsfu.New()
	mux := newSFUMux(sfu)

	rr := doRequest(mux, http.MethodPost, "/offer/session-1/peer-1", []byte("{not json"))

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestSFUMux_Subscribe_MalformedJSON(t *testing.T) {
	sfu := webrtcsfu.New()
	mux := newSFUMux(sfu)

	rr := doRequest(mux, http.MethodPost, "/subscribe/session-1/operator-1", []byte("{not json"))

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestSFUMux_SessionState_UnknownSession_404(t *testing.T) {
	sfu := webrtcsfu.New()
	mux := newSFUMux(sfu)

	req := httptest.NewRequest(http.MethodGet, "/session/nonexistent/state", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestSFUMux_SessionState_KnownSession_ReturnsRecordedState(t *testing.T) {
	sfu := webrtcsfu.New()
	sfu.HandleSessionEvent(webrtcsfu.SessionEvent{Type: webrtcsfu.EventSafeMode, SessionID: "session-1"})
	mux := newSFUMux(sfu)

	req := httptest.NewRequest(http.MethodGet, "/session/session-1/state", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]string
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, "session-1", resp["session_id"])
	assert.Equal(t, "SESSION_SAFE_MODE", resp["state"])
}

func TestSFUMux_Health(t *testing.T) {
	sfu := webrtcsfu.New()
	mux := newSFUMux(sfu)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]string
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, "ok", resp["status"])
	assert.Equal(t, "webrtc-sfu", resp["service"])
}
