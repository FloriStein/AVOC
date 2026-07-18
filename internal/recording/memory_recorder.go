package recording

import (
	"log"
	"sync"
	"time"
)

// MemoryRecorder stores recording entries in-memory (ADR-005 Sprint-4 adapter).
// Not persistent — survives only for the lifetime of the process.
// Replace with a storage adapter once ADR-005 Folge (storage decision) is resolved.
type MemoryRecorder struct {
	mu      sync.RWMutex
	entries map[string][]Entry // keyed by session_id (ULID root anchor)
}

func NewMemoryRecorder() *MemoryRecorder {
	return &MemoryRecorder{entries: make(map[string][]Entry)}
}

func (r *MemoryRecorder) StartSession(sessionID, vehicleID, operatorID string) {
	r.mu.Lock()
	r.entries[sessionID] = []Entry{}
	r.mu.Unlock()
	log.Printf("[RECORDING] session started: id=%s vehicle=%s operator=%s", sessionID, vehicleID, operatorID)
}

func (r *MemoryRecorder) EndSession(sessionID string) {
	r.mu.RLock()
	count := len(r.entries[sessionID])
	r.mu.RUnlock()
	log.Printf("[RECORDING] session ended: id=%s entries=%d", sessionID, count)
}

func (r *MemoryRecorder) RecordControlEvent(p ControlEventParams) {
	r.append(Entry{
		SessionID:   p.SessionID,
		EventID:     p.EventID,
		VehicleID:   p.VehicleID,
		OperatorID:  p.OperatorID,
		Timestamp:   time.Now(),
		EntryType:   "control",
		CommandType: p.CommandType,
		Value:       p.Value,
	})
}

func (r *MemoryRecorder) RecordStateSnapshot(p StateSnapshotParams) {
	r.append(Entry{
		SessionID:   p.SessionID,
		EventID:     p.EventID,
		VehicleID:   p.VehicleID,
		OperatorID:  p.OperatorID,
		Timestamp:   time.Now(),
		EntryType:   "state",
		SystemState: p.SystemState,
		CtrlState:   p.CtrlState,
	})
}

func (r *MemoryRecorder) RecordSafetyEvent(p SafetyEventParams) {
	r.append(Entry{
		SessionID:  p.SessionID,
		EventID:    p.EventID,
		VehicleID:  p.VehicleID,
		OperatorID: p.OperatorID,
		Timestamp:  time.Now(),
		EntryType:  "safety",
		EventType:  p.EventType,
		Reason:     p.Reason,
	})
	log.Printf("[RECORDING] safety event: session=%s type=%s reason=%s", p.SessionID, p.EventType, p.Reason)
}

func (r *MemoryRecorder) GetEntries(sessionID string) []Entry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entries := r.entries[sessionID]
	result := make([]Entry, len(entries))
	copy(result, entries)
	return result
}

func (r *MemoryRecorder) append(e Entry) {
	r.mu.Lock()
	r.entries[e.SessionID] = append(r.entries[e.SessionID], e)
	r.mu.Unlock()
}
