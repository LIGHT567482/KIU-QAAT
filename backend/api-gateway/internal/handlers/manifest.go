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
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/qaat/api-gateway/internal/checkin"
	"github.com/qaat/api-gateway/internal/clock"
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

// manifestSlot is ONE row of the weekly timetable grid — one per day a unit runs.
//
// manifestSession carries only the EARLIEST slot of the week per unit (the LATERAL … LIMIT 1
// below), which is all the attendance picker needs but is not a timetable: a unit taught Monday
// and Thursday appeared once. The coordinator's grid was therefore complete online (it reads
// /coordinator/overview, which returns the full grid) and quietly missing days offline — the one
// state where the grid is the only copy they have. These rows are the same shape that endpoint
// returns, so the offline grid is the online grid.
type manifestSlot struct {
	UnitID          string `json:"unit_id"`
	UnitName        string `json:"unit_name"`
	DayOfWeek       int    `json:"day_of_week"`
	StartTime       string `json:"start_time"`
	DurationMinutes int    `json:"duration_minutes"`
	Room            string `json:"room"`
	LecturerName    string `json:"lecturer_name"`
	LecturerPhone   string `json:"lecturer_phone"`
	// WHO IS TEACHING, AND WHICH COMBINED LECTURE THIS IS.
	//
	// A lecture taught to several cohorts at once has one lecturer and several coordinators, each
	// on their own hotspot. The lecturer gates in on ONE of them and reads out three digits; the
	// others type it in. Every phone derives those digits itself from (lecturer, class, today) —
	// see CombinedClassCode — so nothing has to be fetched or exchanged in a hall with no signal.
	//
	// Both fields are here because a phone cannot derive the code without them. The key is NOT
	// stored anywhere: it is computed from the room, weekday and start time this slot already
	// carries, which is precisely the institution's own definition of a combined class — units
	// taught by one lecturer at one time in one room. Computing it server-side rather than leaving
	// each phone to build the string means a change to the rule ships in one place, and two phones
	// can never disagree about what to hash because one of them normalised a room name differently.
	LecturerID       string `json:"lecturer_id"`
	CombinedClassKey string `json:"combined_class_key"`
}

// manifestCachePrefix carries the SHAPE of the manifest body, not the day.
//
// Manifests are cached until midnight, so a deploy that changes what a field MEANS leaves every
// coordinator who already fetched today reading the old meaning until tomorrow. That is how this
// version came to exist: combined_class_key used to be set on every roomed lecture and now marks
// only a genuinely shared one, and a stale cached body would keep putting a code-entry field in
// front of coordinators who have no use for it, for the rest of the day, on the phones of exactly
// the people the change was for.
//
// Bump the token whenever the meaning of the body changes. Every place that builds or sweeps a
// manifest key uses this constant, so the writer and the invalidators cannot drift apart — if they
// did, an edited timetable would stop reaching the phones and nothing would say so.
const manifestCachePrefix = "manifest:v2:"

