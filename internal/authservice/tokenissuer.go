package authservice

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims extends jwt.RegisteredClaims with system-specific fields.
// JWT = Identity only — no Session-ID (ADR-016).
type Claims struct {
	jwt.RegisteredClaims
	Role OperatorRole `json:"role"`
}

// TokenIssuer is the JWT port for auth-service (ADR-031, HEXAUTH-01) — lets Handler depend on an
// interface instead of importing github.com/golang-jwt/jwt/v5 directly. Unlike the FleetStore
// port (HEX-01), this brings no DB-independence (JWT signing is pure computation, already
// testable without external resources) — the sole benefit is dependency inversion.
type TokenIssuer interface {
	IssueToken(subject string, role OperatorRole, ttl time.Duration) (string, error)
	ParseToken(tokenStr string) (*Claims, error)
}

// JWTTokenIssuer is the production TokenIssuer backed by golang-jwt/jwt/v5 with HS256 signing.
type JWTTokenIssuer struct {
	secret []byte
}

var _ TokenIssuer = (*JWTTokenIssuer)(nil)

func NewJWTTokenIssuer(secret string) *JWTTokenIssuer {
	return &JWTTokenIssuer{secret: []byte(secret)}
}

func (j *JWTTokenIssuer) IssueToken(subject string, role OperatorRole, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   subject,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
		Role: role,
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(j.secret)
}

// ParseToken rejects a token whose header claims a signing method other than the HMAC family
// (SEC-01) — without this check, jwt.ParseWithClaims trusts whatever alg the caller sends
// (including "none" or an asymmetric algorithm), which can let a forged token bypass the secret
// entirely (the classic JWT "alg confusion" attack). Mirrors the check already present in
// internal/fleetservice/handler.go and cmd/control-server/main.go.
func (j *JWTTokenIssuer) ParseToken(tokenStr string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return j.secret, nil
	})
	return claims, err
}
