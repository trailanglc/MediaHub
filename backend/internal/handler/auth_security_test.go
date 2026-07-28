package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/anhtuanlc/mediahub/internal/auth"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func TestSecurity_UnauthenticatedProtectedRoutes(t *testing.T) {
	env := newSecurityEnv(t)
	paths := []struct {
		method, path string
	}{
		{http.MethodGet, "/api/auth/me"},
		{http.MethodGet, "/api/members"},
		{http.MethodGet, "/api/system/health"},
		{http.MethodGet, "/api/settings"},
	}
	for _, p := range paths {
		rec := env.doJSON(p.method, p.path, nil, nil)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s: want 401, got %d", p.method, p.path, rec.Code)
		}
	}
}

func TestSecurity_InvalidAndTamperedJWT(t *testing.T) {
	env := newSecurityEnv(t)

	rec := env.withBearer("not-a-jwt", http.MethodGet, "/api/auth/me", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("garbage token: want 401, got %d", rec.Code)
	}

	wrongIssuer := auth.NewTokenIssuer("wrong-secret", 15*time.Minute)
	tok, _, _, err := wrongIssuer.IssueAccess(uuid.New(), "owner")
	if err != nil {
		t.Fatal(err)
	}
	rec = env.withBearer(tok, http.MethodGet, "/api/auth/me", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong secret: want 401, got %d", rec.Code)
	}

	ownerEmail, ownerPass := env.ownerCredentials()
	ownerCookie := env.login(ownerEmail, ownerPass)

	viewerEmail := "sec-viewer-jwt-" + uuid.New().String()[:8] + "@example.com"
	createRec := env.withAccessCookie(ownerCookie, http.MethodPost, "/api/members", map[string]string{
		"email":    viewerEmail,
		"password": "TestPass123!",
		"role":     "viewer",
	})
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create viewer: %d %s", createRec.Code, createRec.Body.String())
	}
	viewerCookie := env.login(viewerEmail, "TestPass123!")

	parsed, _ := jwt.ParseWithClaims(viewerCookie.Value, &auth.AccessClaims{}, func(tok *jwt.Token) (interface{}, error) {
		return []byte("test-jwt-secret-for-security-suite"), nil
	})
	claims := parsed.Claims.(*auth.AccessClaims)
	claims.Role = "owner"
	tampered, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("test-jwt-secret-for-security-suite"))
	if err != nil {
		t.Fatal(err)
	}
	rec = env.withBearer(tampered, http.MethodGet, "/api/members", nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("tampered role claim should not grant owner API: want 403, got %d", rec.Code)
	}
}

func TestSecurity_RevokedAccessAfterLogout(t *testing.T) {
	env := newSecurityEnv(t)
	email, pass := env.ownerCredentials()
	loginRec := env.doJSON(http.MethodPost, "/api/auth/login", map[string]string{
		"email": email, "password": pass,
	}, nil)
	access := env.login(email, pass)
	refresh := env.refreshCookie(loginRec)

	req := httptestNewRequestWithCookies(http.MethodPost, "/api/auth/logout", nil, access, refresh)
	logoutRec := env.do(req)
	if logoutRec.Code != http.StatusNoContent {
		t.Fatalf("logout: %d", logoutRec.Code)
	}

	rec := env.withAccessCookie(access, http.MethodGet, "/api/auth/me", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("after logout access token: want 401, got %d", rec.Code)
	}
}

func TestSecurity_RoleBasedAccess(t *testing.T) {
	env := newSecurityEnv(t)
	ownerEmail, ownerPass := env.ownerCredentials()
	ownerCookie := env.login(ownerEmail, ownerPass)

	viewerEmail := "sec-viewer-rbac-" + uuid.New().String()[:8] + "@example.com"
	createRec := env.withAccessCookie(ownerCookie, http.MethodPost, "/api/members", map[string]string{
		"email": viewerEmail, "password": "TestPass123!", "role": "viewer",
	})
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create viewer: %d %s", createRec.Code, createRec.Body.String())
	}
	viewerCookie := env.login(viewerEmail, "TestPass123!")

	rec := env.withAccessCookie(viewerCookie, http.MethodGet, "/api/members", nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("viewer list members: want 403, got %d", rec.Code)
	}

	rec = env.withAccessCookie(viewerCookie, http.MethodGet, "/api/settings", nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("viewer settings: want 403, got %d", rec.Code)
	}

	rec = env.withAccessCookie(viewerCookie, http.MethodGet, "/api/system/health", nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("viewer system health: want 403, got %d", rec.Code)
	}

	rec = env.withAccessCookie(ownerCookie, http.MethodGet, "/api/members", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("owner list members: want 200, got %d", rec.Code)
	}
}

