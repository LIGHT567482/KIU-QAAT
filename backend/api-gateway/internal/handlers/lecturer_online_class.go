package handlers

// DISTANCE LEARNING — the lecturer's side.
//
//	GET    /api/v1/lecturer/online-class        the class they have running now, with the live code
//	POST   /api/v1/lecturer/online-class        start one
//	POST   /api/v1/lecturer/online-class/end    end it, which is what books the contact hours
//
// WHY THE LECTURER AND NOT THE COORDINATOR. Every other session in this system is opened by a
// coordinator, because the coordinator is the person physically holding the door of the room. A
// distance class has no room and no door. Waiting for a coordinator to open it would mean a
// Sunday-evening e-learning lecture cannot start unless a member of staff is sitting somewhere with
// the app open, which is not how these cohorts run. So the lecturer starts their own online class,
// and their JWT is the identity proof that the QR-scan-at-the-door was providing on campus.
//
// THE ONE HARD RULE: the cohort must already be marked ONLINE (course_offerings.delivery_mode, set
// by migration 087 from the session type the institution already types, and editable by the admin).
// Without that rule this endpoint would be a way to open a room-less, LAN-less session for a Day
// cohort — and every campus student could then mark themselves present from home, which is exactly
// what the proximity gate exists to prevent. The refusal below is therefore not a validation
// nicety; it is the boundary that keeps the online path from eating the physical one.
//
// The lecturer is recorded present at the moment they start, with lecturer_scanned_at set — the
// same column the gate scan writes, because it means the same thing (this lecturer turned up and
// began teaching) and because every downstream report, the missed-class list included, keys on it.
// What differs is that the session says ONLINE, so nobody reading the record can mistake a remote
// start for a scan at a door.

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qaat/api-gateway/internal/checkin"
	"github.com/qaat/api-gateway/internal/clock"
	"github.com/qaat/api-gateway/internal/middleware"
)

type onlineClassRequest struct {
	UnitID      string `json:"unit_id"`
	OfferingID  string `json:"offering_id"` // which cohort; required when the unit serves several
	Fingerprint string `json:"fingerprint"`
}

type onlineClassResponse struct {
	SessionID        string `json:"session_id"`
	UnitID           string `json:"unit_id"`
	UnitName         string `json:"unit_name"`
	CohortLabel      string `json:"cohort_label"`
	StudentCode      string `json:"student_code"` // ROTATING — see the note in StudentCheckin
	SecondsRemaining int    `json:"seconds_remaining"`
	CheckinWindowEnd string `json:"checkin_window_end"`
	Present          int    `json:"present"`
	Enrolled         int    `json:"enrolled"`
	StartedAt        string `json:"started_at"`
}

// onlineCohort is what the ONLINE rule resolves to before anything is written.
type onlineCohort struct {
	offeringID, unitName, label, coordinatorID string
	enrolled                                   int
}

