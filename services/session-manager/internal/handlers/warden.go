package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/qaat/session-manager/internal/auth"
)

// POST /api/v1/sessions/warden-link
// Generates a one-time warden delegation link bound to GPS geofence + time window.
// Role: COORDINATOR

type wardenLinkRequest struct {
	CoordinatorID   string `json:"coordinator_id"`
	WardenStudentID string `json:"warden_student_id"`
	SessionUnitID   string `json:"session_unit_id"`
	VenueID         string `json:"venue_id"`
	SessionDate     string `json:"session_date"`
	TimeWindowStart string `json:"time_window_start"`
	TimeWindowEnd   string `json:"time_window_end"`
}

type geofence struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	RadiusM   int     `json:"radius_meters"`
}

type wardenLinkResponse struct {
	DelegationLink string   `json:"delegation_link"`
	TokenHash      string   `json:"token_hash"`
	ExpiresAt      string   `json:"expires_at"`
	Geofence       geofence `json:"geofence"`
}

func WardenLink(pool *pgxpool.Pool, rdb *redis.Client, baseURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req wardenLinkRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", "malformed body")
			return
		}
		if req.CoordinatorID == "" || req.WardenStudentID == "" ||
			req.SessionUnitID == "" || req.VenueID == "" {
			writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", "missing required fields")
			return
		}

		// venues is RLS-protected; scope the connection to the verified tenant.
		conn, err := pool.Acquire(r.Context())
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "INTERNAL_ERROR", "db unavailable")
			return
		}
		defer conn.Release()
		if err := setTenantConn(r.Context(), conn, auth.TenantID(r.Context())); err != nil {
			writeErr(w, http.StatusForbidden, "INVALID_TENANT", "missing or malformed tenant context")
			return
		}

		var lat, lng float64
		var radius int
		row := conn.QueryRow(r.Context(),
			`SELECT COALESCE(gps_latitude,0), COALESCE(gps_longitude,0), geofence_radius_meters
			 FROM venues WHERE venue_id = $1`, req.VenueID)
		if err := row.Scan(&lat, &lng, &radius); err != nil {
			writeErr(w, http.StatusNotFound, "VENUE_NOT_FOUND", "venue not found")
			return
		}

		tokenBytes := make([]byte, 32)
		if _, err := rand.Read(tokenBytes); err != nil {
			writeErr(w, http.StatusInternalServerError, "INTERNAL_ERROR", "token generation failed")
			return
		}
		token := hex.EncodeToString(tokenBytes)
		h := sha256.Sum256([]byte(token))
		tokenHash := hex.EncodeToString(h[:])

		windowEnd, err := time.Parse(time.RFC3339, req.TimeWindowEnd)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", "time_window_end must be RFC3339")
			return
		}
		ttl := time.Until(windowEnd) + 5*time.Minute
		if ttl <= 0 {
			ttl = 2*time.Hour + 35*time.Minute
		}

		meta, _ := json.Marshal(map[string]interface{}{
			"coordinator_id":    req.CoordinatorID,
			"warden_student_id": req.WardenStudentID,
			"unit_id":           req.SessionUnitID,
			"venue_id":          req.VenueID,
			"session_date":      req.SessionDate,
			"time_window_start": req.TimeWindowStart,
			"time_window_end":   req.TimeWindowEnd,
			"geofence": map[string]interface{}{
				"latitude":      lat,
				"longitude":     lng,
				"radius_meters": radius,
			},
		})
		rdb.Set(r.Context(), "warden:"+tokenHash, meta, ttl) //nolint:errcheck

		writeJSON(w, http.StatusOK, wardenLinkResponse{
			DelegationLink: fmt.Sprintf("%s/s/%s", baseURL, token),
			TokenHash:      tokenHash,
			ExpiresAt:      time.Now().Add(ttl).UTC().Format(time.RFC3339),
			Geofence:       geofence{Latitude: lat, Longitude: lng, RadiusM: radius},
		})
	}
}

// POST /api/v1/sessions/warden-validate
// Called by the Warden PWA to verify GPS position before opening the session.

type wardenValidateRequest struct {
	Token     string  `json:"token"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

func WardenValidate(rdb *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req wardenValidateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", "malformed body")
			return
		}

		h := sha256.Sum256([]byte(req.Token))
		tokenHash := hex.EncodeToString(h[:])

		raw, err := rdb.Get(r.Context(), "warden:"+tokenHash).Bytes()
		if err != nil {
			writeErr(w, http.StatusUnauthorized, "TOKEN_INVALID", "delegation token is invalid or expired")
			return
		}

		var meta map[string]interface{}
		json.Unmarshal(raw, &meta) //nolint:errcheck

		gf, _ := meta["geofence"].(map[string]interface{})
		vLat, _ := gf["latitude"].(float64)
		vLng, _ := gf["longitude"].(float64)
		radiusM, _ := gf["radius_meters"].(float64)

		dist := haversineMeters(req.Latitude, req.Longitude, vLat, vLng)
		if dist > radiusM {
			writeErr(w, http.StatusForbidden, "GEOFENCE_FAIL",
				fmt.Sprintf("device is %.0fm from venue (limit %.0fm)", dist, radiusM))
			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status":   "VALID",
			"metadata": meta,
		})
	}
}

// haversineMeters returns the great-circle distance in metres between two WGS-84 points.
func haversineMeters(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6_371_000.0
	toRad := math.Pi / 180.0
	dLat := (lat2 - lat1) * toRad
	dLon := (lon2 - lon1) * toRad
	a := math.Pow(math.Sin(dLat/2), 2) +
		math.Cos(lat1*toRad)*math.Cos(lat2*toRad)*math.Pow(math.Sin(dLon/2), 2)
	return R * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

func writeErr(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": code, "message": msg}) //nolint:errcheck
}

func writeJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body) //nolint:errcheck
}
