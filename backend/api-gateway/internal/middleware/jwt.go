package middleware

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
)

type contextKey string

const (
	CtxUserID   contextKey = "user_id"
	CtxTenantID contextKey = "tenant_id"
	CtxRole     contextKey = "role"
	CtxJTI      contextKey = "jti"
)

type Claims struct {
	jwt.RegisteredClaims
	Role     string `json:"role"`
	TenantID string `json:"tenant_id"`
}

// JWTAuth validates the Bearer token, checks the jti blacklist, and injects
// user_id, tenant_id, role, and jti into the request context.
func JWTAuth(publicKey *rsa.PublicKey, issuer, audience string, rdb *redis.Client) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenStr := extractBearer(r)
			if tokenStr == "" {
				writeUnauthorized(w, "MISSING_TOKEN", "Authorization: Bearer <token> header is required")
				return
			}

			claims, err := parseToken(tokenStr, publicKey, issuer, audience)
			if err != nil {
				writeUnauthorized(w, "TOKEN_INVALID", "token is invalid or expired")
				return
			}

			// jti blacklist check (logout / rotation).
			blacklisted, err := isBlacklisted(r.Context(), rdb, claims.ID)
			if err != nil || blacklisted {
				writeUnauthorized(w, "TOKEN_REVOKED", "token has been revoked")
				return
			}

			ctx := context.WithValue(r.Context(), CtxUserID, claims.Subject)
			ctx = context.WithValue(ctx, CtxTenantID, claims.TenantID)
			ctx = context.WithValue(ctx, CtxRole, claims.Role)
			ctx = context.WithValue(ctx, CtxJTI, claims.ID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func parseToken(tokenStr string, pub *rsa.PublicKey, issuer, audience string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return pub, nil
	}, jwt.WithIssuer(issuer), jwt.WithAudience(audience))
	if err != nil {
		return nil, err
	}
	c, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid claims")
	}
	return c, nil
}

func isBlacklisted(ctx context.Context, rdb *redis.Client, jti string) (bool, error) {
	n, err := rdb.Exists(ctx, "jti:blacklist:"+jti).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func extractBearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(h, "Bearer ")
}

// Context helpers — used in handlers to read auth claims without type assertions.
func GetUserID(ctx context.Context) string   { v, _ := ctx.Value(CtxUserID).(string); return v }
func GetTenantID(ctx context.Context) string { v, _ := ctx.Value(CtxTenantID).(string); return v }
func GetRole(ctx context.Context) string     { v, _ := ctx.Value(CtxRole).(string); return v }

func writeUnauthorized(w http.ResponseWriter, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(map[string]string{"error": code, "message": message}) //nolint:errcheck
}
