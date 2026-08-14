package handlers

// Multi-slot timetable (timetable_slots, migration 041) — the weekly grid that the
// redesigned Timetable page renders and that the timetable importer fills. A unit
// can have many slots (one per day, each with its own room), unlike the legacy
// single offering_unit_schedules row.
//
//   GET    /api/v1/dashboard/timetable/slots         (ADMIN, QA) — offerings + all slots
//   PUT    /api/v1/dashboard/timetable/slots         (ADMIN, QA) — upsert one slot
//   DELETE /api/v1/dashboard/timetable/slots/{slot_id}
//   POST   /api/v1/admin/tenants/{tenant_id}/timetable/import — bulk CSV/XLSX

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/qaat/api-gateway/internal/middleware"
)

type slotOffering struct {
	OfferingID  string `json:"offering_id"`
	CourseID    string `json:"course_id"`
	CourseName  string `json:"course_name"`
	SessionType string `json:"session_type"`
	StudyYear   int    `json:"study_year"`
	Semester    int    `json:"semester"`
	Level       string `json:"level"`
	Intake      string `json:"intake"`
	Coordinator string `json:"coordinator_name"`
}

type slotRow struct {
	SlotID       string `json:"slot_id"`
	OfferingID   string `json:"offering_id"`
	UnitID       string `json:"unit_id"`
	UnitName     string `json:"unit_name"`
	DayOfWeek    int    `json:"day_of_week"`
	StartTime    string `json:"start_time"`
	Duration     int    `json:"duration_minutes"`
	Room         string `json:"room"`
	RoomCode     string `json:"room_code"` // the managed room this slot resolved to, "" if unmatched
	LecturerID   string `json:"lecturer_id"`
	LecturerName string `json:"lecturer_name"`
	// The department this lecture belongs to, via its unit's course. Carried so the grid can
	// tell a departmental TLC which rows are theirs to change.
	Department string `json:"department"`
}

// GET /api/v1/dashboard/timetable/slots
func GetTimetableSlots(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
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

		offRows, err := conn.Query(r.Context(), `
			SELECT o.offering_id::text, o.course_id, c.name, o.session_type,
			       o.study_year, o.semester, COALESCE(o.level,''), COALESCE(o.intake,''),
			       COALESCE(u.full_name,'')
			FROM course_offerings o
			JOIN courses c ON c.course_id = o.course_id
			LEFT JOIN users u ON u.user_id::text = o.coordinator_id
			WHERE o.tenant_id = $1
			ORDER BY c.name, o.session_type, o.study_year, o.semester, o.level, o.intake`, tenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		offerings := []slotOffering{}
		for offRows.Next() {
			var o slotOffering
			offRows.Scan(&o.OfferingID, &o.CourseID, &o.CourseName, &o.SessionType, &o.StudyYear, &o.Semester, &o.Level, &o.Intake, &o.Coordinator) //nolint:errcheck
			offerings = append(offerings, o)
		}
		offRows.Close()

		// The unit's department travels with the slot so the grid can grey out what this viewer
		// may not edit, instead of offering an editor that answers 403 on save.
		sRows, err := conn.Query(r.Context(), `
			SELECT s.slot_id::text, s.offering_id::text, s.unit_id, COALESCE(cu.name, s.unit_id),
			       s.day_of_week, to_char(s.start_time,'HH24:MI'), s.duration_minutes,
			       COALESCE(s.room,''), COALESCE(s.venue_id,''),
			       COALESCE(s.lecturer_id::text,''), COALESCE(l.full_name,''),
			       COALESCE(c.department,'')
			FROM timetable_slots s
			LEFT JOIN course_units cu ON cu.unit_id = s.unit_id
			LEFT JOIN courses     c  ON c.course_id = cu.course_id
			LEFT JOIN lecturers   l  ON l.lecturer_id = s.lecturer_id
			WHERE s.tenant_id = $1
			ORDER BY s.day_of_week, s.start_time`, tenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer sRows.Close()
		slots := []slotRow{}
		for sRows.Next() {
			var s slotRow
			sRows.Scan(&s.SlotID, &s.OfferingID, &s.UnitID, &s.UnitName, &s.DayOfWeek, &s.StartTime, &s.Duration, &s.Room, &s.RoomCode, &s.LecturerID, &s.LecturerName, &s.Department) //nolint:errcheck
			slots = append(slots, s)
		}
		// The whole institution's timetable stays READABLE for a departmental TLC — rooms are
		// shared, and you cannot avoid a clash you cannot see. tlc_department is what they may
		// EDIT; empty means everything, which is what an admin, a QA officer, and an
		// institution-wide TLC each get.
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"offerings": offerings,
			"slots":     slots,
			"tlc_department": tlcDepartment(r.Context(), conn, tenantID,
				middleware.GetUserID(r.Context()), middleware.GetRole(r.Context())),
		})
	}
}

