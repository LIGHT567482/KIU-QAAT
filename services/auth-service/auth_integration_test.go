package main_test

// Integration tests — require real PostgreSQL + Redis.
// Skipped automatically when DB_URL env var is not set.
// CI runs these against the service containers declared in .github/workflows/ci.yml.

import (
	"bytes"
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"

	"github.com/qaat/auth-service/internal/config"
	"github.com/qaat/auth-service/internal/crypto"
	"github.com/qaat/auth-service/internal/handlers"
	"github.com/qaat/auth-service/internal/store"
)

const (
	testTenantA = "a0000000-0000-0000-0000-000000000001"
	testTenantB = "b0000000-0000-0000-0000-000000000002"
	testPW      = "TestPassword1!"
)

func skipIfNoIntegration(t *testing.T) {
	t.Helper()
	if os.Getenv("DB_URL") == "" {
		t.Skip("skipping: DB_URL not set")
	}
}

func setupTestServer(t *testing.T) (*httptest.Server, *pgxpool.Pool, *redis.Client) {
	t.Helper()
	skipIfNoIntegration(t)

	pool, err := pgxpool.New(context.Background(), os.Getenv("DB_URL"))
	if err != nil || pool.Ping(context.Background()) != nil {
		t.Fatalf("postgres unavailable: %v", err)
	}

	opts, _ := redis.ParseURL(os.Getenv("REDIS_URL"))
	rdb := redis.NewClient(opts)
	if _, err := rdb.Ping(context.Background()).Result(); err != nil {
		t.Fatalf("redis unavailable: %v", err)
	}

	privPath := os.Getenv("RSA_PRIVATE_KEY_PATH")
	pubPath  := os.Getenv("RSA_PUBLIC_KEY_PATH")
	if privPath == "" {
		t.Skip("RSA_PRIVATE_KEY_PATH not set")
	}
	privKey, err := loadPrivKey(privPath)
	if err != nil {
		t.Fatalf("load private key: %v", err)
	}
	pubKey, err := loadPubKey(pubPath)
	if err != nil {
		t.Fatalf("load public key: %v", err)
	}

	jwtCfg := &config.JWTConfig{
		PrivateKey: privKey,
		PublicKey:  pubKey,
		Issuer:     "qaat-auth-test",
		Audience:   "qaat-api-test",
		TTL:        time.Hour,
	}

	userStore  := store.NewUserStore(pool)
	tokenStore := store.NewTokenStore(rdb)
	jwtSvc     := crypto.NewJWTService(jwtCfg)
	h          := handlers.NewAuthHandler(userStore, tokenStore, jwtSvc, jwtCfg.TTL, true, "")

	r := chi.NewRouter()
	r.Use(chimiddleware.Recoverer)
	r.Post("/api/v1/auth/login",   h.Login)
	r.Post("/api/v1/auth/refresh", h.Refresh)
	r.Post("/api/v1/auth/logout",  h.Logout)

	srv := httptest.NewServer(r)
	t.Cleanup(func() { srv.Close(); pool.Close(); rdb.Close() })
	return srv, pool, rdb
}

func insertUser(t *testing.T, pool *pgxpool.Pool, email, tenantID string) {
	t.Helper()
	hash, _ := bcrypt.GenerateFromPassword([]byte(testPW), 12)
	_, err := pool.Exec(context.Background(), `
		INSERT INTO users (email, password_hash, role, full_name, tenant_id, is_active)
		VALUES ($1, $2, 'COORDINATOR', 'Test User', $3, true)
		ON CONFLICT (tenant_id, email) DO NOTHING`,
		email, string(hash), tenantID)
	if err != nil {
		t.Fatalf("insert test user: %v", err)
	}
}

func doLogin(t *testing.T, srv *httptest.Server, email, tenantID string) *http.Response {
	t.Helper()
	body, _ := json.Marshal(map[string]string{
		"email": email, "password": testPW, "tenant_id": tenantID,
	})
	resp, err := http.Post(srv.URL+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("login request: %v", err)
	}
	return resp
}

// ─── Tests ────────────────────────────────────────────────────────────────────

func TestLogin_ValidCredentials(t *testing.T) {
	srv, pool, _ := setupTestServer(t)
	insertUser(t, pool, "t_valid@alpha.edu", testTenantA)

	resp := doLogin(t, srv, "t_valid@alpha.edu", testTenantA)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(b))
	}
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result) //nolint:errcheck
	if result["access_token"] == nil {
		t.Error("expected access_token in response")
	}
	if result["role"] != "COORDINATOR" {
		t.Errorf("expected role COORDINATOR, got %v", result["role"])
	}
}

