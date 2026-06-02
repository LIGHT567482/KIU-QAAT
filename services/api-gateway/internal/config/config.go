package config

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Port              string
	DBURL             string
	AdminDBURL        string
	RedisURL          string
	AuthServiceURL    string
	QRGeneratorURL    string
	SessionManagerURL string
	SyncReceiverURL   string
	CORSOrigins       []string
	JWT               JWTConfig
	LogLevel          string
	Env               string
}

type JWTConfig struct {
	PublicKey *rsa.PublicKey
	Issuer    string
	Audience  string
}

func Load() (*Config, error) {
	pubKey, err := loadPublicKey(env("RSA_PUBLIC_KEY_PATH", "keys/auth_public.pem"))
	if err != nil {
		return nil, fmt.Errorf("load RSA public key: %w", err)
	}

	originsRaw := env("CORS_ORIGINS", "http://localhost:3000,http://localhost:3001")
	origins := strings.Split(originsRaw, ",")

	return &Config{
		Port: env("PORT", "8443"),
		// Tenant data plane: a NON-superuser, NON-owner role (qaat_app) so RLS is
		// actually enforced. See migration 009 / update.md C1.
		DBURL: mustEnv("DB_URL"),
		// Platform admin plane: a privileged connection used ONLY by the
		// ADMIN-RBAC-gated handlers (admin.go), which legitimately operate across
		// tenants (onboarding tenants, creating users, registering beacons).
		// Defaults to DB_URL if unset.
		AdminDBURL: env("ADMIN_DB_URL", env("DB_URL", "")),
		RedisURL:   mustEnv("REDIS_URL"),
		AuthServiceURL:    env("AUTH_SERVICE_URL", "http://auth-service:8081"),
		QRGeneratorURL:    env("QR_GENERATOR_URL", "http://qr-generator:3002"),
		SessionManagerURL: env("SESSION_MANAGER_URL", "http://session-manager:8082"),
		SyncReceiverURL:   env("SYNC_RECEIVER_URL", "http://sync-receiver:8083"),
		CORSOrigins:       origins,
		JWT: JWTConfig{
			PublicKey: pubKey,
			Issuer:    env("JWT_ISSUER", "qaat-auth"),
			Audience:  env("JWT_AUDIENCE", "qaat-api"),
		},
		LogLevel: env("LOG_LEVEL", "info"),
		Env:      env("ENVIRONMENT", "production"),
	}, nil
}

func loadPublicKey(path string) (*rsa.PublicKey, error) {
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
		return nil, fmt.Errorf("not an RSA public key")
	}
	return rsaPub, nil
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic(fmt.Sprintf("required environment variable %s is not set", key))
	}
	return v
}