// PUT /api/v1/dashboard/timetable/slots — upsert one slot.
func UpsertTimetableSlot(pool *pgxpool.Pool, rdb *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		var req struct {
			OfferingID string `json:"offering_id"`
			UnitID     string `json:"unit_id"`
			DayOfWeek  int    `json:"day_of_week"`
			StartTime  string `json:"start_time"`
			Duration   int    `json:"duration_minutes"`
			Room       string `json:"room"`
			LecturerID string `json:"lecturer_id"`
		}
		if err := decodeJSON(r, &req); err != nil || req.OfferingID == "" || req.UnitID == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "offering_id and unit_id are required"))
			return
		}
		if req.DayOfWeek < 1 || req.DayOfWeek > 7 || !validHHMM(req.StartTime) {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_SCHEDULE", "day_of_week 1–7 and start_time HH:MM are required"))
			return
		}
		if req.Duration <= 0 {
			req.Duration = 60
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
		// A departmental TLC designs their OWN department's timetable and no one else's.
		if err := checkTimetableScope(r.Context(), conn, tenantID,
			middleware.GetUserID(r.Context()), middleware.GetRole(r.Context()), req.UnitID); err != nil {
			writeTimetableScopeRefusal(w, err)
			return
		}

		var coordinatorID, sessionType string
		_ = conn.QueryRow(r.Context(), `SELECT COALESCE(coordinator_id,''), COALESCE(session_type,'') FROM course_offerings WHERE offering_id=$1::uuid AND tenant_id=$2`, req.OfferingID, tenantID).Scan(&coordinatorID, &sessionType)

		// A WEEKEND cohort may only be scheduled on Sat/Sun; a Day/Evening cohort only on
		// Mon–Fri. This stops "day units" appearing in a weekend cohort (and vice-versa).
		if weekend := strings.Contains(strings.ToLower(sessionType), "weekend"); weekend != (req.DayOfWeek == 6 || req.DayOfWeek == 7) {
			day := "a weekday (Mon–Fri)"
			if weekend {
				day = "a weekend day (Sat/Sun)"
			}
			writeJSON(w, http.StatusConflict, errBody("DAY_MISMATCH",
				fmt.Sprintf("A %s cohort can only be timetabled on %s.", sessionType, day)))
			return
		}

		// Clash guard: no two units in the SAME cohort may occupy overlapping time on the
		// same day (two ranges overlap when each starts before the other ends). The exact
		// slot being upserted is excluded so re-saving it isn't a self-clash.
		var clashUnit, clashStart string
		clashErr := conn.QueryRow(r.Context(), `
			SELECT unit_id, to_char(start_time,'HH24:MI')
			FROM timetable_slots
			WHERE tenant_id = $1 AND offering_id = $2::uuid AND day_of_week = $3
			  AND NOT (unit_id = $4 AND start_time = $5::time)
			  AND start_time < ($5::time + make_interval(mins => $6))
			  AND $5::time    < (start_time + make_interval(mins => duration_minutes))
			LIMIT 1`,
			tenantID, req.OfferingID, req.DayOfWeek, req.UnitID, req.StartTime, req.Duration).Scan(&clashUnit, &clashStart)
		if clashErr == nil {
			writeJSON(w, http.StatusConflict, errBody("SCHEDULE_CLASH",
				fmt.Sprintf("That time clashes with %s at %s on the same day — a cohort can't have two units at once.", clashUnit, clashStart)))
			return
		}

		// ── Room clash, across the WHOLE institution ────────────────────────
		//
		// The guard above asks whether this COHORT is already busy — a question about the students'
		// diary, scoped to one offering. This asks whether the ROOM is, and it deliberately looks
		// past the offering, the department and the college.
		//
		// That is the case nobody was catching. Each department has its own TLC, they schedule
		// independently into one shared pool of rooms, and two of them booking Block C 101 for
		// Tuesday at 14:00 was accepted by both — discovered by two lecturers and eighty students
		// arriving at the same door. Rooms are compared case- and whitespace-insensitively because
		// the room is free text somebody types; an unroomed slot books nothing and is skipped.
		if room := strings.TrimSpace(req.Room); room != "" {
			var otherUnit, otherStart, otherCohort string
			roomErr := conn.QueryRow(r.Context(), `
				SELECT ts.unit_id, to_char(ts.start_time,'HH24:MI'),
				       COALESCE(NULLIF(c.department,''), COALESCE(NULLIF(c.school,''), ''))
				FROM timetable_slots ts
				LEFT JOIN course_units cu ON cu.unit_id = ts.unit_id
				LEFT JOIN courses c ON c.course_id = cu.course_id
				WHERE ts.tenant_id = $1 AND ts.day_of_week = $2
				  AND btrim(lower(ts.room)) = btrim(lower($3))
				  -- Not this same slot being re-saved.
				  AND NOT (ts.offering_id = $4::uuid AND ts.unit_id = $5 AND ts.start_time = $6::time)
				  AND ts.start_time < ($6::time + make_interval(mins => $7))
				  AND $6::time     < (ts.start_time + make_interval(mins => ts.duration_minutes))
				  -- THE COMBINED CLASS IS NOT A CLASH. One lecturer teaching several cohorts in one
				  -- room at one hour is normal here, and the cohorts often carry the unit under
				  -- different codes — so two slots sharing a room and time collide only when they
				  -- name DIFFERENT lecturers, which is the case where only one of them can really
				  -- be teaching. Mirrors the database constraint in migration 099; unassigned slots
				  -- fold onto one sentinel so "nobody named twice" is still caught.
				  AND COALESCE(ts.lecturer_id, '00000000-0000-0000-0000-000000000000'::uuid)
				      <> COALESCE(NULLIF($8,'')::uuid, '00000000-0000-0000-0000-000000000000'::uuid)
				LIMIT 1`,
				tenantID, req.DayOfWeek, room, req.OfferingID, req.UnitID, req.StartTime, req.Duration,
				req.LecturerID,
			).Scan(&otherUnit, &otherStart, &otherCohort)
			if roomErr == nil {
				// Name the other department where we know it. A TLC told only "the room is taken"
				// has to go and find whose booking it is before they can do anything about it, and
				// the whole point is that the other booking belongs to somebody else.
				owner := ""
				if strings.TrimSpace(otherCohort) != "" {
					owner = " (" + otherCohort + ")"
				}
				writeJSON(w, http.StatusConflict, errBody("ROOM_DOUBLE_BOOKED",
					fmt.Sprintf("%s is already booked for %s%s at %s that day. Pick another room or another time.",
						room, otherUnit, owner, otherStart)))
				return
			}
		}

		// The typed room is also resolved against the managed room registry, so the slot carries a
		// structured venue_id wherever the text names a real room. The free text stays as written —
		// it is what the grid displays, and an unrecognised room must not be silently dropped.
		var slotID string
		err = conn.QueryRow(r.Context(), `
			INSERT INTO timetable_slots (tenant_id, offering_id, unit_id, day_of_week, start_time, duration_minutes, room, lecturer_id, venue_id)
			VALUES ($1, $2::uuid, $3, $4, $5::time, $6, NULLIF($7,''), NULLIF($8,'')::uuid, `+resolveVenueSQL+`)
			ON CONFLICT (offering_id, unit_id, day_of_week, start_time) DO UPDATE
			   SET duration_minutes = EXCLUDED.duration_minutes,
			       room             = EXCLUDED.room,
			       -- COALESCE, not EXCLUDED: the grid's slot editor has historically sent no
			       -- lecturer_id at all, so a plain assignment wrote NULL and every re-save
			       -- silently erased the lecturer the CSV import had set. That is the root of
			       -- "the coordinator dashboard says there are no lecturers for that day" —
			       -- the name was there until someone nudged the slot. An explicit lecturer
			       -- still overwrites; an omitted one now leaves what is there alone.
			       lecturer_id      = COALESCE(EXCLUDED.lecturer_id, timetable_slots.lecturer_id),
			       venue_id         = EXCLUDED.venue_id
			RETURNING slot_id::text`,
			tenantID, req.OfferingID, req.UnitID, req.DayOfWeek, req.StartTime, req.Duration, req.Room, req.LecturerID).Scan(&slotID)
		if err != nil {
			// THE RACE THE CHECK ABOVE CANNOT WIN. That check is a read followed by a write, and
			// two TLCs pressing Save in the same second both read "room free" before either
			// writes. Migration 091's exclusion constraint is what actually makes the overlap
			// impossible; this turns its 23P01 into the same sentence the pre-check would have
			// given, so the loser of the race is told what happened rather than shown a database
			// error naming a GiST index.
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23P01" &&
				strings.Contains(pgErr.ConstraintName, "room_double_booking") {
				writeJSON(w, http.StatusConflict, errBody("ROOM_DOUBLE_BOOKED",
					strings.TrimSpace(req.Room)+" was booked for that time by someone else a moment ago. Pick another room or another time."))
				return
			}
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		bustCoordinatorManifest(r.Context(), rdb, tenantID, coordinatorID)
		writeJSON(w, http.StatusOK, map[string]string{"slot_id": slotID})
	}
}