// manifestSlotsQuery is the weekly grid for one offering, with the one extra question the
// combined-class code depends on: IS THIS LECTURE ACTUALLY SHARED?
//
// A combined class is the exception, not the rule. Most lectures are one cohort in one room, and
// their coordinator must never be shown a code-entry field they have no use for — so the last
// selected column asks whether some OTHER slot has the same lecturer in the same room at the same
// hour, and only a slot that answers yes is given a combined-class key.
//
// The search spans every offering in the tenant, not just this coordinator's: the other cohorts of
// a combined lecture are by definition other offerings, which is the whole point. RLS keeps it
// inside the institution.
//
// Same lecturer is REQUIRED rather than inferred from migration 099. 099 makes a different-lecturer
// overlap impossible, so it might look redundant — but 099 still permits two slots that name NOBODY
// to share a room, and those are unassigned bookings rather than one lecture. Requiring equality
// excludes them for free, because NULL is never equal to NULL, and a slot with no lecturer could not
// derive a code anyway.
//
// Held as a named constant rather than inlined so the test can run the query the manifest actually
// ships — see manifest_combined_test.go.
const manifestSlotsQuery = `
	SELECT s.unit_id, COALESCE(cu.name, s.unit_id), s.day_of_week,
	       to_char(s.start_time,'HH24:MI'), COALESCE(s.duration_minutes, 0),
	       COALESCE(NULLIF(s.room,''), s.venue_id, ''),
	       COALESCE(NULLIF(l.full_name,''), la_l.full_name, ''),
	       COALESCE(NULLIF(l.phone,''),     la_l.phone,     ''),
	       COALESCE(s.lecturer_id::text, ''),
	       EXISTS (
	           SELECT 1 FROM timetable_slots o
	           WHERE o.tenant_id   = s.tenant_id
	             AND o.day_of_week = s.day_of_week
	             AND o.start_time  = s.start_time
	             AND o.lecturer_id = s.lecturer_id
	             AND o.slot_id    <> s.slot_id
	             AND btrim(lower(COALESCE(NULLIF(o.room,''), o.venue_id, ''))) =
	                 btrim(lower(COALESCE(NULLIF(s.room,''), s.venue_id, '')))
	       )
	FROM timetable_slots s
	LEFT JOIN course_units cu ON cu.unit_id = s.unit_id
	LEFT JOIN lecturers   l  ON l.lecturer_id = s.lecturer_id
	LEFT JOIN LATERAL (
	    SELECT l2.full_name, l2.phone FROM lecturer_assignments la
	    JOIN lecturers l2 ON l2.lecturer_id = la.lecturer_id AND l2.tenant_id = la.tenant_id
	    WHERE la.unit_id = s.unit_id
	    ORDER BY la.academic_year DESC LIMIT 1
	) la_l ON true
	WHERE s.offering_id = $1::uuid AND s.tenant_id = $2
	ORDER BY s.day_of_week, s.start_time`

// scanManifestSlots reads the rows of [manifestSlotsQuery] and decides which of them carry a
// combined-class key. Split out from buildManifest so the rule is testable on its own: the phone
// reads "this slot has a key" as "this lecture is shared" and puts its code field on screen on
// exactly that basis, so getting this wrong is visible to every coordinator all day.
func scanManifestSlots(rows pgx.Rows) []manifestSlot {
	slots := make([]manifestSlot, 0)
	for rows.Next() {
		var s manifestSlot
		var combined bool
		if rows.Scan(&s.UnitID, &s.UnitName, &s.DayOfWeek, &s.StartTime,
			&s.DurationMinutes, &s.Room, &s.LecturerName, &s.LecturerPhone,
			&s.LecturerID, &combined) != nil {
			continue
		}
		// The key is derived from the same three fields every cohort's slot carries, so all of them
		// land on one string. An unroomed slot cannot be a combined class — there is no room to
		// share — so it gets no key rather than a misleading one, and neither does a lecture that
		// nobody else is sitting in. The phone cannot make that second judgement itself: its
		// manifest holds only its own offering's slots and it never sees the cohort next door.
		if combined && strings.TrimSpace(s.Room) != "" {
			s.CombinedClassKey = checkin.CombinedClassKey(s.Room, s.DayOfWeek, s.StartTime)
		}
		slots = append(slots, s)
	}
	return slots
}

