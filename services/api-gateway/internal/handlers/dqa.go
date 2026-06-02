package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/qaat/api-gateway/internal/middleware"
)

type thresholdConfig struct {
	AttendanceThreshold  int `json:"attendance_threshold"`
	CheckinWindowMinutes int `json:"checkin_window_minutes"`
	AutoKillMinutes      int `json:"auto_kill_minutes"`
	RSSIThresholdDBM     int `json:"rssi_threshold_dbm"`
}

// GET /api/v1/dashboard/dqa/thresholds
func GetThresholds(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())

		var cfg thresholdConfig
		err := pool.QueryRow(r.Context(), `
			SELECT attendance_threshold, checkin_window_minutes,
			       auto_kill_minutes, rssi_threshold_dbm
			FROM tenants WHERE tenant_id = $1`, tenantID).
			Scan(&cfg.AttendanceThreshold, &cfg.CheckinWindowMinutes,
				&cfg.AutoKillMinutes, &cfg.RSSIThresholdDBM)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{
				"error": "TENANT_NOT_FOUND", "message": "tenant not found",
			})
			return
		}
		writeJSON(w, http.StatusOK, cfg)
	}
}

// PUT /api/v1/dashboard/dqa/thresholds
func PutThresholds(pool *pgxpool.Pool, rdb *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())

		var cfg thresholdConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "INVALID_REQUEST", "message": "malformed body",
			})
			return
		}

		// Validate ranges.
		if cfg.AttendanceThreshold < 1 || cfg.AttendanceThreshold > 100 {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "INVALID_VALUE", "message": "attendance_threshold must be 1–100",
			})
			return
		}

		_, err := pool.Exec(r.Context(), `
			UPDATE tenants
			SET attendance_threshold   = $1,
			    checkin_window_minutes = $2,
			    auto_kill_minutes      = $3,
			    rssi_threshold_dbm     = $4
			WHERE tenant_id = $5`,
			cfg.AttendanceThreshold, cfg.CheckinWindowMinutes,
			cfg.AutoKillMinutes, cfg.RSSIThresholdDBM, tenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "INTERNAL_ERROR", "message": "update failed",
			})
			return
		}

		// Bust the Daily Manifest cache — next fetch will include new policy.
		pattern := fmt.Sprintf("manifest:%s:*", tenantID)
		keys, _ := rdb.Keys(r.Context(), pattern).Result()
		if len(keys) > 0 {
			rdb.Del(r.Context(), keys...) //nolint:errcheck
		}

		writeJSON(w, http.StatusOK, cfg)
	}
}
