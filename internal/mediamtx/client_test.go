package mediamtx

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// callLog records every request the test server received, guarded by mu — needed because
// KickVehicle fires deleteSession sequentially but tests also exercise it concurrently (race case).
type callLog struct {
	mu    sync.Mutex
	paths []string
}

func (c *callLog) record(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.paths = append(c.paths, path)
}

func (c *callLog) count(path string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, p := range c.paths {
		if p == path {
			n++
		}
	}
	return n
}

func newListHandler(items []webrtcSession) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(webrtcSessionsResponse{Items: items})
	}
}

// TestListSessions_FiltersByVehiclePath verifies the client only returns sessions whose Path
// matches the requested vehicleID (own filtering logic, not delegated to the server).
func TestListSessions_FiltersByVehiclePath(t *testing.T) {
	server := httptest.NewServer(newListHandler([]webrtcSession{
		{ID: "s1", Path: "vehicle-001"},
		{ID: "s2", Path: "vehicle-002"},
		{ID: "s3", Path: "vehicle-001"},
	}))
	defer server.Close()

	c := NewClient(server.URL)
	sessions, err := c.listSessions("vehicle-001")

	require.NoError(t, err)
	assert.Len(t, sessions, 2)
	assert.ElementsMatch(t, []string{"s1", "s3"}, []string{sessions[0].ID, sessions[1].ID})
}

// TestListSessions_EmptyItems covers the boundary case of no sessions at all.
func TestListSessions_EmptyItems(t *testing.T) {
	server := httptest.NewServer(newListHandler(nil))
	defer server.Close()

	c := NewClient(server.URL)
	sessions, err := c.listSessions("vehicle-001")

	require.NoError(t, err)
	assert.Empty(t, sessions)
}

// TestListSessions_MalformedJSON covers a malformed response body from MediaMTX.
func TestListSessions_MalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("{not valid json"))
	}))
	defer server.Close()

	c := NewClient(server.URL)
	sessions, err := c.listSessions("vehicle-001")

	require.Error(t, err)
	assert.Nil(t, sessions)
}

// TestListSessions_NetworkError covers an unreachable MediaMTX API (connection refused).
func TestListSessions_NetworkError(t *testing.T) {
	server := httptest.NewServer(newListHandler(nil))
	unreachableURL := server.URL
	server.Close() // closed immediately — URL now refuses connections

	c := NewClient(unreachableURL)
	sessions, err := c.listSessions("vehicle-001")

	require.Error(t, err)
	assert.Nil(t, sessions)
}

// TestListSessions_Timeout covers a MediaMTX API that hangs beyond the client's timeout.
func TestListSessions_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		_ = json.NewEncoder(w).Encode(webrtcSessionsResponse{})
	}))
	defer server.Close()

	c := NewClient(server.URL)
	c.http = &http.Client{Timeout: 20 * time.Millisecond}

	sessions, err := c.listSessions("vehicle-001")

	require.Error(t, err)
	assert.Nil(t, sessions)
}

// TestDeleteSession_Success covers the happy path of kicking a subscriber.
func TestDeleteSession_Success(t *testing.T) {
	log := &callLog{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.record(r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := NewClient(server.URL)
	err := c.deleteSession("session-42")

	require.NoError(t, err)
	assert.Equal(t, 1, log.count("/v3/webrtcsessions/kick/session-42"))
}

// TestDeleteSession_NonSuccessStatus covers MediaMTX rejecting the kick request.
func TestDeleteSession_NonSuccessStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	c := NewClient(server.URL)
	err := c.deleteSession("session-missing")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "404")
}

// TestDeleteSession_NetworkError covers an unreachable MediaMTX API during the kick call.
func TestDeleteSession_NetworkError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	unreachableURL := server.URL
	server.Close()

	c := NewClient(unreachableURL)
	err := c.deleteSession("session-1")

	require.Error(t, err)
}