type rosterEntry struct {
	StudentIDHash string `json:"student_id_hash"`
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
	ManifestVersion string            `json:"manifest_version"`
	GeneratedAt     string            `json:"generated_at"`
	ExpiresAt       string            `json:"expires_at"`
	Sessions        []manifestSession `json:"sessions"`
	// Slots is the FULL weekly grid — every day each unit runs, not one row per unit.
	// The phone caches it so its Timetable tab is complete with no signal.
	Slots  []manifestSlot `json:"slots"`
	Policy manifestPolicy `json:"policy"`
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
		today := clock.Today()
		cacheKey := fmt.Sprintf("%s%s:%s:%s", manifestCachePrefix, tenantID, userID, today)

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
		iso := clock.ISOWeekday()
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
// bustTenantManifests clears the cached manifest for EVERY coordinator in a tenant.
//
// The per-coordinator version below is right when one slot is edited by a known
// coordinator. It is wrong for a bulk timetable import or a slot deletion: those
// change the week for many cohorts at once, and until now neither invalidated
// anything. The cache holds until midnight, so a corrected timetable simply did
// not reach the phones that day — which looks, from the room, exactly like the
// timetable never being updated at all.
func bustTenantManifests(ctx context.Context, rdb *redis.Client, tenantID string) {
	if rdb == nil || tenantID == "" {
		return
	}
	keys, err := rdb.Keys(ctx, fmt.Sprintf("%s%s:*", manifestCachePrefix, tenantID)).Result()
	if err == nil && len(keys) > 0 {
		rdb.Del(ctx, keys...) //nolint:errcheck
	}
}

func bustCoordinatorManifest(ctx context.Context, rdb *redis.Client, tenantID, coordinatorID string) {
	if rdb == nil || coordinatorID == "" {
		return
	}
	keys, err := rdb.Keys(ctx, fmt.Sprintf("%s%s:%s:*", manifestCachePrefix, tenantID, coordinatorID)).Result()
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
	var studentHashKey string
	var activeAcademicYear string
	var activeSemester int
	err = conn.QueryRow(ctx, `
		SELECT 75 /* fixed: internal/policy.AttendanceThresholdPercent */, t.checkin_window_minutes,
		       t.auto_kill_minutes, t.student_hash_key,
		       COALESCE(t.active_academic_year, ''),
		       COALESCE(t.active_semester, 0)
		FROM tenants t
		WHERE t.tenant_id = $1`, tenantID).
		Scan(&policy.AttendanceThreshold, &policy.CheckinWindowMinutes,
			&policy.AutoKillMinutes, &studentHashKey,
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

	// Tolerant level match, as TimetableOverview already uses. The strict form
	// (`cu.level = o.level`) is NULL whenever an offering has no level set, and a
	// NULL predicate drops EVERY unit — so one blank column emptied the
	// coordinator's whole app while the admin's web grid still showed a full week.
	semFilter := `cu.year = o.study_year AND cu.semester = o.semester
	              AND (cu.level = o.level
	                   OR COALESCE(NULLIF(cu.level, ''), '') = ''
	                   OR COALESCE(NULLIF(o.level, ''), '') = '')`
	semArgs := []interface{}{offeringID, tenantID}
	if activeAcademicYear != "" {
		semFilter += fmt.Sprintf(" AND (cu.academic_year = $%d OR cu.academic_year IS NULL OR cu.academic_year = '')", len(semArgs)+1)
		semArgs = append(semArgs, activeAcademicYear)
	}

	rows, err := conn.Query(ctx, fmt.Sprintf(`
		SELECT cu.unit_id, cu.name,
		       -- Room: the slot's own venue first. The unit-level default is a weaker
		       -- answer (it cannot differ per day) and was previously the ONLY one the
		       -- phone saw, so the app and the grid could name two different rooms for
		       -- the same lecture.
		       COALESCE(NULLIF(ts.room, ''), ts.venue_id, cu.default_venue_id, ''),
		       -- Day/time now come from timetable_slots, the table the admin grid and the
		       -- CSV import actually write. offering_unit_schedules survives only as a
		       -- fallback for institutions still on the older set-once schedule; reading
		       -- it FIRST is what made the coordinator's app report an empty day while
		       -- every other surface showed the week.
		       COALESCE(ts.day_of_week, ous.day_of_week, 0),
		       COALESCE(to_char(ts.start_time, 'HH24:MI'), to_char(ous.session_start, 'HH24:MI'), ''),
		       COALESCE(ts.duration_minutes, ous.session_duration_minutes, 0),
		       COALESCE(lec.staff_id, ''), COALESCE(lec.full_name, ''), COALESCE(lec.phone, '')
		FROM course_offerings o
		JOIN course_units cu ON cu.course_id = o.course_id
		-- One row per unit: a unit timetabled twice a week would otherwise duplicate the
		-- whole manifest entry. The earliest slot in the week is the representative one.
		LEFT JOIN LATERAL (
		    SELECT t.day_of_week, t.start_time, t.duration_minutes, t.room, t.venue_id, t.lecturer_id
		    FROM timetable_slots t
		    WHERE t.unit_id = cu.unit_id AND t.offering_id = o.offering_id
		    ORDER BY t.day_of_week, t.start_time
		    LIMIT 1
		) ts ON true
		LEFT JOIN offering_unit_schedules ous
		       ON ous.offering_id = o.offering_id AND ous.unit_id = cu.unit_id
		-- The lecturer for this unit: the slot's own, else the unit's assignment. Slot
		-- first, so a one-off cover lecturer set on the timetable is respected.
		LEFT JOIN LATERAL (
		    SELECT COALESCE(sl.staff_id, al.staff_id)   AS staff_id,
		           COALESCE(sl.full_name, al.full_name) AS full_name,
		           COALESCE(sl.phone, al.phone)         AS phone
		    FROM (SELECT 1) _
		    LEFT JOIN lecturers sl ON sl.lecturer_id = ts.lecturer_id
		    LEFT JOIN LATERAL (
		        SELECT l.staff_id, l.full_name, l.phone
		        FROM lecturer_assignments la
		        JOIN lecturers l ON l.lecturer_id = la.lecturer_id
		        WHERE la.unit_id = cu.unit_id
		        ORDER BY la.academic_year DESC
		        LIMIT 1
		    ) al ON true
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
	// The shared-lecturer code, one per LECTURE.
	//
	// It used to be one code per lecturer per DAY. A lecturer covering an 08:00 and a 14:00
	// lecture read out the same four digits for both, so a coordinator could start their 14:00
	// room with digits overheard at 08:00 and the lecturer never had to arrive. The code is now
	// keyed on the lecturer AND the hour they are teaching, so it starts that lecture and nothing
	// else.
	//
	// Keyed on start time rather than on the unit, deliberately: the whole point of this code is
	// that the same lecture is timetabled under DIFFERENT unit codes and sometimes different unit
	// names in each cohort. What the sharing coordinators genuinely have in common is the lecturer
	// and the hour, so that is the key. It follows that cohorts sharing a lecturer must be
	// timetabled at the same start time — which is what "the lecturer is teaching them at once"
	// means in the first place.
	codeByLecture := map[string]string{}
	for i := range sessions {
		sid, start := sessions[i].LecturerStaffID, sessions[i].ScheduledStart
		if sid == "" || start == "" {
			continue
		}
		key := sid + "@" + start
		c, seen := codeByLecture[key]
		if !seen {
			c = lectureShareCode(ctx, conn, tenantID, sid, sessions[i].DayOfWeek, start)
			codeByLecture[key] = c
		}
		sessions[i].SessionCode = c
	}
	// NB: the daily session-window gate is applied at the ManifestDaily handler
	// (live, post-cache) so the cached body always carries the full session list;
	// freezing an empty list here would survive a later reopening of the window.

	// The full weekly grid for this offering — the same rows /coordinator/overview serves as
	// "slots", so the phone's cached Timetable matches the online one day for day. Best-effort:
	// the manifest's core job is attendance, and a grid that failed to build must not cost the
	// coordinator their roster.
	slots := make([]manifestSlot, 0)
	if offeringID != "" {
		sRows, sErr := conn.Query(ctx, manifestSlotsQuery, offeringID, tenantID)
		if sErr == nil {
			slots = scanManifestSlots(sRows)
			sRows.Close()
		}
	}

	roster := make(map[string][]rosterEntry)
	for _, uid := range unitIDs {
		// COHORT ISOLATION: the roster for a unit is ONLY this coordinator's own cohort
		// (their offering) — not every student of the shared course. Two coordinators of
		// the same course but different cohorts therefore never see each other's students.
		rRows, err := conn.Query(ctx, `
			SELECT s.student_id, COALESCE(s.full_name,'')
			FROM students_extended s
			JOIN course_offerings o ON o.offering_id = s.offering_id
			JOIN course_units cu ON cu.course_id = o.course_id
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
			var studentID, fullName string
			if err := rRows.Scan(&studentID, &fullName); err != nil {
				continue
			}
			entries = append(entries, rosterEntry{
				StudentIDHash: hashStudentID(studentHashKey, studentID),
				StudentID:     studentID,
				FullName:      fullName,
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
		ManifestVersion: fmt.Sprintf("%s-%s", date, idPrefix),
		GeneratedAt:     now.Format(time.RFC3339),
		ExpiresAt:       now.Truncate(24 * time.Hour).Add(24 * time.Hour).Format(time.RFC3339),
		Sessions:        sessions,
		Slots:           slots,
		Policy:          policy,
		StudentHashKey:  studentHashKey,
		Roster:          roster,
		// WindowOpen/WindowMessage are set by the ManifestDaily handler (live).
	}, nil
}

// lectureShareCode returns the 4-digit code for ONE lecture — this lecturer, this weekday, this
// start time — IF that lecture is genuinely shared: timetabled for 2+ distinct offerings, i.e. 2+
// coordinators, at the same hour. Otherwise "" (a single coordinator's lecturer just STARTs in the
// room; no code is needed, and issuing one would only be a spare key lying around).
//
// The sharing test reads timetable_slots, and that is the fix for a bug that made this silently
// useless in exactly the case it exists for. It used to count offerings through
// lecturer_assignments ALONE — tenant-wide, with no day or time filter. Two consequences, both
// bad: it issued codes for lecturers nobody was sharing today, and it issued NO code at all for a
// lecturer attached to their lecture on the timetable slot itself (timetable_slots.lecturer_id)
// rather than through an assignment. The manifest's own lecturer lookup resolves "the slot's own
// lecturer, else the unit's assignment", so a slot-attached lecturer shows up by name on every
// coordinator's timetable and then, at the moment two rooms needed to start, produced nothing.
// Both attachment styles are in live use.
//
// The code is stable for the day (get-or-create) and identical for every coordinator sharing the
// lecture, which is what lets a coordinator verify it with no internet: it reached their phone in
// their own manifest. Best-effort — any DB error yields "", never a wrong code.
func lectureShareCode(ctx context.Context, conn *pgxpool.Conn, tenantID, staffID string, dayOfWeek int, startTime string) string {
	if staffID == "" || startTime == "" {
		return ""
	}
	var offerings int
	if err := conn.QueryRow(ctx, `
		SELECT COUNT(DISTINCT ts.offering_id)
		FROM timetable_slots ts
		JOIN lecturers l ON l.staff_id = $2
		WHERE ts.tenant_id = $1
		  AND ts.day_of_week = $3
		  AND to_char(ts.start_time, 'HH24:MI') = $4
		  AND ( ts.lecturer_id = l.lecturer_id
		     OR ( ts.lecturer_id IS NULL AND EXISTS (
		            SELECT 1 FROM lecturer_assignments la
		            WHERE la.unit_id = ts.unit_id
		              AND la.lecturer_id = l.lecturer_id) ) )`,
		tenantID, staffID, dayOfWeek, startTime).Scan(&offerings); err != nil {
		return ""
	}
	if offerings < 2 {
		return "" // one coordinator → the normal on-hotspot START is enough
	}
	// Subject includes the hour, so the same lecturer's next lecture gets a different code.
	return getOrCreateDailyCode(ctx, conn, tenantID, fmt.Sprintf("lec:%s@%d:%s", staffID, dayOfWeek, startTime))
}

// getOrCreateDailyCode returns a stable 4-digit code for [subject] (e.g. "lec:<staff>" or
// "unit:<id>") for TODAY, unique across the whole tenant for the day. Idempotent — every caller
// that day gets the same value. Best-effort: any DB error → "".
func getOrCreateDailyCode(ctx context.Context, conn *pgxpool.Conn, tenantID, subject string) string {
	var code string
	_ = conn.QueryRow(ctx, `
		SELECT code FROM lecturer_daily_codes
		WHERE lecturer_id = $1 AND valid_date = CURRENT_DATE`,
		subject).Scan(&code)
	if code != "" {
		return code
	}
	for i := 0; i < 40; i++ {
		candidate := fmt.Sprintf("%04d", rand.Intn(10000))
		_, _ = conn.Exec(ctx, `
			INSERT INTO lecturer_daily_codes (lecturer_id, code)
			VALUES ($1, $2) ON CONFLICT DO NOTHING`, subject, candidate)
		var got string
		_ = conn.QueryRow(ctx, `
			SELECT code FROM lecturer_daily_codes
			WHERE lecturer_id = $1 AND valid_date = CURRENT_DATE`,
			subject).Scan(&got)
		if got != "" {
			return got
		}
	}
	return ""
}

// hashStudentID returns HMAC-SHA256(student_hash_key, student_id) as hex. Keying
// the hash with a per-tenant secret prevents reversing a low-entropy registration
// number by brute force or rainbow table (F-07). The coordinator app computes the
// same value from the key delivered in the manifest.
func hashStudentID(key, id string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(id))
	return hex.EncodeToString(mac.Sum(nil))
}
