package authservice_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"avoc/internal/authservice"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Stub UserStore ───────────────────────────────────────────────────────────

type stubUserStore struct {
	users  []*authservice.User
	hashes map[string]string // username → cleartext password
	nextID int
}

func newStubStore(users ...authservice.User) *stubUserStore {
	s := &stubUserStore{hashes: make(map[string]string), nextID: 1}
	for i := range users {
		u := users[i]
		if u.ID == 0 {
			u.ID = s.nextID
			s.nextID++
		} else if u.ID >= s.nextID {
			s.nextID = u.ID + 1
		}
		s.users = append(s.users, &u)
	}
	return s
}

func (s *stubUserStore) findByUsername(username string) *authservice.User {
	for _, u := range s.users {
		if u.Username == username {
			return u
		}
	}
	return nil
}

func (s *stubUserStore) Create(_ context.Context, username, password string, role authservice.OperatorRole) error {
	if s.findByUsername(username) != nil {
		return fmt.Errorf("user already exists")
	}
	s.users = append(s.users, &authservice.User{ID: s.nextID, Username: username, Role: role, IsActive: true})
	s.hashes[username] = password
	s.nextID++
	return nil
}

func (s *stubUserStore) Authenticate(_ context.Context, username, password string) (*authservice.User, error) {
	u := s.findByUsername(username)
	if u == nil {
		return nil, fmt.Errorf("invalid credentials")
	}
	if !u.IsActive {
		return nil, fmt.Errorf("account deactivated")
	}
	if password != s.hashes[username] {
		return nil, fmt.Errorf("invalid credentials")
	}
	return u, nil
}

