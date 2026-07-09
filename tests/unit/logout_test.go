// Logout Gate Tests (ADR-025).
// Verifies that POST /auth/logout blocks when the operator has an active
// ACTIVE_OPERATOR session and allows logout otherwise.
package unit_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"avoc/internal/controlserver/session"
	"avoc/internal/controlserver/statemachine"
	"avoc/tests/unit/mocks"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── HasActiveOperatorSession unit tests ───────────────────────────────────────

func TestHasActiveOperatorSession_NoSessions(t *testing.T) {
	mgr := newMgr(t)
	assert.False(t, mgr.HasActiveOperatorSession("op-1"),
		"no sessions → must return false")
}

func TestHasActiveOperatorSession_ActiveOperatorExists(t *testing.T) {
	mgr := newMgr(t)
	mgr.StartSession("vehicle-001", "op-1")
	assert.True(t, mgr.HasActiveOperatorSession("op-1"),
		"op-1 has ACTIVE_OPERATOR session → must return true")
}

func TestHasActiveOperatorSession_ObserverOnly(t *testing.T) {
	mgr := newMgr(t)
	// op-1 takes ACTIVE_OPERATOR
	mgr.StartSession("vehicle-001", "op-1")
	// op-2 gets OBSERVER (vehicle already controlled)
	mgr.StartSession("vehicle-001", "op-2")

	assert.False(t, mgr.HasActiveOperatorSession("op-2"),
		"op-2 is only OBSERVER → must return false (OBSERVER kann sich abmelden)")
}

func TestHasActiveOperatorSession_WrongOperator(t *testing.T) {
	mgr := newMgr(t)
	mgr.StartSession("vehicle-001", "op-1")
	assert.False(t, mgr.HasActiveOperatorSession("op-2"),
		"op-2 hat keine Session → must return false")
}

func TestHasActiveOperatorSession_AfterRelease(t *testing.T) {
	mgr := newMgr(t)
	sess := mgr.StartSession("vehicle-001", "op-1")
	assert.True(t, mgr.HasActiveOperatorSession("op-1"))

	mgr.ReleaseSession(sess.ID)
	assert.False(t, mgr.HasActiveOperatorSession("op-1"),
		"nach ReleaseSession → must return false")
}

func TestHasActiveOperatorSession_MultipleVehicles(t *testing.T) {
	mgr := newMgr(t)
	mgr.StartSession("vehicle-001", "op-1")
	mgr.StartSession("vehicle-002", "op-2")

	assert.True(t, mgr.HasActiveOperatorSession("op-1"))
	assert.True(t, mgr.HasActiveOperatorSession("op-2"))
	assert.False(t, mgr.HasActiveOperatorSession("op-3"),
		"op-3 hat keine Session → false")
}

func TestHasActiveOperatorSession_StaleLock(t *testing.T) {
	mgr := newMgr(t)
	sess := mgr.StartSession("vehicle-001", "op-1")
	// Release without going through normal path (simulates crash/restart)
	mgr.ReleaseSession(sess.ID)

	// op-2 now gets ACTIVE_OPERATOR (stale lock cleaned up in StartSession)
	mgr.StartSession("vehicle-001", "op-2")
	assert.False(t, mgr.HasActiveOperatorSession("op-1"),
		"op-1 Session wurde released → false")
	assert.True(t, mgr.HasActiveOperatorSession("op-2"))
}

// ── HTTP handler tests for POST /auth/logout ──────────────────────────────────

const testJWTSecret = "test-secret-for-logout-tests"

// signToken creates a minimal JWT with the given subject (operator ID).
func signToken(t *testing.T, subject string) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   subject,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	})
	signed, err := tok.SignedString([]byte(testJWTSecret))
	require.NoError(t, err)
	return signed
}

