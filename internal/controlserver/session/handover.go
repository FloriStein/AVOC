package session

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"avoc/internal/controlserver/statemachine"
	"avoc/internal/controlserver/vehiclecontext"
	"avoc/pkg/logger"
)

var svcLog = logger.New("control-server")

// HandoverManager coordinates operator handover (ADR-011/015), isolated per vehicle
// (ADR-026 follow-up, MV-11): resolves each vehicle's own State Machine via the
// shared vehiclecontext.Registry instead of a disconnected standalone Machine, so a
// handover in progress on one vehicle can never block or leak into another's.
// Rules: max 1 ACTIVE_OPERATOR per vehicle, both sides must confirm, current
// operator retains control during HANDOVER_PENDING, SFU notified immediately on
// completion.
type HandoverManager struct {
	mu         sync.Mutex
	vehicles   *vehiclecontext.Registry
	sessions   *Manager
	authURL    string
	httpClient *http.Client
	pending    map[string]*pendingHandover // vehicleID -> pending
}

type pendingHandover struct {
	fromOperatorID string
	toOperatorID   string
}

func NewHandoverManager(vehicles *vehiclecontext.Registry, sessions *Manager, authURL string) *HandoverManager {
	return &HandoverManager{
		vehicles:   vehicles,
		sessions:   sessions,
		authURL:    authURL,
		httpClient: &http.Client{},
		pending:    make(map[string]*pendingHandover),
	}
}

// RequestHandover initiates a handover for vehicleID — transitions its OPERATOR
// STATE to HANDOVER_PENDING. The current operator retains control until
// ConfirmHandover is called (ADR-011).
func (h *HandoverManager) RequestHandover(vehicleID, fromOperatorID, toOperatorID string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	vc := h.vehicles.Get(vehicleID)
	_, _, _, opState := vc.SM.Get()
	if opState != statemachine.OpActive {
		return fmt.Errorf("handover requires ACTIVE_OPERATOR state, got %s", opState)
	}

	h.pending[vehicleID] = &pendingHandover{
		fromOperatorID: fromOperatorID,
		toOperatorID:   toOperatorID,
	}

	vc.SM.TransitionOperator(statemachine.OpHandoverPending)
	svcLog.Info("handover requested",
		"vehicle_id", vehicleID, "from_operator", fromOperatorID, "to_operator", toOperatorID)
	return nil
}

// ConfirmHandover completes the handover for vehicleID — target becomes
// ACTIVE_OPERATOR. Issues a new ACTIVE_OPERATOR token via Auth Service and
// notifies the SFU (ADR-015).
func (h *HandoverManager) ConfirmHandover(vehicleID, confirmingOperatorID string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	pending, ok := h.pending[vehicleID]
	if !ok {
		return fmt.Errorf("no handover pending for vehicle %s", vehicleID)
	}
	if confirmingOperatorID != pending.toOperatorID {
		return fmt.Errorf("confirming operator %s is not the handover target %s",
			confirmingOperatorID, pending.toOperatorID)
	}

	if err := h.issueHandoverToken(pending.toOperatorID); err != nil {
		return fmt.Errorf("handover token issuance failed: %w", err)
	}

	h.sessions.UpdateOperator(vehicleID, pending.toOperatorID, string(statemachine.OpActive))
	h.vehicles.Get(vehicleID).SM.TransitionOperator(statemachine.OpActive)
	h.sessions.PushSFUEvent("OPERATOR_HANDOVER")

	svcLog.Event(logger.EventOperatorHandover, "handover confirmed",
		"vehicle_id", vehicleID, "new_active_operator", pending.toOperatorID)
	delete(h.pending, vehicleID)
	return nil
}

// CancelHandover aborts the handover for vehicleID — current operator retains control.
func (h *HandoverManager) CancelHandover(vehicleID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	pending, ok := h.pending[vehicleID]
	if !ok {
		return
	}
	svcLog.Info("handover cancelled",
		"vehicle_id", vehicleID, "from_operator", pending.fromOperatorID, "to_operator", pending.toOperatorID)
	delete(h.pending, vehicleID)
	h.vehicles.Get(vehicleID).SM.TransitionOperator(statemachine.OpActive)
}

// IsPending returns true if a handover is currently in progress for vehicleID.
func (h *HandoverManager) IsPending(vehicleID string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	_, ok := h.pending[vehicleID]
	return ok
}

func (h *HandoverManager) issueHandoverToken(targetOperatorID string) error {
	if h.authURL == "" {
		return nil // no auth service configured (e.g. in tests)
	}
	body, _ := json.Marshal(map[string]string{"target_id": targetOperatorID})
	resp, err := h.httpClient.Post(h.authURL+"/auth/handover/token", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("auth service returned %d", resp.StatusCode)
	}
	return nil
}
