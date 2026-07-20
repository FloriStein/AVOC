package authservice

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// This file holds the auth-service use cases with real decision logic — credential/token
// validation combined with a token-issuance policy — extracted out of Handler so they're
// testable without HTTP (ADR-031, HEXAUTH-04). Pure pass-throughs (VehicleRegister: a single
// IssueToken call, ListUsers/CreateUser: a single UserStore call) deliberately stay inline in
// Handler — a use-case wrapper there would only add indirection.

var (
	// ErrInvalidCredentials distinguishes a failed OperatorLogin (401) from a token-issuance
	// failure (500) — both surface as an error from login, callers must not conflate them.
	ErrInvalidCredentials = errors.New("authservice: invalid credentials")
	// ErrInvalidToken distinguishes a failed RefreshToken parse (401) from a reissue failure (500).
	ErrInvalidToken = errors.New("authservice: invalid token")
	// ErrHandoverUnauthorized/ErrTargetIDRequired distinguish HandoverToken's three failure modes
	// (401/400/500) — see handoverToken.
	ErrHandoverUnauthorized = errors.New("authservice: handover unauthorized")
	ErrTargetIDRequired     = errors.New("authservice: target_id required")
)

// login authenticates username/password and issues a 24h operator token on success.
func login(ctx context.Context, userStore UserStore, tokens TokenIssuer, username, password string) (string, error) {
	user, err := userStore.Authenticate(ctx, username, password)
	if err != nil {
		return "", ErrInvalidCredentials
	}
	token, err := tokens.IssueToken(user.Username, user.Role, 24*time.Hour)
	if err != nil {
		return "", fmt.Errorf("authservice: issue token: %w", err)
	}
	return token, nil
}

// refreshToken validates tokenStr and reissues a fresh 24h token for the same subject/role.
func refreshToken(tokens TokenIssuer, tokenStr string) (string, error) {
	claims, err := tokens.ParseToken(tokenStr)
	if err != nil {
		return "", ErrInvalidToken
	}
	token, err := tokens.IssueToken(claims.Subject, claims.Role, 24*time.Hour)
	if err != nil {
		return "", fmt.Errorf("authservice: reissue token: %w", err)
	}
	return token, nil
}

// handoverToken validates the optional current token (if present) and the required targetID,
// then issues a short-lived (1h) ACTIVE_OPERATOR token for targetID — the ADR-011 operator
// handover policy. currentToken is optional because the handover-initiating request may itself
// come from an already-authenticated context validated upstream (mirrors the original Handler
// behavior unchanged).
func handoverToken(tokens TokenIssuer, currentToken, targetID string) (string, error) {
	if currentToken != "" {
		if _, err := tokens.ParseToken(currentToken); err != nil {
			return "", ErrHandoverUnauthorized
		}
	}
	if targetID == "" {
		return "", ErrTargetIDRequired
	}
	token, err := tokens.IssueToken(targetID, RoleActiveOperator, 1*time.Hour)
	if err != nil {
		return "", fmt.Errorf("authservice: issue handover token: %w", err)
	}
	return token, nil
}

// canModifyUser is the ADR-024 "cannot modify own account" guard — shared by DeleteUser and
// UpdateUserRole, which each duplicated this same check before this extraction (Rule of Three).
func canModifyUser(callerUsername string, target User) bool {
	return callerUsername != target.Username
}