func TestSecurity_PrivilegeEscalationBlocked(t *testing.T) {
	env := newSecurityEnv(t)
	ownerEmail, ownerPass := env.ownerCredentials()
	ownerCookie := env.login(ownerEmail, ownerPass)

	escalateCreate := env.withAccessCookie(ownerCookie, http.MethodPost, "/api/members", map[string]string{
		"email":    "sec-owner-fake-" + uuid.New().String()[:8] + "@example.com",
		"password": "TestPass123!",
		"role":     "owner",
	})
	if escalateCreate.Code != http.StatusBadRequest {
		t.Fatalf("create owner role member: want 400, got %d %s", escalateCreate.Code, escalateCreate.Body.String())
	}

	managerEmail := "sec-mgr-esc-" + uuid.New().String()[:8] + "@example.com"
	createRec := env.withAccessCookie(ownerCookie, http.MethodPost, "/api/members", map[string]string{
		"email": managerEmail, "password": "TestPass123!", "role": "manager",
	})
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create manager: %d", createRec.Code)
	}

	var managerPublicID string
	listRec := env.withAccessCookie(ownerCookie, http.MethodGet, "/api/members", nil)
	var listBody struct {
		Items []struct {
			Email    string `json:"email"`
			PublicID string `json:"public_id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &listBody); err != nil {
		t.Fatal(err)
	}
	for _, item := range listBody.Items {
		if item.Email == managerEmail {
			managerPublicID = item.PublicID
			break
		}
	}
	if managerPublicID == "" {
		t.Fatal("manager public_id not found")
	}

	managerCookie := env.login(managerEmail, "TestPass123!")
	patchRec := env.withAccessCookie(managerCookie, http.MethodPatch, "/api/members/"+managerPublicID, map[string]string{
		"role": "owner",
	})
	if patchRec.Code != http.StatusForbidden {
		t.Fatalf("manager patch to owner: want 403, got %d", patchRec.Code)
	}
}

func TestSecurity_SetupAlreadyCompleted(t *testing.T) {
	env := newSecurityEnv(t)
	rec := env.doJSON(http.MethodPost, "/api/setup/owner", map[string]string{
		"email":    "another-owner-" + uuid.New().String()[:8] + "@example.com",
		"password": "TestPass123!",
	}, nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate setup: want 409, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestSecurity_RefreshTokenReuseRevokesFamily(t *testing.T) {
	env := newSecurityEnv(t)
	email, pass := env.ownerCredentials()
	loginRec := env.doJSON(http.MethodPost, "/api/auth/login", map[string]string{
		"email": email, "password": pass,
	}, nil)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login: %d", loginRec.Code)
	}
	oldRefresh := env.refreshCookie(loginRec)

	refreshReq := httptestNewRequestWithCookies(http.MethodPost, "/api/auth/refresh", nil, nil, oldRefresh)
	firstRefresh := env.do(refreshReq)
	if firstRefresh.Code != http.StatusOK {
		t.Fatalf("first refresh: %d %s", firstRefresh.Code, firstRefresh.Body.String())
	}

	reuseReq := httptestNewRequestWithCookies(http.MethodPost, "/api/auth/refresh", nil, nil, oldRefresh)
	reuseRec := env.do(reuseReq)
	if reuseRec.Code != http.StatusUnauthorized {
		t.Fatalf("reuse old refresh: want 401, got %d", reuseRec.Code)
	}
}

func TestSecurity_LoginRateLimit(t *testing.T) {
	env := newSecurityEnv(t)
	email := "ratelimit-" + uuid.New().String()[:8] + "@example.com"
	for i := 0; i < 6; i++ {
		rec := env.doJSON(http.MethodPost, "/api/auth/login", map[string]string{
			"email": email, "password": "wrong-password",
		}, nil)
		if i < 5 && rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: want 401, got %d", i+1, rec.Code)
		}
		if i >= 5 && rec.Code != http.StatusTooManyRequests {
			t.Fatalf("attempt %d: want 429, got %d", i+1, rec.Code)
		}
	}
}

func TestSecurity_CORSReflectsOriginInDevelopment(t *testing.T) {
	env := newSecurityEnv(t)
	req := httptest.NewRequest(http.MethodOptions, "/api/auth/login", nil)
	req.Header.Set("Origin", "http://192.168.8.100:3000")
	req.Header.Set("Access-Control-Request-Method", "POST")
	rec := env.do(req)
	if rec.Header().Get("Access-Control-Allow-Origin") != "http://192.168.8.100:3000" {
		t.Fatalf("CORS allow-origin = %q", rec.Header().Get("Access-Control-Allow-Origin"))
	}
	if rec.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatal("expected credentials CORS")
	}
}

func TestSecurity_SessionInvalidatedAfterRoleChange(t *testing.T) {
	env := newSecurityEnv(t)
	ownerEmail, ownerPass := env.ownerCredentials()
	ownerCookie := env.login(ownerEmail, ownerPass)

	memberEmail := "sec-role-inv-" + uuid.New().String()[:8] + "@example.com"
	createRec := env.withAccessCookie(ownerCookie, http.MethodPost, "/api/members", map[string]string{
		"email": memberEmail, "password": "TestPass123!", "role": "viewer",
	})
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create member: %d", createRec.Code)
	}

	var createBody struct {
		User struct {
			PublicID string `json:"public_id"`
		} `json:"user"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &createBody); err != nil {
		t.Fatal(err)
	}

	memberCookie := env.login(memberEmail, "TestPass123!")
	meBefore := env.withAccessCookie(memberCookie, http.MethodGet, "/api/auth/me", nil)
	if meBefore.Code != http.StatusOK {
		t.Fatalf("me before role change: %d", meBefore.Code)
	}

	patchRec := env.withAccessCookie(ownerCookie, http.MethodPatch, "/api/members/"+createBody.User.PublicID, map[string]string{
		"role": "manager",
	})
	if patchRec.Code != http.StatusOK {
		t.Fatalf("patch role: %d %s", patchRec.Code, patchRec.Body.String())
	}

	meAfter := env.withAccessCookie(memberCookie, http.MethodGet, "/api/auth/me", nil)
	if meAfter.Code != http.StatusUnauthorized {
		t.Fatalf("old access token after role change: want 401, got %d", meAfter.Code)
	}
}

func httptestNewRequestWithCookies(method, path string, body any, access, refresh *http.Cookie) *http.Request {
	var r *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, r)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if access != nil {
		req.AddCookie(access)
	}
	if refresh != nil {
		req.AddCookie(refresh)
	}
	return req
}