// buildLogoutHandler wires up a minimal mux with POST /auth/logout
// using the same logic as cmd/control-server/main.go.
func buildLogoutHandler(mgr *session.Manager) http.Handler {
	type contextKey string
	const claimsKey contextKey = "jwt_subject"

	requireJWT := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if !strings.HasPrefix(authHeader, "Bearer ") {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			tok, err := jwt.Parse(strings.TrimPrefix(authHeader, "Bearer "),
				func(t *jwt.Token) (any, error) {
					if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
						return nil, fmt.Errorf("unexpected alg")
					}
					return []byte(testJWTSecret), nil
				})
			if err != nil || !tok.Valid {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			subject, _ := tok.Claims.GetSubject()
			ctx := context.WithValue(r.Context(), claimsKey, subject)
			next(w, r.WithContext(ctx))
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/logout", requireJWT(func(w http.ResponseWriter, r *http.Request) {
		operatorID, _ := r.Context().Value(claimsKey).(string)
		if mgr.HasActiveOperatorSession(operatorID) {
			http.Error(w, `{"error":"active_session","message":"Session erst beenden"}`, http.StatusConflict)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	return mux
}

func logoutRequest(t *testing.T, srv *httptest.Server, token string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/auth/logout", bytes.NewReader(nil))
	require.NoError(t, err)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return res
}

func TestLogout_NoActiveSession_Returns204(t *testing.T) {
	mgr := session.NewManager(&mocks.MockSFUPublisher{})
	srv := httptest.NewServer(buildLogoutHandler(mgr))
	defer srv.Close()

	token := signToken(t, "op-1")
	res := logoutRequest(t, srv, token)
	assert.Equal(t, http.StatusNoContent, res.StatusCode,
		"kein aktive Session → 204 No Content")
}

func TestLogout_ActiveOperatorSession_Returns409(t *testing.T) {
	mgr := session.NewManager(&mocks.MockSFUPublisher{})
	mgr.StartSession("vehicle-001", "op-1")
	srv := httptest.NewServer(buildLogoutHandler(mgr))
	defer srv.Close()

	token := signToken(t, "op-1")
	res := logoutRequest(t, srv, token)
	assert.Equal(t, http.StatusConflict, res.StatusCode,
		"aktive ACTIVE_OPERATOR Session → 409 Conflict")

	var body map[string]string
	json.NewDecoder(res.Body).Decode(&body)
	assert.Equal(t, "active_session", body["error"])
}

func TestLogout_ObserverSession_Returns204(t *testing.T) {
	mgr := session.NewManager(&mocks.MockSFUPublisher{})
	// op-1 hat ACTIVE_OPERATOR
	mgr.StartSession("vehicle-001", "op-1")
	// op-2 hat nur OBSERVER
	mgr.StartSession("vehicle-001", "op-2")
	srv := httptest.NewServer(buildLogoutHandler(mgr))
	defer srv.Close()

	token := signToken(t, "op-2")
	res := logoutRequest(t, srv, token)
	assert.Equal(t, http.StatusNoContent, res.StatusCode,
		"OBSERVER kann sich jederzeit abmelden → 204")
}

func TestLogout_MissingToken_Returns401(t *testing.T) {
	mgr := session.NewManager(&mocks.MockSFUPublisher{})
	srv := httptest.NewServer(buildLogoutHandler(mgr))
	defer srv.Close()

	res := logoutRequest(t, srv, "") // kein Token
	assert.Equal(t, http.StatusUnauthorized, res.StatusCode)
}

func TestLogout_InvalidToken_Returns401(t *testing.T) {
	mgr := session.NewManager(&mocks.MockSFUPublisher{})
	srv := httptest.NewServer(buildLogoutHandler(mgr))
	defer srv.Close()

	res := logoutRequest(t, srv, "not.a.valid.jwt")
	assert.Equal(t, http.StatusUnauthorized, res.StatusCode)
}

func TestLogout_AfterSessionEnd_Returns204(t *testing.T) {
	mgr := session.NewManager(&mocks.MockSFUPublisher{})
	sess := mgr.StartSession("vehicle-001", "op-1")
	srv := httptest.NewServer(buildLogoutHandler(mgr))
	defer srv.Close()

	token := signToken(t, "op-1")

	// Während Session aktiv → 409
	res := logoutRequest(t, srv, token)
	assert.Equal(t, http.StatusConflict, res.StatusCode, "aktive Session → 409")

	// Session beenden
	mgr.ReleaseSession(sess.ID)

	// Jetzt → 204
	res = logoutRequest(t, srv, token)
	assert.Equal(t, http.StatusNoContent, res.StatusCode, "nach Session-Ende → 204")
}

func TestLogout_OtherOperatorHasSession_Returns204(t *testing.T) {
	mgr := session.NewManager(&mocks.MockSFUPublisher{})
	// op-1 hat Session, op-2 nicht
	mgr.StartSession("vehicle-001", "op-1")
	srv := httptest.NewServer(buildLogoutHandler(mgr))
	defer srv.Close()

	// op-2 versucht sich abzumelden → soll gehen (hat keine Session)
	token := signToken(t, "op-2")
	res := logoutRequest(t, srv, token)
	assert.Equal(t, http.StatusNoContent, res.StatusCode,
		"op-2 hat keine Session → 204, obwohl op-1 eine hat")
}

func TestLogout_Concurrent_SafeUnderRace(t *testing.T) {
	mgr := session.NewManager(&mocks.MockSFUPublisher{})
	sess := mgr.StartSession("vehicle-001", "op-1")
	srv := httptest.NewServer(buildLogoutHandler(mgr))
	defer srv.Close()

	token := signToken(t, "op-1")

	// 20 gleichzeitige Logout-Versuche während Session läuft → alle 409
	results := make([]int, 20)
	var wg strings.Builder // dummy, only for sync
	_ = wg
	ch := make(chan int, 20)
	for i := range 20 {
		go func(i int) {
			res := logoutRequest(t, srv, token)
			ch <- res.StatusCode
		}(i)
	}
	for i := range 20 {
		results[i] = <-ch
	}
	for _, code := range results {
		assert.Equal(t, http.StatusConflict, code, "concurrent logout with active session → all 409")
	}

	// Session freigeben, dann alle 20 → 204
	mgr.ReleaseSession(sess.ID)
	ch2 := make(chan int, 20)
	for range 20 {
		go func() {
			res := logoutRequest(t, srv, token)
			ch2 <- res.StatusCode
		}()
	}
	for i := range 20 {
		results[i] = <-ch2
	}
	for _, code := range results {
		assert.Equal(t, http.StatusNoContent, code, "nach Release → alle 204")
	}
}

// ── State machine: SAFE_MODE → IDLE transition ────────────────────────────────

func TestStateMachine_SafeMode_To_Idle_Allowed(t *testing.T) {
	sm := statemachine.New()
	sm.TransitionSystem(statemachine.StateConnecting)
	sm.TransitionSystem(statemachine.StateAuthenticated)
	sm.TransitionToConnected()
	sm.TransitionSystem(statemachine.StateSafeMode)

	current, _, _, _ := sm.Get()
	require.Equal(t, statemachine.StateSafeMode, current)

	sm.TransitionSystem(statemachine.StateIdle)
	current, ctrl, _, op := sm.Get()
	assert.Equal(t, statemachine.StateIdle, current, "SAFE_MODE → IDLE muss erlaubt sein")
	assert.Equal(t, statemachine.ControlInit, ctrl, "Control muss auf INIT zurückgesetzt sein")
	assert.Equal(t, statemachine.OpNoOperator, op, "Operator muss auf NO_OPERATOR zurückgesetzt sein")
}

func TestStateMachine_SafeMode_To_Idle_ResetsControl(t *testing.T) {
	sm := statemachine.New()
	sm.TransitionSystem(statemachine.StateConnecting)
	sm.TransitionSystem(statemachine.StateAuthenticated)
	sm.TransitionToConnected()
	sm.TransitionOperator(statemachine.OpActive)
	sm.TransitionSystem(statemachine.StateSafeMode)

	_, ctrl, _, _ := sm.Get()
	require.Equal(t, statemachine.ControlBlocked, ctrl, "SAFE_MODE → CONTROL_BLOCKED")

	sm.TransitionSystem(statemachine.StateIdle)
	_, ctrl, _, op := sm.Get()
	assert.Equal(t, statemachine.ControlInit, ctrl, "IDLE → CONTROL_INIT")
	assert.Equal(t, statemachine.OpNoOperator, op, "IDLE → NO_OPERATOR")
}
