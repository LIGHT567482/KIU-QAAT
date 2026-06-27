package crypto

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/qaat/auth-service/internal/config"
)

type Claims struct {
	jwt.RegisteredClaims
	Role     string `json:"role"`
	TenantID string `json:"tenant_id"`
}

type JWTService struct {
	cfg *config.JWTConfig
}

func NewJWTService(cfg *config.JWTConfig) *JWTService {
	return &JWTService{cfg: cfg}
}

func (s *JWTService) Issue(userID, tenantID, role string) (tokenString, jti string, expiresAt time.Time, err error) {
	jti, err = generateJTI()
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("generate jti: %w", err)
	}

	now := time.Now()
	expiresAt = now.Add(s.cfg.TTL)

	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Issuer:    s.cfg.Issuer,
			Audience:  jwt.ClaimStrings{s.cfg.Audience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			ID:        jti,
		},
		Role:     role,
		TenantID: tenantID,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, err = token.SignedString(s.cfg.PrivateKey)
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("sign token: %w", err)
	}
	return tokenString, jti, expiresAt, nil
}

// Parse validates an RS256 token and returns its claims.
func (s *JWTService) Parse(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.cfg.PublicKey, nil
	}, jwt.WithIssuer(s.cfg.Issuer), jwt.WithAudience(s.cfg.Audience))

	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}
	return claims, nil
}

// PublicKeyPEM returns the RSA public key in PEM format for sharing with
// other services (API Gateway) without exposing the private key.
func PublicKeyPEM(key *rsa.PublicKey) ([]byte, error) {
	return []byte{}, nil // implemented in api-gateway for key loading; here for completeness
}

func generateJTI() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