// TestKickVehicle_Success_KicksOnlyMatchingSessions is the end-to-end happy path: two sessions
// match the vehicle path, one belongs to another vehicle and must be left alone.
func TestKickVehicle_Success_KicksOnlyMatchingSessions(t *testing.T) {
	log := &callLog{}
	mux := http.NewServeMux()
	mux.HandleFunc("/v3/webrtcsessions/list", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(webrtcSessionsResponse{Items: []webrtcSession{
			{ID: "s1", Path: "vehicle-001"},
			{ID: "s2", Path: "vehicle-002"},
			{ID: "s3", Path: "vehicle-001"},
		}})
	})
	mux.HandleFunc("/v3/webrtcsessions/kick/", func(w http.ResponseWriter, r *http.Request) {
		log.record(r.URL.Path)
		w.WriteHeader(http.StatusOK)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	c := NewClient(server.URL)
	c.KickVehicle("vehicle-001")

	assert.Equal(t, 1, log.count("/v3/webrtcsessions/kick/s1"))
	assert.Equal(t, 1, log.count("/v3/webrtcsessions/kick/s3"))
	assert.Equal(t, 0, log.count("/v3/webrtcsessions/kick/s2"))
}

// TestKickVehicle_NoMatchingSessions covers the boundary case where the vehicle has no active
// subscribers — no kick request must be sent.
func TestKickVehicle_NoMatchingSessions(t *testing.T) {
	log := &callLog{}
	mux := http.NewServeMux()
	mux.HandleFunc("/v3/webrtcsessions/list", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(webrtcSessionsResponse{Items: []webrtcSession{
			{ID: "s1", Path: "vehicle-999"},
		}})
	})
	mux.HandleFunc("/v3/webrtcsessions/kick/", func(w http.ResponseWriter, r *http.Request) {
		log.record(r.URL.Path)
		w.WriteHeader(http.StatusOK)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	c := NewClient(server.URL)
	c.KickVehicle("vehicle-001")

	assert.Empty(t, log.paths)
}

// TestKickVehicle_EmptyVehicleID is a boundary case — must behave like any other non-matching ID
// (no panic, no kick calls), not a special "kick everything" wildcard.
func TestKickVehicle_EmptyVehicleID(t *testing.T) {
	log := &callLog{}
	mux := http.NewServeMux()
	mux.HandleFunc("/v3/webrtcsessions/list", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(webrtcSessionsResponse{Items: []webrtcSession{
			{ID: "s1", Path: "vehicle-001"},
		}})
	})
	mux.HandleFunc("/v3/webrtcsessions/kick/", func(w http.ResponseWriter, r *http.Request) {
		log.record(r.URL.Path)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	c := NewClient(server.URL)
	assert.NotPanics(t, func() { c.KickVehicle("") })
	assert.Empty(t, log.paths)
}

// TestKickVehicle_ListError_LogsAndReturns covers the swallow path: KickVehicle is called from
// the SAFE_MODE trigger path and must never panic or block the caller when MediaMTX itself is
// unreachable — the media kick is best-effort.
func TestKickVehicle_ListError_LogsAndReturns(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	unreachableURL := server.URL
	server.Close()

	c := NewClient(unreachableURL)
	assert.NotPanics(t, func() { c.KickVehicle("vehicle-001") })
}

// TestKickVehicle_ListMalformedJSON_LogsAndReturns covers a malformed list response — must not
// panic and must not attempt any kick call.
func TestKickVehicle_ListMalformedJSON_LogsAndReturns(t *testing.T) {
	log := &callLog{}
	mux := http.NewServeMux()
	mux.HandleFunc("/v3/webrtcsessions/list", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not json"))
	})
	mux.HandleFunc("/v3/webrtcsessions/kick/", func(w http.ResponseWriter, r *http.Request) {
		log.record(r.URL.Path)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	c := NewClient(server.URL)
	assert.NotPanics(t, func() { c.KickVehicle("vehicle-001") })
	assert.Empty(t, log.paths)
}

// TestKickVehicle_OneDeleteFails_LoopContinues verifies that a failed kick for one session does
// not abort kicking the remaining sessions of the same vehicle (fire-and-forget per session).
func TestKickVehicle_OneDeleteFails_LoopContinues(t *testing.T) {
	log := &callLog{}
	mux := http.NewServeMux()
	mux.HandleFunc("/v3/webrtcsessions/list", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(webrtcSessionsResponse{Items: []webrtcSession{
			{ID: "fails", Path: "vehicle-001"},
			{ID: "succeeds", Path: "vehicle-001"},
		}})
	})
	mux.HandleFunc("/v3/webrtcsessions/kick/fails", func(w http.ResponseWriter, r *http.Request) {
		log.record(r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
	})
	mux.HandleFunc("/v3/webrtcsessions/kick/succeeds", func(w http.ResponseWriter, r *http.Request) {
		log.record(r.URL.Path)
		w.WriteHeader(http.StatusOK)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	c := NewClient(server.URL)
	c.KickVehicle("vehicle-001")

	assert.Equal(t, 1, log.count("/v3/webrtcsessions/kick/fails"))
	assert.Equal(t, 1, log.count("/v3/webrtcsessions/kick/succeeds"))
}

// TestKickVehicle_Idempotent_SecondCallFindsNothing covers repeated invocation: once all sessions
// for a vehicle have been kicked, a second call must find nothing left to do and not error.
func TestKickVehicle_Idempotent_SecondCallFindsNothing(t *testing.T) {
	kicked := make(map[string]bool)
	var mu sync.Mutex
	mux := http.NewServeMux()
	mux.HandleFunc("/v3/webrtcsessions/list", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		var items []webrtcSession
		if !kicked["s1"] {
			items = append(items, webrtcSession{ID: "s1", Path: "vehicle-001"})
		}
		_ = json.NewEncoder(w).Encode(webrtcSessionsResponse{Items: items})
	})
	mux.HandleFunc("/v3/webrtcsessions/kick/s1", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		kicked["s1"] = true
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	c := NewClient(server.URL)
	assert.NotPanics(t, func() { c.KickVehicle("vehicle-001") })
	assert.NotPanics(t, func() { c.KickVehicle("vehicle-001") })

	mu.Lock()
	defer mu.Unlock()
	assert.True(t, kicked["s1"])
}

// TestKickVehicle_ConcurrentCalls exercises the client from multiple goroutines at once (run with
// -race) — Client holds no mutable state beyond the *http.Client, so concurrent KickVehicle calls
// for different vehicles must not race.
func TestKickVehicle_ConcurrentCalls(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v3/webrtcsessions/list", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(webrtcSessionsResponse{Items: []webrtcSession{
			{ID: "s1", Path: "vehicle-001"},
		}})
	})
	mux.HandleFunc("/v3/webrtcsessions/kick/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	c := NewClient(server.URL)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		vehicleID := fmt.Sprintf("vehicle-%03d", i)
		go func() {
			defer wg.Done()
			c.KickVehicle(vehicleID)
		}()
	}
	wg.Wait()
}
