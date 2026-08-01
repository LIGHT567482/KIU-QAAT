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
	"math/rand"
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
	ScheduledStart string `json:"scheduled_start,omitempty"`
	ScheduledEnd   string `json:"scheduled_end,omitempty"`
	// DayOfWeek is the unit's scheduled lecture day from the timetable
	// (1=Mon…7=Sun, 0 = unscheduled / runs any day). Drives "today's sessions".
	DayOfWeek       int `json:"day_of_week"`
	DurationMinutes int `json:"session_duration_minutes,omitempty"`
	// The lecturer assigned to this unit — so the coordinator app identifies them
	// AUTOMATICALLY when the unit is chosen (no manual staff-ID entry) and can show
	// their contact on the timetable.
	LecturerStaffID string `json:"lecturer_staff_id,omitempty"`
	LecturerName    string `json:"lecturer_name,omitempty"`
	LecturerPhone   string `json:"lecturer_phone,omitempty"`
	// SessionCode is ONE code that covers BOTH the lecturer and the session: it is generated per
	// lecturer per day (unique across the tenant that day) and delivered on each of that lecturer's
	// sessions, and is set ONLY when the lecturer is shared across several coordinators' concurrent
	// sessions today. A coordinator whose lecturer is physically in another room enters this single
	// code to mark the lecturer present and start attendance. Empty otherwise.
	SessionCode string `json:"session_code,omitempty"`
}

type rosterEntry struct {
	StudentIDHash  string `json:"student_id_hash"`
	QRSerialNumber string `json:"qr_serial_number"`
	// Reg-no + name so the coordinator can see WHO is absent (incl. never-present students) even
	// offline — the privacy hash alone can't be reversed on-device. The durable ledger still keys
	// on the hash; these are display fields only.
	StudentID string `json:"student_id,omitempty"`
	FullName  string `json:"full_name,omitempty"`
}

type manifestPolicy struct {
	AttendanceThreshold  int `json:"attendance_threshold"`
	CheckinWindowMinutes int `json:"checkin_window_minutes"`
	AutoKillMinutes      int `json:"auto_kill_minutes"`
}

type dailyManifest struct {
	ManifestVersion      string            `json:"manifest_version"`
	GeneratedAt          string            `json:"generated_at"`
	ExpiresAt            string            `json:"expires_at"`
	Sessions             []manifestSession `json:"sessions"`
	Policy               manifestPolicy    `json:"policy"`
	InstitutionPublicKey string            `json:"institution_public_key"`
	// StudentHashKey is the per-tenant secret the Coordinator uses to recompute
	// keyed student-id hashes (HMAC-SHA256). Delivered over TLS and stored only
	// inside the AES-encrypted manifest vault on the device (F-07).
	StudentHashKey string                   `json:"student_hash_key"`
	Roster         map[string][]rosterEntry `json:"roster"`
	// WindowOpen is false when the current local time is outside the tenant's
	// daily session window; the coordinator then sees no startable sessions.
	WindowOpen    bool   `json:"window_open"`
	WindowMessage string `json:"window_message,omitempty"`
}