// DELETE /api/v1/dashboard/timetable/slots/{slot_id}
func DeleteTimetableSlot(pool *pgxpool.Pool, rdb *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		slotID := chi.URLParam(r, "slot_id")
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
		// The slot names its unit, and the unit names its department — so the same rule that
		// governs creating a lecture governs removing one. Checked BEFORE the delete, because a
		// deletion that has already happened cannot be refused.
		var slotUnit string
		if err := conn.QueryRow(r.Context(),
			`SELECT unit_id FROM timetable_slots WHERE slot_id=$1::uuid AND tenant_id=$2`,
			slotID, tenantID).Scan(&slotUnit); err != nil {
			writeJSON(w, http.StatusNotFound, errBody("NOT_FOUND", "no such timetable slot"))
			return
		}
		if err := checkTimetableScope(r.Context(), conn, tenantID,
			middleware.GetUserID(r.Context()), middleware.GetRole(r.Context()), slotUnit); err != nil {
			writeTimetableScopeRefusal(w, err)
			return
		}

		if _, err := conn.Exec(r.Context(), `DELETE FROM timetable_slots WHERE slot_id=$1::uuid AND tenant_id=$2`, slotID, tenantID); err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		// A removed lecture must disappear from the phones too, not linger until midnight.
		bustTenantManifests(r.Context(), rdb, tenantID)
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	}
}