func TestLogin_WrongPassword_Returns401(t *testing.T) {
	srv, pool, _ := setupTestServer(t)
	insertUser(t, pool, "t_badpw@alpha.edu", testTenantA)

	body, _ := json.Marshal(map[string]string{
		"email": "t_badpw@alpha.edu", "password": "WrongPW123!", "tenant_id": testTenantA,
	})
	resp, _ := http.Post(srv.URL+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestLogin_CrossTenantIsBlocked(t *testing.T) {
	// User belongs to Tenant A — logging in with Tenant B's ID must return 401.
	srv, pool, _ := setupTestServer(t)
	insertUser(t, pool, "t_cross@alpha.edu", testTenantA)

	body, _ := json.Marshal(map[string]string{
		"email": "t_cross@alpha.edu", "password": testPW, "tenant_id": testTenantB,
	})
	resp, _ := http.Post(srv.URL+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("cross-tenant login must be blocked (401), got %d", resp.StatusCode)
	}
}

func TestRefresh_BlacklistsOldJTI(t *testing.T) {
	srv, pool, rdb := setupTestServer(t)
	insertUser(t, pool, "t_refresh@alpha.edu", testTenantA)

	loginResp := doLogin(t, srv, "t_refresh@alpha.edu", testTenantA)
	var loginData map[string]interface{}
	json.NewDecoder(loginResp.Body).Decode(&loginData) //nolint:errcheck
	loginResp.Body.Close()

	oldToken := loginData["access_token"].(string)
	oldJTI   := loginData["jti"].(string)

	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/auth/refresh", nil)
	req.Header.Set("Authorization", "Bearer "+oldToken)
	refreshResp, _ := http.DefaultClient.Do(req)
	refreshResp.Body.Close()

	if refreshResp.StatusCode != http.StatusOK {
		t.Fatalf("refresh failed: %d", refreshResp.StatusCode)
	}

	n, _ := rdb.Exists(context.Background(), "jti:blacklist:"+oldJTI).Result()
	if n == 0 {
		t.Error("old JTI was not blacklisted after refresh — replay attack possible")
	}
}

func TestRefresh_RevokedToken_Returns401(t *testing.T) {
	// After logout, refresh must be rejected.
	srv, pool, _ := setupTestServer(t)
	insertUser(t, pool, "t_revoked@alpha.edu", testTenantA)

	loginResp := doLogin(t, srv, "t_revoked@alpha.edu", testTenantA)
	var loginData map[string]interface{}
	json.NewDecoder(loginResp.Body).Decode(&loginData) //nolint:errcheck
	loginResp.Body.Close()
	token := loginData["access_token"].(string)

	// Logout first.
	logoutReq, _ := http.NewRequest("POST", srv.URL+"/api/v1/auth/logout", nil)
	logoutReq.Header.Set("Authorization", "Bearer "+token)
	logoutResp, _ := http.DefaultClient.Do(logoutReq)
	logoutResp.Body.Close()

	// Now try to refresh with the revoked token.
	refreshReq, _ := http.NewRequest("POST", srv.URL+"/api/v1/auth/refresh", nil)
	refreshReq.Header.Set("Authorization", "Bearer "+token)
	refreshResp, _ := http.DefaultClient.Do(refreshReq)
	refreshResp.Body.Close()

	if refreshResp.StatusCode != http.StatusUnauthorized {
		t.Errorf("revoked token refresh must return 401, got %d", refreshResp.StatusCode)
	}
}

func TestLogin_AccountLockoutAfter5Failures(t *testing.T) {
	srv, pool, _ := setupTestServer(t)
	insertUser(t, pool, "t_lockout@alpha.edu", testTenantA)

	wrongBody, _ := json.Marshal(map[string]string{
		"email": "t_lockout@alpha.edu", "password": "WrongPW!", "tenant_id": testTenantA,
	})
	for i := 0; i < 5; i++ {
		resp, _ := http.Post(srv.URL+"/api/v1/auth/login", "application/json", bytes.NewReader(wrongBody))
		resp.Body.Close()
	}

	// Correct password now also locked.
	correctBody, _ := json.Marshal(map[string]string{
		"email": "t_lockout@alpha.edu", "password": testPW, "tenant_id": testTenantA,
	})
	resp, _ := http.Post(srv.URL+"/api/v1/auth/login", "application/json", bytes.NewReader(correctBody))
	resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("expected 429 after lockout, got %d", resp.StatusCode)
	}
}

// ─── Key loading helpers ──────────────────────────────────────────────────────

func loadPrivKey(path string) (*rsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return x509.ParsePKCS1PrivateKey(block.Bytes)
	}
	return key.(*rsa.PrivateKey), nil
}

func loadPubKey(path string) (*rsa.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	return pub.(*rsa.PublicKey), nil
}
