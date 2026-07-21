package webrtcsfu

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/pion/webrtc/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestPeerConnection(t *testing.T) *webrtc.PeerConnection {
	t.Helper()
	pc, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = pc.Close() })
	return pc
}

func TestHandleSessionEvent_Created_SetsEmptyRouting(t *testing.T) {
	s := New()

	s.HandleSessionEvent(SessionEvent{Type: EventCreated, SessionID: "session-1"})

	assert.Equal(t, []string{}, s.routing["session-1"])
}

func TestGetSessionState_UnknownSession_ReturnsFalse(t *testing.T) {
	s := New()

	_, ok := s.GetSessionState("nonexistent")

	assert.False(t, ok)
}

func TestGetSessionState_ReturnsLastRecordedEvent(t *testing.T) {
	s := New()
	s.HandleSessionEvent(SessionEvent{Type: EventCreated, SessionID: "session-1"})
	s.HandleSessionEvent(SessionEvent{Type: EventSafeMode, SessionID: "session-1"})

	state, ok := s.GetSessionState("session-1")

	require.True(t, ok)
	assert.Equal(t, EventSafeMode, state)
}

func TestHandleSessionEvent_OperatorAssigned_SetsRoutingToOperator(t *testing.T) {
	s := New()

	s.HandleSessionEvent(SessionEvent{Type: EventOperatorAssigned, SessionID: "session-1", OperatorID: "operator-1"})

	assert.Equal(t, []string{"operator-1"}, s.routing["session-1"])
}

func TestHandleSessionEvent_OperatorHandover_SetsRoutingToOperator(t *testing.T) {
	s := New()
	s.HandleSessionEvent(SessionEvent{Type: EventOperatorAssigned, SessionID: "session-1", OperatorID: "operator-1"})

	s.HandleSessionEvent(SessionEvent{Type: EventOperatorHandover, SessionID: "session-1", OperatorID: "operator-2"})

	assert.Equal(t, []string{"operator-2"}, s.routing["session-1"])
}

func TestHandleSessionEvent_SafeMode_DropsActivePeersButKeepsState(t *testing.T) {
	s := New()
	pc := newTestPeerConnection(t)
	s.peers["peer-1"] = &Peer{ID: "peer-1", SessionID: "session-1", Connection: pc}
	s.routing["session-1"] = []string{"operator-1"}

	s.HandleSessionEvent(SessionEvent{Type: EventSafeMode, SessionID: "session-1"})

	_, stillPresent := s.peers["peer-1"]
	assert.False(t, stillPresent)
	assert.Equal(t, EventSafeMode, s.state["session-1"])
}

func TestHandleSessionEvent_SafeMode_OnlyDropsPeersOfThatSession(t *testing.T) {
	s := New()
	pcA := newTestPeerConnection(t)
	pcB := newTestPeerConnection(t)
	s.peers["peer-a"] = &Peer{ID: "peer-a", SessionID: "session-a", Connection: pcA}
	s.peers["peer-b"] = &Peer{ID: "peer-b", SessionID: "session-b", Connection: pcB}

	s.HandleSessionEvent(SessionEvent{Type: EventSafeMode, SessionID: "session-a"})

	_, aPresent := s.peers["peer-a"]
	_, bPresent := s.peers["peer-b"]
	assert.False(t, aPresent)
	assert.True(t, bPresent)
}

func TestHandleSessionEvent_Ended_ClearsRoutingAndState(t *testing.T) {
	s := New()
	pc := newTestPeerConnection(t)
	s.peers["peer-1"] = &Peer{ID: "peer-1", SessionID: "session-1", Connection: pc}
	s.HandleSessionEvent(SessionEvent{Type: EventOperatorAssigned, SessionID: "session-1", OperatorID: "operator-1"})

	s.HandleSessionEvent(SessionEvent{Type: EventEnded, SessionID: "session-1"})

	_, routingPresent := s.routing["session-1"]
	_, statePresent := s.state["session-1"]
	_, peerPresent := s.peers["peer-1"]
	assert.False(t, routingPresent)
	assert.False(t, statePresent)
	assert.False(t, peerPresent)
}

