package handlers

// The lecturer's own side of the patrol record.
//
// A QA patroller walks a corridor and ticks taught / not-taught. That tick is timestamped, filed
// against a named lecturer, and until now it was the ONLY account of the moment. A lecturer who
// was teaching in a room the patroller never reached had nothing to answer with but their word,
// offered days later against a record made at the time.
//
// So they get a record made at the time too: one button in the app, pressed while standing in the
// room, which writes down where the phone is, when, and which timetabled slot that lands in.
//
// Three endpoints, and the split matters:
//
//	GET  /api/v1/lecturer/timetable        — their whole week, so the PHONE can resolve the slot
//	                                          with no signal. This is the piece that makes the
//	                                          feature work where it has to work.
//	POST /api/v1/lecturer/presence-claims  — a batch of claims filed offline, uploaded later.
//	GET  /api/v1/dashboard/qa/presence-claims — what QA reads when a lecturer complains.
//
// This is evidence, not proof, and the code treats it that way: nothing here overturns a patrol
// tick, marks attendance, or feeds a report. It produces a second record for a human to read
// beside the first.

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qaat/api-gateway/internal/middleware"
)

// GET /api/v1/lecturer/timetable lives in lecturer_timetable.go — it grew from the phone's
// minimal week into the dashboard's full grid, and one handler serving both is what keeps the
// screen a lecturer plans from and the week their phone matches against from disagreeing.

// ─── POST /api/v1/lecturer/presence-claims ───────────────────────────────────

// presenceClaimIn is one claim as the phone filed it.
type presenceClaimIn struct {
	// Minted on the phone. This is what makes the upload idempotent — a batch retried after a
	// timeout, which is the NORMAL case on the connection these are filed on, must not file the
	// same moment twice.
	ClaimID string `json:"claim_id"`

	Latitude       *float64 `json:"latitude"`
	Longitude      *float64 `json:"longitude"`
	AccuracyMetres *float64 `json:"accuracy_metres"`
	LocationStatus string   `json:"location_status"` // OK | NO_FIX | PERMISSION_DENIED | DISABLED

	CapturedAt string `json:"captured_at"` // RFC3339, the PHONE's clock

	UnitID           string `json:"unit_id"`
	UnitName         string `json:"unit_name"`
	Room             string `json:"room"`
	DayOfWeek        int    `json:"day_of_week"`
	ScheduledTime    string `json:"scheduled_time"` // HH:MM
	SessionDate      string `json:"session_date"`   // YYYY-MM-DD
	MatchKind        string `json:"match_kind"`     // IN_SLOT | NEAR_SLOT | NEAREST | NONE
	MinutesFromStart *int   `json:"minutes_from_start"`

	Note string `json:"note"`
}

// validLocationStatus and validMatchKind mirror the CHECK constraints in migration 081. Checked
// here as well as there so a phone on an older build gets a claim FILED with a sane default,
// rather than the whole batch rejected by the database for a string it has never heard of.
func validLocationStatus(s string) string {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "OK", "NO_FIX", "PERMISSION_DENIED", "DISABLED":
		return strings.ToUpper(strings.TrimSpace(s))
	default:
		return "NO_FIX"
	}
}

func validMatchKind(s string) string {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "IN_SLOT", "NEAR_SLOT", "NEAREST", "NONE":
		return strings.ToUpper(strings.TrimSpace(s))
	default:
		return "NONE"
	}
}