// resolveOnlineCohort finds the ONLINE cohort this lecturer may open a class for, and explains
// itself when it refuses — "your cohort is not set up for e-learning" is actionable, a bare 403
// sends someone to the IT office.
func resolveOnlineCohort(ctx context.Context, pool *pgxpool.Pool, tenantID, lecturerID string,
	req onlineClassRequest) (onlineCohort, string, string) {

	args := []interface{}{tenantID, lecturerID, req.UnitID}
	filter := ""
	if strings.TrimSpace(req.OfferingID) != "" {
		if !middleware.ValidTenantID(strings.TrimSpace(req.OfferingID)) {
			return onlineCohort{}, "INVALID_REQUEST", "that cohort id is not a valid id"
		}
		args = append(args, strings.TrimSpace(req.OfferingID))
		filter = " AND o.offering_id = $4::uuid"
	}

	rows, err := pool.Query(ctx, `
		SELECT DISTINCT o.offering_id::text, COALESCE(cu.name, ts.unit_id),
		       COALESCE(NULLIF(CONCAT_WS(' · ', c.name, o.session_type,
		                'Yr' || o.study_year, 'Sem' || o.semester, NULLIF(o.intake,'')), ''), ''),
		       COALESCE(o.coordinator_id, ''),
		       COALESCE(o.delivery_mode, 'IN_PERSON'),
		       (SELECT COUNT(*) FROM students_extended se
		         WHERE se.offering_id = o.offering_id
		           AND se.enrollment_status = 'ACTIVE')
		FROM timetable_slots ts
		JOIN course_offerings o ON o.offering_id = ts.offering_id
		JOIN course_units cu    ON cu.unit_id = ts.unit_id
		LEFT JOIN courses c     ON c.course_id = o.course_id
		WHERE ts.tenant_id = $1 AND ts.unit_id = $3
		  AND ( ts.lecturer_id = $2::uuid
		     OR ( ts.lecturer_id IS NULL
		          AND EXISTS (SELECT 1 FROM lecturer_assignments la
		                      WHERE la.unit_id = ts.unit_id
		                        AND la.lecturer_id = $2::uuid) ) )`+filter, args...)
	if err != nil {
		return onlineCohort{}, "INTERNAL_ERROR", err.Error()
	}
	defer rows.Close()

	var online []onlineCohort
	inPerson := 0
	for rows.Next() {
		var c onlineCohort
		var mode string
		if rows.Scan(&c.offeringID, &c.unitName, &c.label, &c.coordinatorID, &mode, &c.enrolled) != nil {
			continue
		}
		if mode == "ONLINE" {
			online = append(online, c)
		} else {
			inPerson++
		}
	}

	switch {
	case len(online) == 1:
		return online[0], "", ""
	case len(online) > 1:
		// Ambiguity is refused rather than guessed: picking one would put half a cohort's
		// attendance under the other's roster, and nobody would notice until the exam board.
		return onlineCohort{}, "AMBIGUOUS_COHORT",
			"you teach this unit to more than one e-learning cohort — choose which one"
	case inPerson > 0:
		return onlineCohort{}, "NOT_AN_ONLINE_COHORT",
			"this cohort is taught in person, so its attendance is taken in the room. " +
				"If it should be an e-learning cohort, the administrator sets that on the cohort."
	default:
		return onlineCohort{}, "NOT_YOUR_UNIT",
			"you have no timetabled cohort for this unit"
	}
}

