// Package session implements the Session Manager — the Global Session Authority (GSA) of the
// Control Server (ADR-015). Supports multiple concurrent sessions (ADR-025):
// exactly one ACTIVE_OPERATOR per vehicle; all others are OBSERVERs.
package session

import (
	"sync"
	"time"

	"avoc/pkg/ulid"
)

// Session represents an active Control Session (ADR-015).
type Session struct {
	ID           string    // ULID — root anchor, survives SAFE_MODE (ADR-016)
	VehicleID    string
	OperatorID   string
	OperatorRole string // "ACTIVE_OPERATOR" or "OBSERVER"
	CreatedAt    time.Time
}

// RecoveryCheckpoint is saved on every SAFE_MODE entry (ADR-015).
type RecoveryCheckpoint struct {
	SessionID        string
	VehicleID        string
	OperatorID       string
	LastSystemState  string
	LastControlState string
	SafetyReason     string
	CheckpointTS     time.Time
}

// Manager is the Global Session Authority (GSA).
// Multiple sessions can coexist; at most one ACTIVE_OPERATOR per vehicle (ADR-025).
type Manager struct {
	mu                sync.RWMutex
	sessions          map[string]*Session // sessionID → Session
	vehicleController map[string]string   // vehicleID → active controller sessionID
	checkpoint        *RecoveryCheckpoint
	sfuPublisher      SFUPublisher
}

func NewManager(sfuPublisher SFUPublisher) *Manager {
	return &Manager{
		sfuPublisher:      sfuPublisher,
		sessions:          make(map[string]*Session),
		vehicleController: make(map[string]string),
	}
}

// StartSession creates a new session for the given vehicle and operator.
// Returns ACTIVE_OPERATOR if the vehicle has no live controller; OBSERVER otherwise.
func (m *Manager) StartSession(vehicleID, operatorID string) Session {
	m.mu.Lock()
	defer m.mu.Unlock()

	role := "ACTIVE_OPERATOR"
	if ctrlSessID, locked := m.vehicleController[vehicleID]; locked {
		if _, alive := m.sessions[ctrlSessID]; alive {
			role = "OBSERVER"
		} else {
			// Stale lock — controller session was removed without ReleaseSession.
			delete(m.vehicleController, vehicleID)
		}
	}

	s := Session{
		ID:           ulid.Generate(),
		VehicleID:    vehicleID,
		OperatorID:   operatorID,
		OperatorRole: role,
		CreatedAt:    time.Now(),
	}
	m.sessions[s.ID] = &s
	if role == "ACTIVE_OPERATOR" {
		m.vehicleController[vehicleID] = s.ID
	}
	return s
}

// GetSession returns a session by ID. Returns false if the session does not exist.
func (m *Manager) GetSession(sessionID string) (Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[sessionID]
	if !ok {
		return Session{}, false
	}
	return *s, true
}

// GetCurrentSession returns the first ACTIVE_OPERATOR session found.
// Kept for backward compatibility with components that don't track session_id.
func (m *Manager) GetCurrentSession() (Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, s := range m.sessions {
		if s.OperatorRole == "ACTIVE_OPERATOR" {
			return *s, true
		}
	}
	return Session{}, false
}

// HasActiveOperatorSession returns true if the given operatorID has a live ACTIVE_OPERATOR session.
// Used by POST /auth/logout to block logout while in control of a vehicle (ADR-025).
func (m *Manager) HasActiveOperatorSession(operatorID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, s := range m.sessions {
		if s.OperatorID == operatorID && s.OperatorRole == "ACTIVE_OPERATOR" {
			return true
		}
	}
	return false
}

// IsVehicleLocked returns true if a live ACTIVE_OPERATOR session exists for vehicleID.
func (m *Manager) IsVehicleLocked(vehicleID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ctrlSessID, locked := m.vehicleController[vehicleID]
	if !locked {
		return false
	}
	_, alive := m.sessions[ctrlSessID]
	return alive
}

// ReleaseSession removes the session and, if it was the ACTIVE_OPERATOR, releases the vehicle lock.
func (m *Manager) ReleaseSession(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[sessionID]
	if !ok {
		return
	}
	if s.OperatorRole == "ACTIVE_OPERATOR" {
		if ctrlSessID := m.vehicleController[s.VehicleID]; ctrlSessID == sessionID {
			delete(m.vehicleController, s.VehicleID)
		}
	}
	delete(m.sessions, sessionID)
}