func ManifestDaily(pool *pgxpool.Pool, rdb *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		userID := middleware.GetUserID(r.Context())
		today := time.Now().UTC().Format("2006-01-02")
		cacheKey := fmt.Sprintf("manifest:%s:%s:%s", tenantID, userID, today)

		source := "fresh"
		var manifest *dailyManifest
		if cached, err := rdb.Get(r.Context(), cacheKey).Bytes(); err == nil {
			var m dailyManifest
			if json.Unmarshal(cached, &m) == nil {
				manifest = &m
				source = "cache"
			}
		}
		if manifest == nil {
			built, err := buildManifest(r.Context(), pool, tenantID, userID, today)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{
					"error": "INTERNAL_ERROR", "message": "could not build manifest",
				})
				return
			}
			manifest = built
			raw, _ := json.Marshal(manifest)
			midnight := time.Now().UTC().Truncate(24 * time.Hour).Add(24 * time.Hour)
			rdb.Set(r.Context(), cacheKey, raw, time.Until(midnight)) //nolint:errcheck
		}

		// Re-evaluate the daily session window on EVERY request (the cached body was frozen at
		// first fetch). IMPORTANT: the manifest carries the FULL cohort unit list — we do NOT strip
		// it to "today". Stripping it broke the coordinator's Timetable (units for other days, and
		// the whole grid offline, would vanish) and made units disappear when opening attendance.
		// "Today's" filtering for the attendance picker is done on the device instead. Here we only
		// decide whether a session may be STARTED now: a unit timetabled for today is always
		// startable; otherwise fall back to the tenant's global lecture-day/time window.
		open, msg := withinSessionWindow(r.Context(), pool, tenantID)
		iso := int(time.Now().Weekday())
		if iso == 0 {
			iso = 7 // Sunday → 7
		}
		hasToday := false
		for _, s := range manifest.Sessions {
			if s.DayOfWeek == iso {
				hasToday = true
				break
			}
		}
		switch {
		case hasToday:
			manifest.WindowOpen = true
			manifest.WindowMessage = ""
		case open:
			manifest.WindowOpen = true
			manifest.WindowMessage = msg
		default:
			manifest.WindowOpen = false
			manifest.WindowMessage = msg
		}
		// manifest.Sessions is intentionally left as the full cohort list (no stripping).

		raw, _ := json.Marshal(manifest)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Manifest-Source", source)
		w.Write(raw) //nolint:errcheck
	}
}