func (s *stubUserStore) FindByID(_ context.Context, id int) (*authservice.User, error) {
	for _, u := range s.users {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, nil
}

func (s *stubUserStore) List(_ context.Context) ([]authservice.User, error) {
	out := make([]authservice.User, len(s.users))
	for i, u := range s.users {
		out[i] = *u
	}
	return out, nil
}

func (s *stubUserStore) Delete(_ context.Context, id int) error {
	for i, u := range s.users {
		if u.ID == id {
			s.users = append(s.users[:i], s.users[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("user not found")
}

func (s *stubUserStore) UpdateRole(_ context.Context, id int, role authservice.OperatorRole) error {
	for _, u := range s.users {
		if u.ID == id {
			u.Role = role
			return nil
		}
	}
	return fmt.Errorf("user not found")
}

func (s *stubUserStore) SeedAdmin(_ context.Context, username, password string) error {
	if u := s.findByUsername(username); u != nil {
		u.Role = authservice.RoleAdmin
		return nil
	}
	s.users = append(s.users, &authservice.User{ID: s.nextID, Username: username, Role: authservice.RoleAdmin, IsActive: true})
	s.hashes[username] = password
	s.nextID++
	return nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func makeHandler(store authservice.UserStore) *authservice.Handler {
	return authservice.NewHandler("test-secret-32-chars-for-testing!", store)
}

func postJSON(t *testing.T, handler http.HandlerFunc, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler(rr, req)
	return rr
}

func getWithToken(t *testing.T, handler http.HandlerFunc, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rr := httptest.NewRecorder()
	handler(rr, req)
	return rr
}

func loginAs(t *testing.T, h *authservice.Handler, username, password string) string {
	t.Helper()
	rr := postJSON(t, h.OperatorLogin, map[string]string{"username": username, "password": password})
	require.Equal(t, 200, rr.Code, "login should succeed for %s", username)
	var resp map[string]string
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	return resp["token"]
}

// ─── Login Edge Cases ─────────────────────────────────────────────────────────

func TestLogin_ValidCredentials_ReturnsJWT(t *testing.T) {
	store := newStubStore(authservice.User{Username: "op1", Role: authservice.RoleObserver, IsActive: true})
	store.hashes["op1"] = "correct_pass"
	h := makeHandler(store)

	rr := postJSON(t, h.OperatorLogin, map[string]string{"username": "op1", "password": "correct_pass"})

	assert.Equal(t, 200, rr.Code)
	var resp map[string]string
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.NotEmpty(t, resp["token"])
}

func TestLogin_WrongPassword_Returns401(t *testing.T) {
	store := newStubStore(authservice.User{Username: "op1", Role: authservice.RoleObserver, IsActive: true})
	store.hashes["op1"] = "correct_pass"
	h := makeHandler(store)

	rr := postJSON(t, h.OperatorLogin, map[string]string{"username": "op1", "password": "wrong_pass"})

	assert.Equal(t, 401, rr.Code)
}

func TestLogin_UnknownUser_Returns401(t *testing.T) {
	h := makeHandler(newStubStore())

	rr := postJSON(t, h.OperatorLogin, map[string]string{"username": "nobody", "password": "pass"})

	assert.Equal(t, 401, rr.Code)
}

func TestLogin_DeactivatedUser_Returns401(t *testing.T) {
	store := newStubStore(authservice.User{Username: "inactive", Role: authservice.RoleObserver, IsActive: false})
	store.hashes["inactive"] = "pass"
	h := makeHandler(store)

	rr := postJSON(t, h.OperatorLogin, map[string]string{"username": "inactive", "password": "pass"})

	assert.Equal(t, 401, rr.Code)
}

func TestLogin_MalformedJSON_Returns400(t *testing.T) {
	h := makeHandler(newStubStore())
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString("{invalid json}"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.OperatorLogin(rr, req)

	assert.Equal(t, 400, rr.Code)
}

func TestLogin_RolePreservedInToken(t *testing.T) {
	store := newStubStore(authservice.User{Username: "admin", Role: authservice.RoleAdmin, IsActive: true})
	store.hashes["admin"] = "admin_pass"
	h := makeHandler(store)

	token := loginAs(t, h, "admin", "admin_pass")

	rr := postJSON(t, h.ValidateToken, map[string]string{"token": token})
	assert.Equal(t, 200, rr.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, true, resp["valid"])
	assert.Equal(t, "ADMIN", resp["role"])
}

// ─── RequireAdmin Middleware Edge Cases ───────────────────────────────────────

func TestRequireAdmin_NoToken_Returns401(t *testing.T) {
	h := makeHandler(newStubStore())
	protected := h.RequireAdmin(h.ListUsers)

	rr := getWithToken(t, protected, "")

	assert.Equal(t, 401, rr.Code)
}

func TestRequireAdmin_InvalidToken_Returns401(t *testing.T) {
	h := makeHandler(newStubStore())
	protected := h.RequireAdmin(h.ListUsers)

	rr := getWithToken(t, protected, "not.a.jwt")

	assert.Equal(t, 401, rr.Code)
}

func TestRequireAdmin_NonAdminToken_Returns403(t *testing.T) {
	store := newStubStore(authservice.User{Username: "observer", Role: authservice.RoleObserver, IsActive: true})
	store.hashes["observer"] = "pass"
	h := makeHandler(store)
	protected := h.RequireAdmin(h.ListUsers)

	token := loginAs(t, h, "observer", "pass")
	rr := getWithToken(t, protected, token)

	assert.Equal(t, 403, rr.Code)
}

func TestRequireAdmin_AdminToken_PassesThrough(t *testing.T) {
	store := newStubStore(authservice.User{Username: "admin", Role: authservice.RoleAdmin, IsActive: true})
	store.hashes["admin"] = "admin_pass"
	h := makeHandler(store)
	protected := h.RequireAdmin(h.ListUsers)

	token := loginAs(t, h, "admin", "admin_pass")
	rr := getWithToken(t, protected, token)

	assert.Equal(t, 200, rr.Code)
}

// ─── User CRUD Edge Cases ─────────────────────────────────────────────────────

func TestCreateUser_MissingFields_Returns400(t *testing.T) {
	h := makeHandler(newStubStore())

	rr := postJSON(t, h.CreateUser, map[string]string{"username": "newuser"}) // missing password

	assert.Equal(t, 400, rr.Code)
}

func TestCreateUser_DuplicateUsername_Returns409(t *testing.T) {
	store := newStubStore(authservice.User{Username: "existing", IsActive: true})
	store.hashes["existing"] = "pass"
	h := makeHandler(store)

	rr := postJSON(t, h.CreateUser, map[string]any{
		"username": "existing", "password": "pass", "role": "OBSERVER",
	})

	assert.Equal(t, 409, rr.Code)
}

func TestCreateUser_DefaultsToObserver_WhenRoleEmpty(t *testing.T) {
	store := newStubStore()
	h := makeHandler(store)

	rr := postJSON(t, h.CreateUser, map[string]any{
		"username": "newuser", "password": "somepass",
		// role omitted → handler defaults to OBSERVER
	})

	assert.Equal(t, 201, rr.Code)
	u := store.findByUsername("newuser")
	require.NotNil(t, u)
	assert.Equal(t, authservice.RoleObserver, u.Role)
}

func TestDeleteUser_OwnAccount_Returns403(t *testing.T) {
	// admin is seeded as ID=1 (first user in newStubStore)
	store := newStubStore(authservice.User{Username: "admin", Role: authservice.RoleAdmin, IsActive: true})
	store.hashes["admin"] = "admin_pass"
	h := makeHandler(store)
	token := loginAs(t, h, "admin", "admin_pass")

	req := httptest.NewRequest(http.MethodDelete, "/auth/users/1", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /auth/users/{id}", h.RequireAdmin(h.DeleteUser))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	assert.Equal(t, 403, rr.Code)
}

func TestDeleteUser_NonExistentUser_Returns404(t *testing.T) {
	store := newStubStore(authservice.User{Username: "admin", Role: authservice.RoleAdmin, IsActive: true})
	store.hashes["admin"] = "admin_pass"
	h := makeHandler(store)
	token := loginAs(t, h, "admin", "admin_pass")

	req := httptest.NewRequest(http.MethodDelete, "/auth/users/999", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /auth/users/{id}", h.RequireAdmin(h.DeleteUser))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	assert.Equal(t, 404, rr.Code)
}

func TestUpdateUserRole_EmptyRole_Returns400(t *testing.T) {
	h := makeHandler(newStubStore())

	req := httptest.NewRequest(http.MethodPatch, "/auth/users/1", bytes.NewBufferString(`{"role":""}`))
	req.Header.Set("Content-Type", "application/json")
	mux := http.NewServeMux()
	mux.HandleFunc("PATCH /auth/users/{id}", h.UpdateUserRole)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	assert.Equal(t, 400, rr.Code)
}

// ─── SeedAdmin Idempotency ────────────────────────────────────────────────────

func TestSeedAdmin_Idempotent(t *testing.T) {
	store := newStubStore()
	ctx := context.Background()

	require.NoError(t, store.SeedAdmin(ctx, "admin", "pass"))
	require.NoError(t, store.SeedAdmin(ctx, "admin", "pass"))

	users, _ := store.List(ctx)
	assert.Len(t, users, 1, "only one admin account must exist after two seeds")
}

// ─── Token Refresh + Validate Edge Cases ─────────────────────────────────────

func TestValidateToken_ExpiredOrWrongSecret_ReturnsFalse(t *testing.T) {
	h := makeHandler(newStubStore())
	rr := postJSON(t, h.ValidateToken, map[string]string{"token": "invalid.token.here"})

	assert.Equal(t, 200, rr.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, false, resp["valid"])
}

func TestRefreshToken_InvalidToken_Returns401(t *testing.T) {
	h := makeHandler(newStubStore())
	rr := postJSON(t, h.RefreshToken, map[string]string{"token": "garbage"})

	assert.Equal(t, 401, rr.Code)
}

func TestVehicleRegister_AnyIDSucceeds(t *testing.T) {
	h := makeHandler(newStubStore()) // vehicle register does not check DB

	rr := postJSON(t, h.VehicleRegister, map[string]string{"username": "vehicle-001"})

	assert.Equal(t, 200, rr.Code)
	var resp map[string]string
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.NotEmpty(t, resp["token"])
}
