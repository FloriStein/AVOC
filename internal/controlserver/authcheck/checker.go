// Package authcheck gives the Control Server a narrow, read-only view into the
// operator accounts owned by auth-service's `users` table (2026-07-16, DRIFT-K1).
//
// It queries the shared `avoc` Postgres database directly — the same pattern
// already used by internal/vehicleregistry and pkg/audit — instead of adding a
// new HTTP dependency on auth-service. A safety watchdog polling auth-service
// over HTTP would make the watchdog's own availability depend on a second
// network hop and OBSERVATION-tier failure handling (CONTEXT.MD "Auth Service
// nicht erreichbar"); reading the row control-server already has a connection
// to avoids conflating those two failure classes. This package only reads —
// all writes to `users` remain owned by internal/authservice.
package authcheck

import (
	"context"
	"database/sql"
	"fmt"
)

// Checker reports whether an operator account is still present and active.
type Checker struct {
	db *sql.DB
}

func NewChecker(db *sql.DB) *Checker {
	return &Checker{db: db}
}

// IsActiveOperator returns false (not an error) if the account was deleted —
// a hard DELETE (cmd/auth-service DeleteUser) is the only revocation path that
// exists today (2026-07-16 Grill-Me decision: account-existence check only,
// no token_version — see ADR-009 update block). Deactivation (is_active=false)
// is checked too since the column and the login-time check already exist.
func (c *Checker) IsActiveOperator(ctx context.Context, username string) (bool, error) {
	var isActive bool
	err := c.db.QueryRowContext(ctx,
		`SELECT is_active FROM users WHERE username = $1`, username,
	).Scan(&isActive)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("authcheck: query: %w", err)
	}
	return isActive, nil
}