// POST /api/v1/lecturer/online-class
func StartOnlineClass(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		userID := middleware.GetUserID(r.Context())
		lecturerID, ok := resolveLecturerID(adminPool, r, tenantID, userID)
		if !ok {
			writeJSON(w, http.StatusForbidden, errBody("NOT_A_LECTURER", "no lecturer record is linked to this account"))
			return
		}

		var req onlineClassRequest
		if err := decodeJSON(r, &req); err != nil || strings.TrimSpace(req.UnitID) == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "unit_id is required"))
			return
		}
		req.UnitID = strings.TrimSpace(req.UnitID)

		cohort, code, msg := resolveOnlineCohort(r.Context(), adminPool, tenantID, lecturerID, req)
		if code != "" {
			status := http.StatusForbidden
			switch code {
			case "INVALID_REQUEST":
				status = http.StatusBadRequest
			case "AMBIGUOUS_COHORT":
				status = http.StatusConflict
			case "INTERNAL_ERROR":
				status = http.StatusInternalServerError
			}
			writeJSON(w, status, errBody(code, msg))
			return
		}

		// A class already running is returned rather than duplicated. The lecturer's screen may
		// have been closed and reopened, or the phone lost signal mid-lecture; a second session
		// would split the roster in two and show them a code half the class cannot use.
		if existing, ok := currentOnlineClass(r.Context(), adminPool, tenantID, lecturerID); ok {
			writeJSON(w, http.StatusOK, existing)
			return
		}

		// NOT gated on the tenant's daily session window, unlike a room. That window exists to stop
		// a physical room being opened at three in the morning; e-learning cohorts study in the
		// evenings and at weekends precisely because they cannot attend in the day, so applying it
		// here would refuse the only hours these classes ever run in.

		secret := make([]byte, 32)
		if _, err := rand.Read(secret); err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "could not start the class"))
			return
		}

		var windowMinutes int
		if err := adminPool.QueryRow(r.Context(),
			`SELECT checkin_window_minutes FROM tenants WHERE tenant_id = $1`, tenantID).
			Scan(&windowMinutes); err != nil || windowMinutes <= 0 {
			windowMinutes = 30
		}
		now := time.Now().UTC()
		windowEnd := now.Add(time.Duration(windowMinutes) * time.Minute)

		tx, err := adminPool.Begin(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "could not start the class"))
			return
		}
		defer tx.Rollback(context.Background()) //nolint:errcheck

		// Same lock discipline as OpenSession, keyed on the lecturer: two taps arriving together
		// must not produce two sessions for one lecture.
		if _, err := tx.Exec(r.Context(),
			`SELECT pg_advisory_xact_lock(hashtext($1 || ':online:' || $2))`, tenantID, lecturerID); err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "could not start the class"))
			return
		}

		// coordinator_id is NOT NULL and is half the vector-clock key on attendance_logs, so it
		// must be filled. For an online class the lecturer IS the person running it, so their own
		// user id is the honest value — using the cohort's coordinator would also collide with
		// that coordinator's own open session in the "one open session" rule and stop a physical
		// class and an online one running at the same hour.
		var sessionID string
		err = tx.QueryRow(r.Context(), `
			INSERT INTO sessions
			    (tenant_id, coordinator_id, unit_id, lecturer_id, session_date,
			     gate_open_time, checkin_window_start, checkin_window_end,
			     session_status, checkin_secret, offering_id, delivery_mode)
			VALUES ($1, $2, $3, $4, $5, $6, $6, $7, 'ACTIVE', $8, $9::uuid, 'ONLINE')
			RETURNING session_id::text`,
			tenantID, userID, req.UnitID, lecturerID, clock.Today(),
			now, windowEnd, secret, cohort.offeringID).Scan(&sessionID)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("OPEN_FAILED", "could not start the class: "+err.Error()))
			return
		}

		// The lecturer is present from this moment. lecturer_scanned_at is the column every report
		// and the missed-class list read, and a remote start means the same thing a gate scan does:
		// this named person began teaching. The session's ONLINE mode is what distinguishes them.
		if _, err := tx.Exec(r.Context(), `
			INSERT INTO lecturer_attendance_logs
			    (tenant_id, session_id, lecturer_id, gate_open_time, unit_id, session_date,
			     lecturer_scanned_at, lecturer_fingerprint_hash)
			VALUES ($1, $2, $3, $4, $5, $6, $4, NULLIF($7,''))`,
			tenantID, sessionID, lecturerID, now, req.UnitID, clock.Today(),
			strings.TrimSpace(req.Fingerprint)); err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "could not record your attendance"))
			return
		}

		if err := tx.Commit(r.Context()); err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "could not start the class"))
			return
		}

		writeJSON(w, http.StatusCreated, onlineClassResponse{
			SessionID: sessionID, UnitID: req.UnitID, UnitName: cohort.unitName,
			CohortLabel: cohort.label,
			// ROTATING, not the static student code. On campus the static code is safe because the
			// LAN gate is doing the real work; online there is no LAN, so a code that never changes
			// would be forwarded once and reused by the whole year group for the rest of the hour.
			StudentCode:      checkin.Derive(secret, now),
			SecondsRemaining: checkin.SecondsRemaining(now),
			CheckinWindowEnd: windowEnd.Format(time.RFC3339),
			Enrolled:         cohort.enrolled,
			StartedAt:        now.Format(time.RFC3339),
		})
	}
}

