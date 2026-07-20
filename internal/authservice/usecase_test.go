package authservice

import (
	"context"
	"errors"
	"testing"
	"time"
)

// ucStubUserStore is an internal-package copy of handler_test.go's stubUserStore (that one lives
// in the external authservice_test package and is unreachable from here) — minimal, only
// Authenticate is exercised by the tests in this file.
type ucStubUserStore struct {
	user *User
	err  error
}

func (s *ucStubUserStore) Authenticate(_ context.Context, _, _ string) (*User, error) {
	return s.user, s.err
}
func (s *ucStubUserStore) Create(context.Context, string, string, OperatorRole) error { return nil }
func (s *ucStubUserStore) FindByID(context.Context, int) (*User, error)               { return nil, nil }
func (s *ucStubUserStore) List(context.Context) ([]User, error)                       { return nil, nil }
func (s *ucStubUserStore) Delete(context.Context, int) error                          { return nil }
func (s *ucStubUserStore) UpdateRole(context.Context, int, OperatorRole) error        { return nil }

var _ UserStore = (*ucStubUserStore)(nil)

const ucTestSecret = "uc-test-secret-32-chars-long!!!"

// ─── login ──────────────────────────────────────────────────────────────────────

func TestLogin_ValidCredentials_ReturnsToken(t *testing.T) {
	tokens := NewJWTTokenIssuer(ucTestSecret)
	store := &ucStubUserStore{user: &User{Username: "op1", Role: RoleObserver}}

	token, err := login(context.Background(), store, tokens, "op1", "pw")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	claims, err := tokens.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.Subject != "op1" || claims.Role != RoleObserver {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestLogin_AuthenticateFails_ReturnsErrInvalidCredentials(t *testing.T) {
	tokens := NewJWTTokenIssuer(ucTestSecret)
	store := &ucStubUserStore{err: errors.New("wrong password")}

	_, err := login(context.Background(), store, tokens, "op1", "wrong")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

// ─── refreshToken ───────────────────────────────────────────────────────────────

func TestRefreshToken_ValidToken_ReturnsNewTokenSameSubjectAndRole(t *testing.T) {
	tokens := NewJWTTokenIssuer(ucTestSecret)
	original, err := tokens.IssueToken("op1", RoleAdmin, time.Hour)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}

	refreshed, err := refreshToken(tokens, original)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	claims, err := tokens.ParseToken(refreshed)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.Subject != "op1" || claims.Role != RoleAdmin {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestRefreshToken_InvalidToken_ReturnsErrInvalidToken(t *testing.T) {
	tokens := NewJWTTokenIssuer(ucTestSecret)

	_, err := refreshToken(tokens, "not-a-jwt")
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

// ─── handoverToken ──────────────────────────────────────────────────────────────

func TestHandoverToken_ValidCurrentTokenAndTarget_IssuesActiveOperatorToken(t *testing.T) {
	tokens := NewJWTTokenIssuer(ucTestSecret)
	current, err := tokens.IssueToken("op1", RoleActiveOperator, time.Hour)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}

	token, err := handoverToken(tokens, current, "op2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	claims, err := tokens.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.Subject != "op2" || claims.Role != RoleActiveOperator {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestHandoverToken_NoCurrentToken_StillIssuesForTarget(t *testing.T) {
	tokens := NewJWTTokenIssuer(ucTestSecret)

	token, err := handoverToken(tokens, "", "op2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token == "" {
		t.Fatal("expected a token")
	}
}

func TestHandoverToken_InvalidCurrentToken_ReturnsErrHandoverUnauthorized(t *testing.T) {
	tokens := NewJWTTokenIssuer(ucTestSecret)

	_, err := handoverToken(tokens, "garbage", "op2")
	if !errors.Is(err, ErrHandoverUnauthorized) {
		t.Fatalf("expected ErrHandoverUnauthorized, got %v", err)
	}
}

func TestHandoverToken_MissingTargetID_ReturnsErrTargetIDRequired(t *testing.T) {
	tokens := NewJWTTokenIssuer(ucTestSecret)

	_, err := handoverToken(tokens, "", "")
	if !errors.Is(err, ErrTargetIDRequired) {
		t.Fatalf("expected ErrTargetIDRequired, got %v", err)
	}
}

// ─── canModifyUser ──────────────────────────────────────────────────────────────

func TestCanModifyUser_DifferentUser_ReturnsTrue(t *testing.T) {
	if !canModifyUser("admin", User{Username: "other"}) {
		t.Fatal("expected true for a different user")
	}
}

func TestCanModifyUser_SameUser_ReturnsFalse(t *testing.T) {
	if canModifyUser("admin", User{Username: "admin"}) {
		t.Fatal("expected false for caller's own account")
	}
}
