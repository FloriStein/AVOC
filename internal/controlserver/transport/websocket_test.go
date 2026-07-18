package transport

import (
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

// forgeAlgNoneToken crafts an unsigned "alg: none" JWT, the classic JWT alg-confusion payload
// (SEC-01) — validateJWT must reject it regardless of jwtSecret.
func forgeAlgNoneToken(t *testing.T) string {
	t.Helper()
	claims := jwt.MapClaims{"role": "ADMIN", "sub": "attacker"}
	tok, err := jwt.NewWithClaims(jwt.SigningMethodNone, claims).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("forge alg:none token: %v", err)
	}
	return tok
}

func TestValidateJWT_AlgNone_Rejected(t *testing.T) {
	h := &WSHandler{jwtSecret: []byte("test-secret")}

	_, err := h.validateJWT(forgeAlgNoneToken(t))

	if err == nil {
		t.Fatal("expected error for alg:none token, got nil (SEC-01 regression)")
	}
}

func TestValidateJWT_ValidHMAC_Accepted(t *testing.T) {
	secret := []byte("test-secret")
	h := &WSHandler{jwtSecret: secret}

	claims := &Claims{Role: "OPERATOR"}
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	got, err := h.validateJWT(tok)
	if err != nil {
		t.Fatalf("expected valid HMAC token to be accepted, got error: %v", err)
	}
	if got.Role != "OPERATOR" {
		t.Fatalf("expected role OPERATOR, got %q", got.Role)
	}
}
