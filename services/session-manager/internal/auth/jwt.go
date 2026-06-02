// Package auth gives session-manager its own JWT verification so it never trusts
// a client-supplied tenant header. Although the API gateway authenticates
// requests, this service must not be exploitable if it is ever reachable
// directly (SSRF, in-cluster lateral movement, a misapplied NetworkPolicy):
// it mints warden delegation links and exam clearance tokens, so an attacker who
// could set X-Tenant-ID would otherwise forge those for any tenant.
//
// Mirrors services/sync-receiver/internal/auth (the established pattern).
package auth

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
)

type ctxKey string

const (
	ctxTenantID ctxKey = "tenant_id"
	ctxUserID   ctxKey = "user_id"
	ctxRole     ctxKey = "role"
)

type claims struct {
	jwt.RegisteredClaims
	Role     string `json:"role"`
	TenantID string `json:"tenant_id"`
}

// LoadPublicKey reads an RSA public key (SPKI PEM) from disk.
func LoadPublicKey(path string) (*rsa.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("no PEM block in %s", path)
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("key at %s is not RSA", path)
	}
	return rsaPub, nil
}

// Middleware verifies the Bearer token (RS256, issuer, audience), rejects
// blacklisted jti, and injects the verified tenant_id/user_id/role into context.
func Middleware(pub *rsa.PublicKey, issuer, audience string, rdb *redis.Client) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenStr := bearer(r)
			if tokenStr == "" {
				unauthorized(w, "MISSING_TOKEN", "Authorization: Bearer <token> required")
				return
			}

			c, err := parse(tokenStr, pub, issuer, audience)
			if err != nil {
				unauthorized(w, "TOKEN_INVALID", "token is invalid or expired")
				return
			}

			if n, err := rdb.Exists(r.Context(), "jti:blacklist:"+c.ID).Result(); err != nil || n > 0 {
				unauthorized(w, "TOKEN_REVOKED", "token has been revoked")
				return
			}

			ctx := context.WithValue(r.Context(), ctxTenantID, c.TenantID)
			ctx = context.WithValue(ctx, ctxUserID, c.Subject)
			ctx = context.WithValue(ctx, ctxRole, c.Role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func parse(tokenStr string, pub *rsa.PublicKey, issuer, audience string) (*claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return pub, nil
	}, jwt.WithIssuer(issuer), jwt.WithAudience(audience))
	if err != nil {
		return nil, err
	}
	c, ok := token.Claims.(*claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid claims")
	}
	return c, nil
}

// TenantID returns the verified tenant from the request context, or "".
func TenantID(ctx context.Context) string { v, _ := ctx.Value(ctxTenantID).(string); return v }

// UserID returns the verified subject from the request context, or "".
func UserID(ctx context.Context) string { v, _ := ctx.Value(ctxUserID).(string); return v }

func bearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(h, "Bearer ")
}

func unauthorized(w http.ResponseWriter, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(map[string]string{"error": code, "message": msg}) //nolint:errcheck
}