func TestRegisterOperatorSubscription_FirstCall_AppendsAndReturnsFalse(t *testing.T) {
	s := New()
	pc := newTestPeerConnection(t)
	track, err := webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8}, "video", "test")
	require.NoError(t, err)

	alreadyRouted := s.registerOperatorSubscription("session-1", "operator-1", pc, track)

	assert.False(t, alreadyRouted)
	assert.Equal(t, []string{"operator-1"}, s.routing["session-1"])
	assert.Same(t, pc, s.peers["operator-1"].Connection)
}

func TestRegisterOperatorSubscription_SecondCall_SameOperator_RoutingUnchangedButPeerReplaced(t *testing.T) {
	s := New()
	pc1 := newTestPeerConnection(t)
	track1, err := webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8}, "video", "test")
	require.NoError(t, err)
	s.registerOperatorSubscription("session-1", "operator-1", pc1, track1)

	pc2 := newTestPeerConnection(t)
	track2, err := webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8}, "video", "test")
	require.NoError(t, err)
	alreadyRouted := s.registerOperatorSubscription("session-1", "operator-1", pc2, track2)

	assert.True(t, alreadyRouted)
	assert.Equal(t, []string{"operator-1"}, s.routing["session-1"])
	assert.Same(t, pc2, s.peers["operator-1"].Connection)
}

func TestRemovePeer_RemovesFromMap(t *testing.T) {
	s := New()
	pc := newTestPeerConnection(t)
	s.peers["peer-1"] = &Peer{ID: "peer-1", SessionID: "session-1", Connection: pc}

	s.removePeer("peer-1")

	_, present := s.peers["peer-1"]
	assert.False(t, present)
}

func TestRemovePeer_UnknownPeerID_NoPanic(t *testing.T) {
	s := New()

	assert.NotPanics(t, func() {
		s.removePeer("does-not-exist")
	})
}

// TestSFU_ConcurrentSessionEventsAndPeerOps drives HandleSessionEvent, registerOperatorSubscription
// and removePeer from many goroutines at once (analog internal/safetyservice/bus_test.go's
// TestBus_ConcurrentPublishAndRead) to exercise s.mu under concurrent map access — run with
// `go test -race`.
func TestSFU_ConcurrentSessionEventsAndPeerOps(t *testing.T) {
	s := New()

	const n = 20
	type peerCall struct {
		operatorID string
		pc         *webrtc.PeerConnection
		track      *webrtc.TrackLocalStaticRTP
	}
	calls := make([]peerCall, n)
	for i := 0; i < n; i++ {
		track, err := webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8}, "video", "test")
		require.NoError(t, err)
		calls[i] = peerCall{
			operatorID: fmt.Sprintf("operator-%d", i),
			pc:         newTestPeerConnection(t),
			track:      track,
		}
	}

	var wg sync.WaitGroup
	for _, c := range calls {
		c := c
		wg.Add(3)
		go func() {
			defer wg.Done()
			s.HandleSessionEvent(SessionEvent{Type: EventOperatorAssigned, SessionID: "session-1", OperatorID: "operator-1"})
		}()
		go func() {
			defer wg.Done()
			s.registerOperatorSubscription("session-1", c.operatorID, c.pc, c.track)
		}()
		go func() {
			defer wg.Done()
			s.removePeer(c.operatorID)
		}()
	}
	waitOrTimeout(t, &wg, 5*time.Second)

	// All HandleSessionEvent calls publish the same event type, so the outcome is deterministic
	// regardless of goroutine interleaving — unlike peers/routing, whose exact contents legitimately
	// depend on the race between concurrent registerOperatorSubscription/removePeer calls.
	assert.Equal(t, EventOperatorAssigned, s.state["session-1"])
}

func waitOrTimeout(t *testing.T, wg *sync.WaitGroup, timeout time.Duration) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
		t.Fatal("timed out waiting for concurrent SFU operations to finish")
	}
}
