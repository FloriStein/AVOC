package session

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// stubSFUPublisher is a no-op SFUPublisher — ListSessions tests don't exercise SFU push at all.
type stubSFUPublisher struct{}

func (stubSFUPublisher) PublishSessionEvent(string, string, string) {}

func TestListSessions_Empty(t *testing.T) {
	mgr := NewManager(stubSFUPublisher{})

	sessions := mgr.ListSessions()

	assert.NotNil(t, sessions, "must return an empty slice, not nil, for JSON serialisation")
	assert.Empty(t, sessions)
}

func TestListSessions_ReturnsAllActiveSessions(t *testing.T) {
	mgr := NewManager(stubSFUPublisher{})
	s1 := mgr.StartSession("vehicle-001", "operator-1")
	s2 := mgr.StartSession("vehicle-002", "operator-2")

	sessions := mgr.ListSessions()

	assert.Len(t, sessions, 2)
	ids := []string{sessions[0].ID, sessions[1].ID}
	assert.ElementsMatch(t, []string{s1.ID, s2.ID}, ids)
}

func TestListSessions_MultipleObserversOnSameVehicle(t *testing.T) {
	mgr := NewManager(stubSFUPublisher{})
	mgr.StartSession("vehicle-001", "operator-1") // becomes ACTIVE_OPERATOR
	mgr.StartSession("vehicle-001", "operator-2") // becomes OBSERVER

	sessions := mgr.ListSessions()

	assert.Len(t, sessions, 2)
	roles := map[string]int{}
	for _, s := range sessions {
		roles[s.OperatorRole]++
	}
	assert.Equal(t, 1, roles["ACTIVE_OPERATOR"])
	assert.Equal(t, 1, roles["OBSERVER"])
}

// TestListSessions_ReturnsSnapshotNotAlias verifies mutating the returned slice's elements does
// not corrupt the Manager's internal state — ListSessions must return copies (session.Session is
// a value type dereferenced from the internal *Session map, see manager.go).
func TestListSessions_ReturnsSnapshotNotAlias(t *testing.T) {
	mgr := NewManager(stubSFUPublisher{})
	s := mgr.StartSession("vehicle-001", "operator-1")

	sessions := mgr.ListSessions()
	assert.Len(t, sessions, 1)
	sessions[0].OperatorID = "tampered"

	stored, ok := mgr.GetSession(s.ID)
	assert.True(t, ok)
	assert.Equal(t, "operator-1", stored.OperatorID, "mutating the returned snapshot must not affect internal state")
}

func TestListSessions_AfterReleaseSession_ExcludesReleased(t *testing.T) {
	mgr := NewManager(stubSFUPublisher{})
	s1 := mgr.StartSession("vehicle-001", "operator-1")
	mgr.StartSession("vehicle-002", "operator-2")
	mgr.ReleaseSession(s1.ID)

	sessions := mgr.ListSessions()

	assert.Len(t, sessions, 1)
	assert.Equal(t, "operator-2", sessions[0].OperatorID)
}

// TestListSessions_ConcurrentWithStartSession exercises ListSessions racing against StartSession
// (run with -race) — both take m.mu (RLock/Lock respectively), so no data race should occur.
func TestListSessions_ConcurrentWithStartSession(t *testing.T) {
	mgr := NewManager(stubSFUPublisher{})
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			mgr.StartSession("vehicle-001", "operator")
		}(i)
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = mgr.ListSessions()
		}()
	}
	wg.Wait()
}
