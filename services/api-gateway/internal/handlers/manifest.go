package handlers

// Daily Manifest handler — assembles and returns the encrypted manifest for
// the requesting Coordinator.
//
// Endpoint: GET /api/v1/manifest/daily
// Role:     COORDINATOR

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/qaat/api-gateway/internal/middleware"
)

type manifestSession struct {
	UnitID         string `json:"unit_id"`
	UnitName       string `json:"unit_name"`
	VenueID        string `json:"venue_id"`
	BeaconUUID     string `json:"beacon_uuid"`
	ScheduledStart string `json:"scheduled_start,omitempty"`
	ScheduledEnd   string `json:"scheduled_end,omitempty"`
}

type rosterEntry struct {
	StudentIDHash  string `json:"student_id_hash"`
	QRSerialNumber string `json:"qr_serial_number"`
}

type manifestPolicy struct {
	AttendanceThreshold  int `json:"attendance_threshold"`
	CheckinWindowMinutes int `json:"checkin_window_minutes"`
	AutoKillMinutes      int `json:"auto_kill_minutes"`
	RSSIThresholdDBM     int `json:"rssi_threshold_dbm"`
}

type dailyManifest struct {
	ManifestVersion      string                   `json:"manifest_version"`
	GeneratedAt          string                   `json:"generated_at"`
	ExpiresAt            string                   `json:"expires_at"`
	Sessions             []manifestSession        `json:"sessions"`
	Policy               manifestPolicy           `json:"policy"`
	InstitutionPublicKey string                   `json:"institution_public_key"`
	// StudentHashKey is the per-tenant secret the Coordinator uses to recompute
	// keyed student-id hashes (HMAC-SHA256). Delivered over TLS and stored only
	// inside the AES-encrypted manifest vault on the device (F-07).
	StudentHashKey string                   `json:"student_hash_key"`
	Roster         map[string][]rosterEntry `json:"roster"`
}

func ManifestDaily(pool *pgxpool.Pool, rdb *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		userID   := middleware.GetUserID(r.Context())
		today    := time.Now().UTC().Format("2006-01-02")
		cacheKey := fmt.Sprintf("manifest:%s:%s:%s", tenantID, userID, today)

		if cached, err := rdb.Get(r.Context(), cacheKey).Bytes(); err == nil {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Manifest-Source", "cache")
			w.Write(cached) //nolint:errcheck
			return
		}

		manifest, err := buildManifest(r.Context(), pool, tenantID, userID, today)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "INTERNAL_ERROR", "message": "could not build manifest",
			})
			return
		}

		raw, _ := json.Marshal(manifest)

		midnight := time.Now().UTC().Truncate(24 * time.Hour).Add(24 * time.Hour)
		rdb.Set(r.Context(), cacheKey, raw, time.Until(midnight)) //nolint:errcheck

		w.Header().Set("Content-Type", "application/json")
		w.Write(raw) //nolint:errcheck
	}
}

func buildManifest(ctx context.Context, pool *pgxpool.Pool, tenantID, coordinatorID, date string) (*dailyManifest, error) {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Release()

	if err := middleware.SetTenantConn(ctx, conn, tenantID); err != nil {
		return nil, err
	}

	var policy manifestPolicy
	var publicKeyPEM, studentHashKey string
	err = conn.QueryRow(ctx, `
		SELECT t.attendance_threshold, t.checkin_window_minutes,
		       t.auto_kill_minutes, t.rssi_threshold_dbm, t.student_hash_key,
		       COALESCE(k.rsa_public_key_pem, '')
		FROM tenants t
		LEFT JOIN tenant_rsa_keys k
		       ON k.tenant_id = t.tenant_id AND k.revoked_at IS NULL
		WHERE t.tenant_id = $1
		ORDER BY k.created_at DESC LIMIT 1`, tenantID).
		Scan(&policy.AttendanceThreshold, &policy.CheckinWindowMinutes,
			&policy.AutoKillMinutes, &policy.RSSIThresholdDBM, &studentHashKey, &publicKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("fetch tenant policy: %w", err)
	}

	rows, err := conn.Query(ctx, `
		SELECT cu.unit_id, cu.name, COALESCE(s.venue_id,''), COALESCE(b.beacon_uuid::text,'')
		FROM sessions s
		JOIN course_units cu ON cu.unit_id  = s.unit_id
		LEFT JOIN ble_beacons b ON b.venue_id = s.venue_id
		WHERE s.coordinator_id = $1 AND s.session_date = $2 AND s.tenant_id = $3`,
		coordinatorID, date, tenantID)
	if err != nil {
		return nil, fmt.Errorf("fetch sessions: %w", err)
	}
	defer rows.Close()

	var sessions []manifestSession
	var unitIDs []string
	for rows.Next() {
		var ms manifestSession
		if err := rows.Scan(&ms.UnitID, &ms.UnitName, &ms.VenueID, &ms.BeaconUUID); err != nil {
			return nil, err
		}
		sessions = append(sessions, ms)
		unitIDs = append(unitIDs, ms.UnitID)
	}
	if sessions == nil {
		sessions = []manifestSession{}
	}

	roster := make(map[string][]rosterEntry)
	for _, uid := range unitIDs {
		rRows, err := conn.Query(ctx, `
			SELECT s.student_id, COALESCE(s.qr_serial_number,'')
			FROM students_extended s
			JOIN course_units cu ON cu.course_id = s.course_id
			WHERE cu.unit_id = $1 AND s.enrollment_status = 'ACTIVE' AND s.tenant_id = $2`,
			uid, tenantID)
		if err != nil {
			continue
		}
		var entries []rosterEntry
		for rRows.Next() {
			var studentID, serial string
			if err := rRows.Scan(&studentID, &serial); err != nil {
				continue
			}
			entries = append(entries, rosterEntry{
				StudentIDHash:  hashStudentID(studentHashKey, studentID),
				QRSerialNumber: serial,
			})
		}
		rRows.Close()
		roster[uid] = entries
	}

	now := time.Now().UTC()
	idPrefix := coordinatorID
	if len(idPrefix) > 8 {
		idPrefix = idPrefix[:8]
	}
	return &dailyManifest{
		ManifestVersion:      fmt.Sprintf("%s-%s", date, idPrefix),
		GeneratedAt:          now.Format(time.RFC3339),
		ExpiresAt:            now.Truncate(24*time.Hour).Add(24*time.Hour).Format(time.RFC3339),
		Sessions:             sessions,
		Policy:               policy,
		InstitutionPublicKey: publicKeyPEM,
		StudentHashKey:       studentHashKey,
		Roster:               roster,
	}, nil
}

// hashStudentID returns HMAC-SHA256(student_hash_key, student_id) as hex. Keying
// the hash with a per-tenant secret prevents reversing a low-entropy registration
// number by brute force or rainbow table (F-07). The Coordinator PWA computes the
// same value from the key delivered in the manifest.
func hashStudentID(key, id string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(id))
	return hex.EncodeToString(mac.Sum(nil))
}
