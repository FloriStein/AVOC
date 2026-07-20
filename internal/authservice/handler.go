package authservice

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// OperatorRole maps to ADR-011 OPERATOR STATE roles.
type OperatorRole string

const (
	RoleAdmin          OperatorRole = "ADMIN"
	RoleActiveOperator OperatorRole = "ACTIVE_OPERATOR"
	RoleObserver       OperatorRole = "OBSERVER"
	RoleStandby        OperatorRole = "STANDBY"
	RoleVehicle        OperatorRole = "VEHICLE"
)

type Handler struct {
	tokens    TokenIssuer
	userStore UserStore
}

func NewHandler(secret string, userStore UserStore) *Handler {
	return &Handler{tokens: NewJWTTokenIssuer(secret), userStore: userStore}
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type tokenResponse struct {
	Token string `json:"token"`
}

type validateRequest struct {
	Token string `json:"token"`
}

type validateResponse struct {
	Valid   bool         `json:"valid"`
	Subject string       `json:"subject,omitempty"`
	Role    OperatorRole `json:"role,omitempty"`
	Error   string       `json:"error,omitempty"`
}

func (h *Handler) OperatorLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	token, err := login(r.Context(), h.userStore, h.tokens, req.Username, req.Password)
	switch {
	case errors.Is(err, ErrInvalidCredentials):
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	case err != nil:
		http.Error(w, "token issuance failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, tokenResponse{Token: token})
}

func (h *Handler) VehicleRegister(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	token, err := h.tokens.IssueToken(req.Username, RoleVehicle, 168*time.Hour)
	if err != nil {
		http.Error(w, "token issuance failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, tokenResponse{Token: token})
}

func (h *Handler) ValidateToken(w http.ResponseWriter, r *http.Request) {
	var req validateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	claims, err := h.tokens.ParseToken(req.Token)
	if err != nil {
		writeJSON(w, validateResponse{Valid: false, Error: err.Error()})
		return
	}
	writeJSON(w, validateResponse{
		Valid:   true,
		Subject: claims.Subject,
		Role:    claims.Role,
	})
}

func (h *Handler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	var req validateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	token, err := refreshToken(h.tokens, req.Token)
	switch {
	case errors.Is(err, ErrInvalidToken):
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	case err != nil:
		http.Error(w, "token refresh failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, tokenResponse{Token: token})
}

func (h *Handler) HandoverToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CurrentToken string `json:"current_token"`
		TargetID     string `json:"target_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	token, err := handoverToken(h.tokens, req.CurrentToken, req.TargetID)
	switch {
	case errors.Is(err, ErrHandoverUnauthorized):
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	case errors.Is(err, ErrTargetIDRequired):
		http.Error(w, "target_id required", http.StatusBadRequest)
		return
	case err != nil:
		http.Error(w, "handover token issuance failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, tokenResponse{Token: token})
}

// ─── User Management (ADR-024) ────────────────────────────────────────────────

type userJSON struct {
	ID         int          `json:"id"`
	Username   string       `json:"username"`
	Role       OperatorRole `json:"role"`
	IsActive   bool         `json:"is_active"`
	CreatedAt  time.Time    `json:"created_at"`
	LastAuthAt *time.Time   `json:"last_auth_at,omitempty"`
}

func toUserJSON(u User) userJSON {
	return userJSON{
		ID:         u.ID,
		Username:   u.Username,
		Role:       u.Role,
		IsActive:   u.IsActive,
		CreatedAt:  u.CreatedAt,
		LastAuthAt: u.LastAuthAt,
	}
}

func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.userStore.List(r.Context())
	if err != nil {
		http.Error(w, "query failed", http.StatusInternalServerError)
		return
	}
	result := make([]userJSON, len(users))
	for i, u := range users {
		result[i] = toUserJSON(u)
	}
	writeJSON(w, result)
}

func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string       `json:"username"`
		Password string       `json:"password"`
		Role     OperatorRole `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if req.Username == "" || req.Password == "" {
		http.Error(w, "username and password required", http.StatusBadRequest)
		return
	}
	if req.Role == "" {
		req.Role = RoleObserver
	}
	if err := h.userStore.Create(r.Context(), req.Username, req.Password, req.Role); err != nil {
		http.Error(w, "create failed: "+err.Error(), http.StatusConflict)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (h *Handler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	target, err := h.userStore.FindByID(r.Context(), id)
	if err != nil || target == nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	if !canModifyUser(h.callerID(r), *target) {
		http.Error(w, "cannot delete own account", http.StatusForbidden)
		return
	}
	if err := h.userStore.Delete(r.Context(), id); err != nil {
		http.Error(w, "delete failed: "+err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) UpdateUserRole(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	var req struct {
		Role OperatorRole `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if req.Role == "" {
		http.Error(w, "role required", http.StatusBadRequest)
		return
	}
	target, err := h.userStore.FindByID(r.Context(), id)
	if err != nil || target == nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	if !canModifyUser(h.callerID(r), *target) {
		http.Error(w, "cannot change own role", http.StatusForbidden)
		return
	}
	if err := h.userStore.UpdateRole(r.Context(), id, req.Role); err != nil {
		http.Error(w, "update failed: "+err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]string{"status": "ok", "service": "auth-service"})
}

// ─── Middleware ────────────────────────────────────────────────────────────────

// RequireAdmin returns middleware that enforces a valid JWT with role=ADMIN.
func (h *Handler) RequireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, err := h.claimsFromRequest(r)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if claims.Role != RoleAdmin {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func (h *Handler) claimsFromRequest(r *http.Request) (*Claims, error) {
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return nil, fmt.Errorf("missing token")
	}
	return h.tokens.ParseToken(strings.TrimPrefix(authHeader, "Bearer "))
}

func (h *Handler) callerID(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return ""
	}
	claims, err := h.tokens.ParseToken(strings.TrimPrefix(authHeader, "Bearer "))
	if err != nil {
		return ""
	}
	return claims.Subject
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
