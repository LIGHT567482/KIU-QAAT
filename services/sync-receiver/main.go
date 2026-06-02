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

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/qaat/sync-receiver/internal/auth"
	"github.com/qaat/sync-receiver/internal/sync"
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

	// Server-side HMAC key for chunk ACK signatures.
	signKey := []byte(mustEnv("SYNC_SIGN_KEY"))

	// JWT verification — sync-receiver authenticates requests independently of
	// the gateway and derives the tenant from the verified token, never from a
	// client-supplied header.
	pubKey, err := auth.LoadPublicKey(env("RSA_PUBLIC_KEY_PATH", "keys/auth_public.pem"))
	if err != nil {
		logger.Error("load RSA public key failed", "error", err)
		os.Exit(1)
	}
	jwtIssuer := env("JWT_ISSUER", "qaat-auth")
	jwtAudience := env("JWT_AUDIENCE", "qaat-api")

	// ─── Router ──────────────────────────────────────────────────────────────
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","service":"sync-receiver"}`)
	})

	// All sync routes require a valid JWT; the tenant is taken from the verified
	// claim, not from any client-supplied header.
	r.Group(func(r chi.Router) {
		r.Use(auth.Middleware(pubKey, jwtIssuer, jwtAudience, rdb))

		r.Post("/api/v1/sync/init", sync.Init(pool))
		r.Post("/api/v1/sync/chunk/{upload_id}/{chunk_index}", sync.Chunk(pool, rdb, signKey))
		r.Get("/api/v1/sync/resume/{upload_id}", sync.Resume(pool))
		r.Post("/api/v1/sync/complete/{upload_id}", sync.Complete(pool, rdb))
	})

	// ─── Server ──────────────────────────────────────────────────────────────
	port := env("PORT", "8083")
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  30 * time.Second, // chunk uploads can be large
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Info("sync-receiver listening", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// ─── Graceful shutdown ───────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down sync-receiver")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown error", "error", err)
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