// UpdateOperator replaces the active operator for vehicleID — used during
// Handover (ADR-011/015). Scoped to the given vehicle (ADR-026 follow-up) —
// previously updated "the first ACTIVE_OPERATOR session found" fleet-wide,
// which could reassign the wrong vehicle's controller once 2+ vehicles were active.
func (m *Manager) UpdateOperator(vehicleID, operatorID, operatorRole string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.sessions {
		if s.VehicleID == vehicleID && s.OperatorRole == "ACTIVE_OPERATOR" {
			s.OperatorID = operatorID
			s.OperatorRole = operatorRole
			return
		}
	}
}

// EndSession clears all sessions (called on full session reset).
func (m *Manager) EndSession() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions = make(map[string]*Session)
	m.vehicleController = make(map[string]string)
}

// SaveCheckpoint freezes session state at SAFE_MODE entry (ADR-015).
func (m *Manager) SaveCheckpoint(sysState, ctrlState, safetyReason string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Save checkpoint from the ACTIVE_OPERATOR session.
	for _, s := range m.sessions {
		if s.OperatorRole == "ACTIVE_OPERATOR" {
			m.checkpoint = &RecoveryCheckpoint{
				SessionID:        s.ID,
				VehicleID:        s.VehicleID,
				OperatorID:       s.OperatorID,
				LastSystemState:  sysState,
				LastControlState: ctrlState,
				SafetyReason:     safetyReason,
				CheckpointTS:     time.Now(),
			}
			return
		}
	}
}

// LoadCheckpoint returns the last saved recovery checkpoint, if any.
func (m *Manager) LoadCheckpoint() (*RecoveryCheckpoint, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.checkpoint == nil {
		return nil, false
	}
	cp := *m.checkpoint
	return &cp, true
}

// PushSFUEvent sends a session event to the SFU asynchronously (ADR-015).
func (m *Manager) PushSFUEvent(eventType string) {
	m.mu.RLock()
	var s *Session
	for _, sess := range m.sessions {
		if sess.OperatorRole == "ACTIVE_OPERATOR" {
			s = sess
			break
		}
	}
	m.mu.RUnlock()

	if s == nil || m.sfuPublisher == nil {
		return
	}
	go m.sfuPublisher.PublishSessionEvent(eventType, s.ID, s.OperatorID)
}

// ListSessions returns a snapshot of all currently active sessions.
func (m *Manager) ListSessions() []Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		out = append(out, *s)
	}
	return out
}

// GetSessionByVehicle returns the ACTIVE_OPERATOR session for vehicleID, if
// one is currently live. Used by POST /emergency-stop to attribute a scoped
// E-Stop to the correct session_id/operator_id for audit purposes (ADR-026).
func (m *Manager) GetSessionByVehicle(vehicleID string) (Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	sessID, locked := m.vehicleController[vehicleID]
	if !locked {
		return Session{}, false
	}
	s, ok := m.sessions[sessID]
	if !ok {
		return Session{}, false
	}
	return *s, true
}

// ActiveVehicleIDs returns the distinct vehicle IDs with at least one live
// session — used by SafetyBusWatchdog to fan a fleet-wide failure out to
// every affected vehicle (ADR-026).
func (m *Manager) ActiveVehicleIDs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	seen := make(map[string]bool, len(m.sessions))
	out := make([]string, 0, len(m.sessions))
	for _, s := range m.sessions {
		if !seen[s.VehicleID] {
			seen[s.VehicleID] = true
			out = append(out, s.VehicleID)
		}
	}
	return out
}

// CreateSession is kept for backward compatibility (used by handover manager and tests).
// Prefer StartSession for new code.
func (m *Manager) CreateSession(vehicleID, operatorID, operatorRole string) Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := Session{
		ID:           ulid.Generate(),
		VehicleID:    vehicleID,
		OperatorID:   operatorID,
		OperatorRole: operatorRole,
		CreatedAt:    time.Now(),
	}
	m.sessions[s.ID] = &s
	if operatorRole == "ACTIVE_OPERATOR" {
		m.vehicleController[vehicleID] = s.ID
	}
	return s
}
