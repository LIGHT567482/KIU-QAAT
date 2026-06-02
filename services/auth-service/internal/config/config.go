package config

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port       string
	DBURL      string
	RedisURL   string
	JWT        JWTConfig
	LogLevel   string
	Env        string
}

type JWTConfig struct {
	PrivateKey  *rsa.PrivateKey
	PublicKey   *rsa.PublicKey
	Issuer      string
	Audience    string
	TTL         time.Duration
}

func Load() (*Config, error) {
	privKey, err := loadPrivateKey(env("RSA_PRIVATE_KEY_PATH", "keys/auth_private.pem"))
	if err != nil {
		return nil, fmt.Errorf("load RSA private key: %w", err)
	}

	pubKey, err := loadPublicKey(env("RSA_PUBLIC_KEY_PATH", "keys/auth_public.pem"))
	if err != nil {
		return nil, fmt.Errorf("load RSA public key: %w", err)
	}

	ttlSecs, err := strconv.Atoi(env("JWT_TTL_SECONDS", "86400"))
	if err != nil {
		return nil, fmt.Errorf("invalid JWT_TTL_SECONDS: %w", err)
	}

	return &Config{
		Port:     env("PORT", "8081"),
		DBURL:    mustEnv("DB_URL"),
		RedisURL: mustEnv("REDIS_URL"),
		JWT: JWTConfig{
			PrivateKey: privKey,
			PublicKey:  pubKey,
			Issuer:     env("JWT_ISSUER", "qaat-auth"),
			Audience:   env("JWT_AUDIENCE", "qaat-api"),
			TTL:        time.Duration(ttlSecs) * time.Second,
		},
		LogLevel: env("LOG_LEVEL", "info"),
		Env:      env("ENVIRONMENT", "production"),
	}, nil
}

func loadPrivateKey(path string) (*rsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("no PEM block in %s", path)
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		// fallback: PKCS1
		return x509.ParsePKCS1PrivateKey(block.Bytes)
	}
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("key at %s is not RSA", path)
	}
	return rsaKey, nil
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
		return nil, fmt.Errorf("key at %s is not RSA public key", path)
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
