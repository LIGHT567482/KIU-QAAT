package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/qaat/api-gateway/internal/config"
	"github.com/qaat/api-gateway/internal/router"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config load failed", "error", err)
		os.Exit(1)
	}

	logger := newLogger(cfg.LogLevel)

	// ─── Database ─────────────────────────────────────────────────────────────
	poolCfg, err := pgxpool.ParseConfig(cfg.DBURL)
	if err != nil {
		logger.Error("postgres config parse failed", "error", err)
		os.Exit(1)
	}
	// Clear any tenant GUC before a pooled connection is handed out, so a
	// session-scoped app.current_tenant from a previous request can never leak
	// to a different tenant. Each handler re-sets it via middleware.SetTenantConn.
	poolCfg.BeforeAcquire = func(ctx context.Context, c *pgx.Conn) bool {
		_, err := c.Exec(ctx, "SELECT set_config('app.current_tenant', '', false)")
		return err == nil
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), poolCfg)
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

	// Privileged pool for the ADMIN-gated, cross-tenant platform handlers only.
	// Falls back to the data-plane pool if ADMIN_DB_URL is unset/identical.
	adminPool := pool
	if cfg.AdminDBURL != "" && cfg.AdminDBURL != cfg.DBURL {
		adminPool, err = pgxpool.New(context.Background(), cfg.AdminDBURL)
		if err != nil {
			logger.Error("admin postgres connect failed", "error", err)
			os.Exit(1)
		}
		defer adminPool.Close()
		if err := adminPool.Ping(pingCtx); err != nil {
			logger.Error("admin postgres ping failed", "error", err)
			os.Exit(1)
		}
		logger.Info("admin postgres connected")
	}

	// ─── Redis ────────────────────────────────────────────────────────────────
	opts, err := redis.ParseURL(cfg.RedisURL)
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

	// ─── Router ───────────────────────────────────────────────────────────────
	handler := router.New(
		cfg.JWT.PublicKey,
		cfg.JWT.Issuer,
		cfg.JWT.Audience,
		rdb,
		pool,
		adminPool,
		cfg.CORSOrigins,
		cfg.Env,
		router.Upstreams{
			AuthService:    cfg.AuthServiceURL,
			SessionManager: cfg.SessionManagerURL,
			SyncReceiver:   cfg.SyncReceiverURL,
		},
	)

	// ─── Server ───────────────────────────────────────────────────────────────
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Info("api-gateway listening", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// ─── Graceful shutdown ────────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down api-gateway")
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