// ── Timetable import ─────────────────────────────────────────────────────────

// POST /api/v1/admin/tenants/{tenant_id}/timetable/import  (multipart field "roster")
// Columns: course_id, level, study_year, semester, session_type, unit_id, unit_name,
//
//	day, start_time, duration_minutes (or end_time), room, staff_id (lecturer).
//
// Resolves/creates the offering + unit, links the lecturer if found, upserts slots.
func ImportTimetable(adminPool *pgxpool.Pool, rdb *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := tenantOf(r)
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "expected multipart/form-data"))
			return
		}
		file, _, err := r.FormFile("roster")
		if err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "field 'roster' not found"))
			return
		}
		defer file.Close()
		res, perr := processTimetableCSV(r.Context(), adminPool, tenantID, file)
		if perr != nil {
			writeJSON(w, http.StatusUnprocessableEntity, errBody("CSV_PARSE_ERROR", perr.Error()))
			return
		}
		// A bulk import rewrites the week for every cohort at once. Without this the
		// cached manifest survives until midnight and none of the phones see the new
		// timetable today — indistinguishable, in the room, from the import not working.
		bustTenantManifests(r.Context(), rdb, tenantID)
		writeJSON(w, http.StatusOK, res)
	}
}

func processTimetableCSV(ctx context.Context, pool *pgxpool.Pool, tenantID string, r io.Reader) (*importResult, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("could not read file: %w", err)
	}
	var rows [][]string
	if looksXLSX(data) {
		if rows, err = parseXLSX(data); err != nil {
			return nil, err
		}
	} else {
		cr := csv.NewReader(bytes.NewReader(data))
		cr.TrimLeadingSpace = true
		cr.FieldsPerRecord = -1
		if rows, err = cr.ReadAll(); err != nil {
			return nil, fmt.Errorf("could not parse CSV: %w", err)
		}
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("file has no rows")
	}
	colIdx := make(map[string]int, len(rows[0]))
	for i, h := range rows[0] {
		colIdx[strings.ToLower(strings.TrimSpace(h))] = i
	}
	for _, req := range []string{"course_id", "unit_id", "day", "start_time"} {
		if _, ok := colIdx[req]; !ok {
			return nil, fmt.Errorf("missing required column: %s", req)
		}
	}

	res := &importResult{Errors: []string{}}
	for ln := 1; ln < len(rows); ln++ {
		row := rows[ln]
		get := func(c string) string {
			i, ok := colIdx[c]
			if !ok || i >= len(row) {
				return ""
			}
			return strings.TrimSpace(row[i])
		}
		courseID := get("course_id")
		unitID := get("unit_id")
		day := parseDay(get("day"))
		start, okT := parseClock(get("start_time"))
		if courseID == "" || unitID == "" || day == 0 || !okT {
			res.Skipped++
			res.Errors = append(res.Errors, fmt.Sprintf("line %d: course_id, unit_id, valid day and start_time are required", ln))
			continue
		}
		level := get("level")
		studyYear := atoiSafe(get("study_year"))
		semester := atoiSafe(get("semester"))
		sessionType := get("session_type")
		if sessionType == "" {
			sessionType = "Day"
		}
		// Weekend cohorts only Sat/Sun; Day/Evening only Mon–Fri.
		if weekend := strings.Contains(strings.ToLower(sessionType), "weekend"); weekend != (day == 6 || day == 7) {
			res.Skipped++
			res.Errors = append(res.Errors, fmt.Sprintf("line %d: a %s cohort can't be timetabled on that day", ln, sessionType))
			continue
		}
		dur := atoiSafe(get("duration_minutes"))
		if dur == 0 {
			if end, okE := parseClock(get("end_time")); okE {
				dur = clockDiffMinutes(start, end)
			}
		}
		if dur <= 0 {
			dur = 60
		}

		// Resolve/create the offering for this cohort.
		var offeringID string
		err := pool.QueryRow(ctx, `
			SELECT offering_id::text FROM course_offerings
			WHERE tenant_id=$1 AND course_id=$2 AND session_type=$3 AND study_year=$4 AND semester=$5
			  AND COALESCE(level,'')=COALESCE($6,'')`,
			tenantID, courseID, sessionType, studyYear, semester, level).Scan(&offeringID)
		if err != nil {
			if e := pool.QueryRow(ctx, `
				INSERT INTO course_offerings (tenant_id, course_id, session_type, study_year, semester, level)
				VALUES ($1,$2,$3,$4,$5,NULLIF($6,''))
				RETURNING offering_id::text`,
				tenantID, courseID, sessionType, studyYear, semester, level).Scan(&offeringID); e != nil {
				res.Skipped++
				res.Errors = append(res.Errors, fmt.Sprintf("line %d: offering: %s (does the course exist?)", ln, e.Error()))
				continue
			}
		}

		// Ensure the unit exists (create from unit_name if new).
		_, _ = pool.Exec(ctx, `
			INSERT INTO course_units (unit_id, tenant_id, course_id, name, year, semester, level)
			VALUES ($1,$2,$3,$4,$5,$6,NULLIF($7,''))
			ON CONFLICT (unit_id) DO NOTHING`,
			unitID, tenantID, courseID, orDefault(get("unit_name"), unitID), studyYear, semester, level)

		// Resolve the lecturer by staff_id (optional) and link an assignment.
		var lecturerID string
		if sid := get("staff_id"); sid != "" {
			_ = pool.QueryRow(ctx, `SELECT lecturer_id::text FROM lecturers WHERE tenant_id=$1 AND staff_id=$2`, tenantID, sid).Scan(&lecturerID)
			if lecturerID != "" {
				// course_id + academic_year are NOT NULL on lecturer_assignments.
				_, aerr := pool.Exec(ctx, `
					INSERT INTO lecturer_assignments (tenant_id, lecturer_id, unit_id, course_id, academic_year)
					SELECT $1::uuid, $2::uuid, $3::varchar, $4::varchar,
					       COALESCE((SELECT active_academic_year FROM tenants WHERE tenant_id=$1::uuid),'')
					WHERE NOT EXISTS (
					    SELECT 1 FROM lecturer_assignments
					    WHERE tenant_id=$1::uuid AND lecturer_id=$2::uuid AND unit_id=$3::varchar)`,
					tenantID, lecturerID, unitID, courseID)
				if aerr != nil {
					res.Errors = append(res.Errors, fmt.Sprintf("line %d: lecturer link: %s", ln, aerr.Error()))
				}
			}
		}

		// Clash guard: skip a row that overlaps another unit in the same cohort/day.
		var cU, cS string
		if pool.QueryRow(ctx, `
			SELECT unit_id, to_char(start_time,'HH24:MI') FROM timetable_slots
			WHERE tenant_id=$1 AND offering_id=$2::uuid AND day_of_week=$3
			  AND NOT (unit_id=$4 AND start_time=$5::time)
			  AND start_time < ($5::time + make_interval(mins => $6))
			  AND $5::time    < (start_time + make_interval(mins => duration_minutes))
			LIMIT 1`, tenantID, offeringID, day, unitID, start, dur).Scan(&cU, &cS) == nil {
			res.Skipped++
			res.Errors = append(res.Errors, fmt.Sprintf("line %d: %s clashes with %s at %s (same cohort, same time)", ln, unitID, cU, cS))
			continue
		}

		// The room, against every other department's timetable. Checked here as well as by the
		// constraint so an import reports the clash as a LINE the person can go and fix, rather
		// than losing the row to a database message about a GiST index.
		if room := strings.TrimSpace(get("room")); room != "" {
			var rU, rS string
			if pool.QueryRow(ctx, `
				SELECT unit_id, to_char(start_time,'HH24:MI') FROM timetable_slots
				WHERE tenant_id = $1 AND day_of_week = $2
				  AND btrim(lower(room)) = btrim(lower($3))
				  AND NOT (offering_id = $4::uuid AND unit_id = $5 AND start_time = $6::time)
				  AND start_time < ($6::time + make_interval(mins => $7))
				  AND $6::time   < (start_time + make_interval(mins => duration_minutes))
				LIMIT 1`, tenantID, day, room, offeringID, unitID, start, dur).Scan(&rU, &rS) == nil {
				res.Skipped++
				res.Errors = append(res.Errors, fmt.Sprintf(
					"line %d: room %s is already booked for %s at %s that day (another department's timetable)", ln, room, rU, rS))
				continue
			}
		}

		_, err = pool.Exec(ctx, `
			INSERT INTO timetable_slots (tenant_id, offering_id, unit_id, day_of_week, start_time, duration_minutes, room, lecturer_id, venue_id)
			VALUES ($1,$2::uuid,$3,$4,$5::time,$6,NULLIF($7,''),NULLIF($8,'')::uuid,`+resolveVenueSQL+`)
			ON CONFLICT (offering_id, unit_id, day_of_week, start_time) DO UPDATE
			   SET duration_minutes=EXCLUDED.duration_minutes, room=EXCLUDED.room,
			       venue_id=EXCLUDED.venue_id,
			       lecturer_id=COALESCE(EXCLUDED.lecturer_id, timetable_slots.lecturer_id)`,
			tenantID, offeringID, unitID, day, start, dur, get("room"), lecturerID)
		if err != nil {
			res.Skipped++
			// Two rows of the SAME file can also collide with each other — the pre-check above
			// only sees rows already committed — so this is not merely a race backstop here.
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23P01" &&
				strings.Contains(pgErr.ConstraintName, "room_double_booking") {
				res.Errors = append(res.Errors, fmt.Sprintf(
					"line %d: room %s is double-booked at that time by another row of this file or another department", ln, strings.TrimSpace(get("room"))))
				continue
			}
			res.Errors = append(res.Errors, fmt.Sprintf("line %d: slot: %s", ln, err.Error()))
			continue
		}
		res.Inserted++
	}
	return res, nil
}

