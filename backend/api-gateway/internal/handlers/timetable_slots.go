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
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
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
			JOIN courses c ON c.course_id = o.course_id AND c.tenant_id = o.tenant_id
			LEFT JOIN users u ON u.user_id::text = o.coordinator_id AND u.tenant_id = o.tenant_id
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

		sRows, err := conn.Query(r.Context(), `
			SELECT s.slot_id::text, s.offering_id::text, s.unit_id, COALESCE(cu.name, s.unit_id),
			       s.day_of_week, to_char(s.start_time,'HH24:MI'), s.duration_minutes,
			       COALESCE(s.room,''), COALESCE(s.venue_id,''),
			       COALESCE(s.lecturer_id::text,''), COALESCE(l.full_name,'')
			FROM timetable_slots s
			LEFT JOIN course_units cu ON cu.unit_id = s.unit_id AND cu.tenant_id = s.tenant_id
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
			sRows.Scan(&s.SlotID, &s.OfferingID, &s.UnitID, &s.UnitName, &s.DayOfWeek, &s.StartTime, &s.Duration, &s.Room, &s.RoomCode, &s.LecturerID, &s.LecturerName) //nolint:errcheck
			slots = append(slots, s)
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"offerings": offerings, "slots": slots})
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
			       lecturer_id      = EXCLUDED.lecturer_id,
			       venue_id         = EXCLUDED.venue_id
			RETURNING slot_id::text`,
			tenantID, req.OfferingID, req.UnitID, req.DayOfWeek, req.StartTime, req.Duration, req.Room, req.LecturerID).Scan(&slotID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		bustCoordinatorManifest(r.Context(), rdb, tenantID, coordinatorID)
		writeJSON(w, http.StatusOK, map[string]string{"slot_id": slotID})
	}
}

// DELETE /api/v1/dashboard/timetable/slots/{slot_id}
func DeleteTimetableSlot(pool *pgxpool.Pool) http.HandlerFunc {
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
		if _, err := conn.Exec(r.Context(), `DELETE FROM timetable_slots WHERE slot_id=$1::uuid AND tenant_id=$2`, slotID, tenantID); err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	}
}

// ── Timetable import ─────────────────────────────────────────────────────────

// POST /api/v1/admin/tenants/{tenant_id}/timetable/import  (multipart field "roster")
// Columns: course_id, level, study_year, semester, session_type, unit_id, unit_name,
//          day, start_time, duration_minutes (or end_time), room, staff_id (lecturer).
// Resolves/creates the offering + unit, links the lecturer if found, upserts slots.
func ImportTimetable(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")
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