// GET /api/v1/lecturer/online-class — the running class and its live code, polled by the screen
// the lecturer is sharing. Returns 204 when there is none, so the dashboard can show the start
// button without treating "nothing running" as an error.
func GetOnlineClass(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		userID := middleware.GetUserID(r.Context())
		lecturerID, ok := resolveLecturerID(adminPool, r, tenantID, userID)
		if !ok {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if resp, ok := currentOnlineClass(r.Context(), adminPool, tenantID, lecturerID); ok {
			writeJSON(w, http.StatusOK, resp)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// currentOnlineClass returns the lecturer's running online session, with the code recomputed for
// NOW — the code rotates every 10 seconds and is never stored, so each poll derives it afresh from
// the session secret, which never leaves the server.
func currentOnlineClass(ctx context.Context, pool *pgxpool.Pool, tenantID, lecturerID string) (onlineClassResponse, bool) {
	var (
		resp      onlineClassResponse
		secret    []byte
		windowEnd *time.Time
		startedAt *time.Time
	)
	err := pool.QueryRow(ctx, `
		SELECT s.session_id::text, s.unit_id, COALESCE(cu.name, s.unit_id),
		       COALESCE(NULLIF(CONCAT_WS(' · ', c.name, o.session_type,
		                'Yr' || o.study_year, 'Sem' || o.semester, NULLIF(o.intake,'')), ''), ''),
		       s.checkin_secret, s.checkin_window_end, lal.lecturer_scanned_at,
		       (SELECT COUNT(*) FROM attendance_logs al WHERE al.session_id = s.session_id),
		       (SELECT COUNT(*) FROM students_extended se
		         WHERE se.offering_id = s.offering_id
		           AND se.enrollment_status = 'ACTIVE')
		FROM sessions s
		LEFT JOIN course_units cu    ON cu.unit_id = s.unit_id
		LEFT JOIN course_offerings o ON o.offering_id = s.offering_id
		LEFT JOIN courses c          ON c.course_id = o.course_id
		LEFT JOIN lecturer_attendance_logs lal ON lal.session_id = s.session_id
		WHERE s.tenant_id = $1 AND s.lecturer_id = $2 AND s.delivery_mode = 'ONLINE'
		  AND s.session_status = 'ACTIVE'
		ORDER BY s.gate_open_time DESC LIMIT 1`,
		tenantID, lecturerID).Scan(&resp.SessionID, &resp.UnitID, &resp.UnitName, &resp.CohortLabel,
		&secret, &windowEnd, &startedAt, &resp.Present, &resp.Enrolled)
	if err != nil || len(secret) == 0 {
		return onlineClassResponse{}, false
	}
	now := time.Now().UTC()
	resp.StudentCode = checkin.Derive(secret, now)
	resp.SecondsRemaining = checkin.SecondsRemaining(now)
	if windowEnd != nil {
		resp.CheckinWindowEnd = windowEnd.Format(time.RFC3339)
	}
	if startedAt != nil {
		resp.StartedAt = startedAt.Format(time.RFC3339)
	}
	return resp, true
}

// POST /api/v1/lecturer/online-class/end
//
// Ending is what books the contact hours, so it carries the same quorum rule the physical gate
// applies: a lecture nobody attended does not earn teaching credit. The rule is not relaxed for
// being remote — if anything a claim of having taught to an empty online room is easier to make.
func EndOnlineClass(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		userID := middleware.GetUserID(r.Context())
		lecturerID, ok := resolveLecturerID(adminPool, r, tenantID, userID)
		if !ok {
			writeJSON(w, http.StatusForbidden, errBody("NOT_A_LECTURER", "no lecturer record is linked to this account"))
			return
		}

		var sessionID string
		var attended, enrolled int
		var ratio float64
		err := adminPool.QueryRow(r.Context(), `
			SELECT s.session_id::text,
			       (SELECT COUNT(*) FROM attendance_logs al WHERE al.session_id = s.session_id),
			       (SELECT COUNT(*) FROM students_extended se
			         WHERE se.offering_id = s.offering_id
			           AND se.enrollment_status = 'ACTIVE'),
			       COALESCE(t.lecturer_attendance_ratio, 0.5)
			FROM sessions s
			CROSS JOIN tenants t
			WHERE s.tenant_id = $1 AND s.lecturer_id = $2 AND s.delivery_mode = 'ONLINE'
			  AND s.session_status = 'ACTIVE'
			ORDER BY s.gate_open_time DESC LIMIT 1`,
			tenantID, lecturerID).Scan(&sessionID, &attended, &enrolled, &ratio)
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, errBody("NO_OPEN_CLASS", "you have no online class running"))
			return
		}
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}

		required := 1
		if q := int(math.Ceil(ratio * float64(enrolled))); q > required {
			required = q
		}
		if attended < required {
			writeJSON(w, http.StatusUnprocessableEntity, errBody("NO_QUORUM",
				fmt.Sprintf("not enough students checked in (%d of %d present; %d needed) — "+
					"the class cannot be closed yet", attended, enrolled, required)))
			return
		}

		now := time.Now().UTC()
		if _, err := adminPool.Exec(r.Context(), `
			UPDATE lecturer_attendance_logs
			   SET lecturer_ended_at = $1,
			       gate_close_time = $1,
			       contact_hours = ROUND(EXTRACT(EPOCH FROM ($1 - lecturer_scanned_at)) / 3600.0, 2)
			 WHERE session_id = $2 AND tenant_id = $3 AND lecturer_ended_at IS NULL`,
			now, sessionID, tenantID); err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		if _, err := adminPool.Exec(r.Context(), `
			UPDATE sessions SET session_status = 'CLOSED', coordinator_end_time = $1
			 WHERE session_id = $2 AND tenant_id = $3`, now, sessionID, tenantID); err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status": "ENDED", "session_id": sessionID,
			"present": attended, "enrolled": enrolled,
			"ended_at": now.Format(time.RFC3339),
		})
	}
}