// parseDay maps "Monday"/"Mon"/"1" → 1..7 (0 = unknown).
func parseDay(s string) int {
	s = strings.ToLower(strings.TrimSpace(s))
	switch {
	case strings.HasPrefix(s, "mon"):
		return 1
	case strings.HasPrefix(s, "tue"):
		return 2
	case strings.HasPrefix(s, "wed"):
		return 3
	case strings.HasPrefix(s, "thu"):
		return 4
	case strings.HasPrefix(s, "fri"):
		return 5
	case strings.HasPrefix(s, "sat"):
		return 6
	case strings.HasPrefix(s, "sun"):
		return 7
	}
	if n, err := strconv.Atoi(s); err == nil && n >= 1 && n <= 7 {
		return n
	}
	return 0
}

// parseClock accepts "5:00 pm", "5 pm", "17:00", "08:00", "8" → "HH:MM" (24h).
func parseClock(s string) (string, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return "", false
	}
	pm := strings.Contains(s, "pm")
	am := strings.Contains(s, "am")
	s = strings.TrimSpace(strings.NewReplacer("am", "", "pm", "").Replace(s))
	hh, mm := 0, 0
	if strings.Contains(s, ":") {
		parts := strings.SplitN(s, ":", 2)
		hh = atoiSafe(strings.TrimSpace(parts[0]))
		mm = atoiSafe(strings.TrimSpace(parts[1]))
	} else {
		hh = atoiSafe(s)
	}
	if pm && hh < 12 {
		hh += 12
	}
	if am && hh == 12 {
		hh = 0
	}
	if hh < 0 || hh > 23 || mm < 0 || mm > 59 {
		return "", false
	}
	return fmt.Sprintf("%02d:%02d", hh, mm), true
}

func clockDiffMinutes(start, end string) int {
	sp := strings.SplitN(start, ":", 2)
	ep := strings.SplitN(end, ":", 2)
	if len(sp) != 2 || len(ep) != 2 {
		return 0
	}
	s := atoiSafe(sp[0])*60 + atoiSafe(sp[1])
	e := atoiSafe(ep[0])*60 + atoiSafe(ep[1])
	if e <= s {
		return 0
	}
	return e - s
}