// bustCoordinatorManifest clears any cached daily manifest for a coordinator so a
// schedule change (timetable edit) shows up immediately rather than at the next
// midnight cache rollover.
func bustCoordinatorManifest(ctx context.Context, rdb *redis.Client, tenantID, coordinatorID string) {
	if rdb == nil || coordinatorID == "" {
		return
	}
	keys, err := rdb.Keys(ctx, fmt.Sprintf("manifest:%s:%s:*", tenantID, coordinatorID)).Result()
	if err == nil && len(keys) > 0 {
		rdb.Del(ctx, keys...) //nolint:errcheck
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
	var activeAcademicYear string
	var activeSemester int
	err = conn.QueryRow(ctx, `
		SELECT t.attendance_threshold, t.checkin_window_minutes,
		       t.auto_kill_minutes, t.student_hash_key,
		       COALESCE(k.rsa_public_key_pem, ''),
		       COALESCE(t.active_academic_year, ''),
		       COALESCE(t.active_semester, 0)
		FROM tenants t
		LEFT JOIN tenant_rsa_keys k
		       ON k.tenant_id = t.tenant_id AND k.revoked_at IS NULL
		WHERE t.tenant_id = $1
		ORDER BY k.created_at DESC LIMIT 1`, tenantID).
		Scan(&policy.AttendanceThreshold, &policy.CheckinWindowMinutes,
			&policy.AutoKillMinutes, &studentHashKey, &publicKeyPEM,
			&activeAcademicYear, &activeSemester)
	if err != nil {
		return nil, fmt.Errorf("fetch tenant policy: %w", err)
	}

	// Build the coordinator's unit list, scoped to THEIR cohort (the offering's own
	// year + semester + level). There is NO single institution-wide "active semester":
	// within one academic year different intakes/cohorts sit at different (year,
	// semester) positions simultaneously (yr1/sem1, yr1/sem2, yr2/sem1, yr3/sem2 …), so
	// each coordinator is served their own cohort's current-semester catalog and we do
	// NOT gate on a global semester. The academic year IS shared: restrict to it (units
	// with a blank/NULL academic_year always show).
	_ = activeSemester // per-cohort semester lives on the offering; no global gate

	// One coordinator = one cohort. Resolve THIS coordinator's single offering and scope the
	// ENTIRE manifest (unit list + roster) to it, so another cohort's units or students can
	// never leak in. If stray data gave the coordinator more than one offering, take the most
	// recent — the manifest still shows exactly one cohort.
	var offeringID string
	_ = conn.QueryRow(ctx, `
		SELECT offering_id FROM course_offerings
		WHERE coordinator_id = $1 AND tenant_id = $2
		ORDER BY created_at DESC LIMIT 1`, coordinatorID, tenantID).Scan(&offeringID)

	semFilter := "cu.year = o.study_year AND cu.semester = o.semester AND cu.level = o.level"
	semArgs := []interface{}{offeringID, tenantID}
	if activeAcademicYear != "" {
		semFilter += fmt.Sprintf(" AND (cu.academic_year = $%d OR cu.academic_year IS NULL OR cu.academic_year = '')", len(semArgs)+1)
		semArgs = append(semArgs, activeAcademicYear)
	}

	rows, err := conn.Query(ctx, fmt.Sprintf(`
		SELECT cu.unit_id, cu.name,
		       COALESCE(cu.default_venue_id, ''),
		       COALESCE(ous.day_of_week, 0),
		       COALESCE(to_char(ous.session_start, 'HH24:MI'), ''),
		       COALESCE(ous.session_duration_minutes, 0),
		       COALESCE(lec.staff_id, ''), COALESCE(lec.full_name, ''), COALESCE(lec.phone, '')
		FROM course_offerings o
		JOIN course_units cu ON cu.course_id = o.course_id AND cu.tenant_id = o.tenant_id
		LEFT JOIN offering_unit_schedules ous
		       ON ous.offering_id = o.offering_id AND ous.unit_id = cu.unit_id AND ous.tenant_id = o.tenant_id
		-- the assigned lecturer for this unit (one, without multiplying rows)
		LEFT JOIN LATERAL (
		    SELECT l.staff_id, l.full_name, l.phone
		    FROM lecturer_assignments la
		    JOIN lecturers l ON l.lecturer_id = la.lecturer_id AND l.tenant_id = la.tenant_id
		    WHERE la.unit_id = cu.unit_id AND la.tenant_id = o.tenant_id
		    ORDER BY la.academic_year DESC
		    LIMIT 1
		) lec ON true
		WHERE o.offering_id = $1
		  AND o.tenant_id = $2
		  AND %s
		ORDER BY cu.year, cu.semester, cu.name`, semFilter), semArgs...)
	if err != nil {
		return nil, fmt.Errorf("fetch sessions: %w", err)
	}
	defer rows.Close()

	var sessions []manifestSession
	var unitIDs []string
	for rows.Next() {
		var ms manifestSession
		if err := rows.Scan(&ms.UnitID, &ms.UnitName, &ms.VenueID,
			&ms.DayOfWeek, &ms.ScheduledStart, &ms.DurationMinutes,
			&ms.LecturerStaffID, &ms.LecturerName, &ms.LecturerPhone); err != nil {
			return nil, err
		}
		sessions = append(sessions, ms)
		unitIDs = append(unitIDs, ms.UnitID)
	}
	if sessions == nil {
		sessions = []manifestSession{}
	}
	// Multi-coordinator daily codes: for each unit whose lecturer is shared across 2+ coordinators'
	// offerings, attach that lecturer's unique daily 4-digit code (get-or-create). Cached per staff id.
	// ONE code covering both lecturer + session: generated per lecturer per day, attached to each of
	// that lecturer's sessions, only in the multi-coordinator case.
	codeByStaff := map[string]string{}
	for i := range sessions {
		sid := sessions[i].LecturerStaffID
		if sid == "" {
			continue
		}
		c, seen := codeByStaff[sid]
		if !seen {
			c = lecturerDailyCode(ctx, conn, tenantID, sid)
			codeByStaff[sid] = c
		}
		sessions[i].SessionCode = c
	}
	// NB: the daily session-window gate is applied at the ManifestDaily handler
	// (live, post-cache) so the cached body always carries the full session list;
	// freezing an empty list here would survive a later reopening of the window.

	roster := make(map[string][]rosterEntry)
	for _, uid := range unitIDs {
		// COHORT ISOLATION: the roster for a unit is ONLY this coordinator's own cohort
		// (their offering) — not every student of the shared course. Two coordinators of
		// the same course but different cohorts therefore never see each other's students.
		rRows, err := conn.Query(ctx, `
			SELECT s.student_id, COALESCE(s.qr_serial_number,''), COALESCE(s.full_name,'')
			FROM students_extended s
			JOIN course_offerings o ON o.offering_id = s.offering_id
			JOIN course_units cu ON cu.course_id = o.course_id AND cu.tenant_id = o.tenant_id
			WHERE cu.unit_id = $1 AND o.offering_id = $2 AND s.tenant_id = $3
			  AND s.enrollment_status = 'ACTIVE'`,
			uid, offeringID, tenantID)
		if err != nil {
			continue
		}
		// Non-nil so a unit with no enrolled students marshals to [] rather than null
		// (a null roster value crashes strict clients that expect a JSON array).
		entries := make([]rosterEntry, 0)
		for rRows.Next() {
			var studentID, serial, fullName string
			if err := rRows.Scan(&studentID, &serial, &fullName); err != nil {
				continue
			}
			entries = append(entries, rosterEntry{
				StudentIDHash:  hashStudentID(studentHashKey, studentID),
				QRSerialNumber: serial,
				StudentID:      studentID,
				FullName:       fullName,
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
		ExpiresAt:            now.Truncate(24 * time.Hour).Add(24 * time.Hour).Format(time.RFC3339),
		Sessions:             sessions,
		Policy:               policy,
		InstitutionPublicKey: publicKeyPEM,
		StudentHashKey:       studentHashKey,
		Roster:               roster,
		// WindowOpen/WindowMessage are set by the ManifestDaily handler (live).
	}, nil
}

// lecturerDailyCode returns the lecturer's unique daily 4-digit code IF that lecturer (by staff_id)
// is the assigned lecturer for units in 2+ distinct offerings this tenant (the multi-coordinator
// case) — otherwise "". The code is stable for the whole day (get-or-create), unique per tenant/day,
// so every coordinator sharing the lecturer sees the SAME code. Best-effort: any DB error → "".
func lecturerDailyCode(ctx context.Context, conn *pgxpool.Conn, tenantID, staffID string) string {
	if staffID == "" {
		return ""
	}
	var offerings int
	if err := conn.QueryRow(ctx, `
		SELECT COUNT(DISTINCT o.offering_id)
		FROM lecturer_assignments la
		JOIN lecturers l   ON l.lecturer_id = la.lecturer_id AND l.tenant_id = la.tenant_id
		JOIN course_units cu ON cu.unit_id = la.unit_id AND cu.tenant_id = la.tenant_id
		JOIN course_offerings o ON o.course_id = cu.course_id AND o.tenant_id = cu.tenant_id
		WHERE la.tenant_id = $1 AND l.staff_id = $2`, tenantID, staffID).Scan(&offerings); err != nil {
		return ""
	}
	if offerings < 2 {
		return "" // single coordinator → the normal on-hotspot START is enough; no code needed
	}
	return getOrCreateDailyCode(ctx, conn, tenantID, "lec:"+staffID)
}

// getOrCreateDailyCode returns a stable 4-digit code for [subject] (e.g. "lec:<staff>" or
// "unit:<id>") for TODAY, unique across the whole tenant for the day. Idempotent — every caller
// that day gets the same value. Best-effort: any DB error → "".
func getOrCreateDailyCode(ctx context.Context, conn *pgxpool.Conn, tenantID, subject string) string {
	var code string
	_ = conn.QueryRow(ctx, `
		SELECT code FROM lecturer_daily_codes
		WHERE tenant_id = $1 AND lecturer_id = $2 AND valid_date = CURRENT_DATE`,
		tenantID, subject).Scan(&code)
	if code != "" {
		return code
	}
	for i := 0; i < 40; i++ {
		candidate := fmt.Sprintf("%04d", rand.Intn(10000))
		_, _ = conn.Exec(ctx, `
			INSERT INTO lecturer_daily_codes (tenant_id, lecturer_id, code)
			VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`, tenantID, subject, candidate)
		var got string
		_ = conn.QueryRow(ctx, `
			SELECT code FROM lecturer_daily_codes
			WHERE tenant_id = $1 AND lecturer_id = $2 AND valid_date = CURRENT_DATE`,
			tenantID, subject).Scan(&got)
		if got != "" {
			return got
		}
	}
	return ""
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
