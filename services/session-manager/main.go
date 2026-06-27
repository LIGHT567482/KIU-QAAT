package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/qaat/session-manager/internal/auth"
	"github.com/qaat/session-manager/internal/handlers"
)

func main() {
	logger := newLogger(env("LOG_LEVEL", "info"))

	// ─── Database ────────────────────────────────────────────────────────────
	pool, err := pgxpool.New(context.Background(), mustEnv("DB_URL"))
	if err != nil {
		logger.Error("postgres connect failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		logger.Error("postgres ping failed", "error", err)
		os.Exit(1)
	}
	logger.Info("postgres connected")

	// ─── Redis ───────────────────────────────────────────────────────────────
	opts, err := redis.ParseURL(mustEnv("REDIS_URL"))
	if err != nil {
		logger.Error("redis URL parse failed", "error", err)
		os.Exit(1)
	}
	rdb := redis.NewClient(opts)
	defer rdb.Close()

	if _, err := rdb.Ping(context.Background()).Result(); err != nil {
		logger.Error("redis ping failed", "error", err)
		os.Exit(1)
	}
	logger.Info("redis connected")

	// Public base URL used to build warden delegation links.
	baseURL := env("WARDEN_BASE_URL", "https://qaat.platform")

	// JWT verification — session-manager authenticates requests independently of
	// the gateway and derives the tenant from the verified token, never from a
	// client-supplied header (update.md H1).
	pubKey, err := auth.LoadPublicKey(env("RSA_PUBLIC_KEY_PATH", "keys/auth_public.pem"))
	if err != nil {
		logger.Error("load RSA public key failed", "error", err)
		os.Exit(1)
	}
	jwtVerify := auth.Middleware(pubKey, env("JWT_ISSUER", "qaat-auth"), env("JWT_AUDIENCE", "qaat-api"), rdb)

	// ─── Router ──────────────────────────────────────────────────────────────
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","service":"session-manager"}`)
	})

	// JWT-protected, tenant-scoped endpoints (mint warden links / clearance tokens).
	mux.Handle("/api/v1/sessions/warden-link", jwtVerify(post(handlers.WardenLink(pool, rdb, baseURL))))
	mux.Handle("/api/v1/eligibility/clearance-token", jwtVerify(post(handlers.ClearanceToken(pool))))
	// warden-validate authenticates by the one-time delegation token + GPS held in
	// Redis (a capability token), so it is intentionally not behind the JWT gate.
	mux.HandleFunc("/api/v1/sessions/warden-validate", post(handlers.WardenValidate(rdb)))

	// ─── Server ──────────────────────────────────────────────────────────────
	port := env("PORT", "8082")
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Info("session-manager listening", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// ─── Graceful shutdown ───────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down session-manager")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown error", "error", err)
	}
}

// post wraps a handler so it only responds to POST; anything else gets 405.
func post(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusMethodNotAllowed)
			fmt.Fprintf(w, `{"error":"METHOD_NOT_ALLOWED","message":"use POST"}`)
			return
		}
		h(w, r)
	}
}

func newLogger(level string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}))
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
		slog.Error("missing required env var", "key", key)
		os.Exit(1)
	}
	return v
}