// SubmitPresenceClaims ingests a batch of claims filed on a lecturer's phone.
//
// The lecturer's identity comes from the TOKEN, never from the body. A claim that could name
// somebody else would be worthless the first time anyone noticed — the point of the record is that
// only the person holding that account could have made it.
func SubmitPresenceClaims(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		userID := middleware.GetUserID(r.Context())

		var body struct {
			Claims []presenceClaimIn `json:"claims"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "malformed body"))
			return
		}

		conn, err := pool.Acquire(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "db unavailable"))
			return
		}
		defer conn.Release()
		if err := middleware.SetTenantConn(r.Context(), conn, tenantID); err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "db unavailable"))
			return
		}

		// Who is filing, resolved once from the token.
		var staffID, fullName string
		_ = conn.QueryRow(r.Context(),
			`SELECT COALESCE(staff_id,''), COALESCE(full_name,'') FROM users WHERE user_id = $1::uuid`,
			userID).Scan(&staffID, &fullName)

		deviceHash := deviceFingerprint(r)

		written := 0
		for _, c := range body.Claims {
			if strings.TrimSpace(c.ClaimID) == "" || strings.TrimSpace(c.CapturedAt) == "" {
				continue
			}
			_, execErr := conn.Exec(r.Context(), `
				INSERT INTO lecturer_presence_claims
				  (claim_id, lecturer_user_id, lecturer_staff_id, lecturer_name,
				   latitude, longitude, accuracy_metres, location_status,
				   -- The phone's clock is the record of when the lecturer stood in the room, and a
				   -- PAST timestamp is kept exactly as given: that is the whole point of filing
				   -- offline. A FUTURE one is a wrong device clock rather than evidence, so it is
				   -- clamped to now — filed under a day that has not happened, no report window
				   -- would ever surface it, and the claim would be silently useless to the person
				   -- who made it.
				   captured_at,
				   unit_id, unit_name, room, day_of_week, scheduled_time, session_date,
				   match_kind, minutes_from_start, note, device_hash)
				VALUES ($1::uuid, $2::uuid, $3, $4,
				        $5, $6, $7, $8,
				        LEAST($9::timestamptz, now()),
				        NULLIF($10,''), $11, $12, NULLIF($13::int, 0)::smallint, $14,
				        NULLIF($15,'')::date, $16, $17, $18, $19)
				ON CONFLICT (claim_id) DO NOTHING`,
				c.ClaimID, userID, staffID, fullName,
				c.Latitude, c.Longitude, c.AccuracyMetres, validLocationStatus(c.LocationStatus),
				c.CapturedAt,
				c.UnitID, c.UnitName, c.Room, c.DayOfWeek, c.ScheduledTime, c.SessionDate,
				validMatchKind(c.MatchKind), c.MinutesFromStart, c.Note, deviceHash)
			if execErr == nil {
				written++
			}
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"status": "FILED", "records_written": written})
	}
}

// ─── GET /api/v1/dashboard/qa/presence-claims ────────────────────────────────

// ListPresenceClaims is what a QA officer or school handler opens when a lecturer says the
// patroller never reached the room.
//
// Every claim carries the patrol tick for the SAME lecturer, unit and day beside it, because the
// question is never "what did the lecturer say" on its own — it is whether the two records agree.
// Reading them from two screens and lining them up by eye is how a reviewer gets it wrong.
func ListPresenceClaims(pool *pgxpool.Pool) http.HandlerFunc {
	type claim struct {
		ClaimID        string   `json:"claim_id"`
		LecturerStaff  string   `json:"lecturer_staff_id"`
		LecturerName   string   `json:"lecturer_name"`
		Latitude       *float64 `json:"latitude"`
		Longitude      *float64 `json:"longitude"`
		Accuracy       *float64 `json:"accuracy_metres"`
		LocationStatus string   `json:"location_status"`
		CapturedAt     string   `json:"captured_at"`
		ReceivedAt     string   `json:"received_at"`
		UnitID         string   `json:"unit_id"`
		UnitName       string   `json:"unit_name"`
		Room           string   `json:"room"`
		ScheduledTime  string   `json:"scheduled_time"`
		SessionDate    string   `json:"session_date"`
		MatchKind      string   `json:"match_kind"`
		MinutesFrom    *int     `json:"minutes_from_start"`
		Note           string   `json:"note"`
		// The other record. Absent (nil) when no patroller ticked this slot at all — which is
		// itself the answer to a good number of complaints.
		PatrolTaught  *bool  `json:"patrol_taught"`
		PatrolRoom    string `json:"patrol_room"`
		PatrolTakenAt string `json:"patrol_taken_at"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())

		// Defaults chosen for the screen's actual use: someone is looking at a complaint about
		// something recent. 30 days back, newest first.
		days := 30
		if v := strings.TrimSpace(r.URL.Query().Get("days")); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 365 {
				days = n
			}
		}
		staff := strings.TrimSpace(r.URL.Query().Get("staff_id"))

		conn, err := pool.Acquire(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "db unavailable"))
			return
		}
		defer conn.Release()
		if err := middleware.SetTenantConn(r.Context(), conn, tenantID); err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "db unavailable"))
			return
		}

		rows, qErr := conn.Query(r.Context(), `
			SELECT c.claim_id::text, c.lecturer_staff_id, c.lecturer_name,
			       c.latitude, c.longitude, c.accuracy_metres, c.location_status,
			       c.captured_at, c.received_at,
			       COALESCE(c.unit_id,''), c.unit_name, c.room, c.scheduled_time,
			       COALESCE(c.session_date::text,''), c.match_kind, c.minutes_from_start, c.note,
			       p.taught, COALESCE(p.room,''), p.taken_at
			FROM lecturer_presence_claims c
			-- The patrol tick for the same lecturer, unit and day. LEFT, because "nobody ticked
			-- this at all" is a legitimate and common finding.
			LEFT JOIN LATERAL (
			    SELECT pl.taught, pl.room, pl.taken_at
			    FROM lecturer_patrol_logs pl
			    WHERE pl.session_date = c.session_date
			      AND btrim(lower(pl.lecturer_id)) = btrim(lower(c.lecturer_staff_id))
			      AND (c.unit_id IS NULL OR pl.unit_id = c.unit_id)
			    ORDER BY pl.taken_at DESC
			    LIMIT 1
			) p ON true
			WHERE c.captured_at >= now() - ($1 || ' days')::interval
			  AND ($2 = '' OR btrim(lower(c.lecturer_staff_id)) = btrim(lower($2)))
			ORDER BY c.captured_at DESC
			LIMIT 500`, strconv.Itoa(days), staff)
		if qErr != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", qErr.Error()))
			return
		}
		defer rows.Close()

		out := []claim{}
		for rows.Next() {
			var c claim
			var captured, received time.Time
			var takenAt *time.Time
			if rows.Scan(&c.ClaimID, &c.LecturerStaff, &c.LecturerName,
				&c.Latitude, &c.Longitude, &c.Accuracy, &c.LocationStatus,
				&captured, &received,
				&c.UnitID, &c.UnitName, &c.Room, &c.ScheduledTime,
				&c.SessionDate, &c.MatchKind, &c.MinutesFrom, &c.Note,
				&c.PatrolTaught, &c.PatrolRoom, &takenAt) != nil {
				continue
			}
			c.CapturedAt = captured.Format(time.RFC3339)
			c.ReceivedAt = received.Format(time.RFC3339)
			if takenAt != nil {
				c.PatrolTakenAt = takenAt.Format(time.RFC3339)
			}
			out = append(out, c)
		}
		writeJSON(w, http.StatusOK, out)
	}
}
