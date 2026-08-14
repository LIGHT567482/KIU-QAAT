package handlers

// The lecture that was taught but never timetabled.
//
//	GET  /api/v1/patrol/reference  — the lists the manual form picks from
//	POST /api/v1/patrol/manual     — file an observation with no timetable slot behind it
//
// WHY THIS EXISTS. A monitor's round is generated from the timetable: search a unit or a lecturer,
// get that lecture's slot, tick it. So every observation has to be ABOUT a slot — and a great many
// real lectures are not on the timetable. A unit added after the schedule was locked, a make-up
// hour agreed in a corridor, a class moved into a free room, a visiting lecturer covering a week.
// All taught, all attended, all invisible to quality assurance.
//
// What a monitor could do about it before was nothing, or tick whichever slot looked closest —
// which files a real observation under the wrong lecture. That is worse than the silence, because
// it is wrong in a way that reads as right.
//
// PICK OR TYPE, EVERY TIME. Each field offers the known list and accepts a typed value, and that
// combination is the whole design. Offering only the list would refuse to record the exact lectures
// this is for — the unit that is not in the curriculum yet, the lecturer hired last week. Offering
// only free text would produce a pile of near-miss spellings that no report can group. So the list
// is the default and the typed value is always allowed, and the record says which one happened
// (a resolved unit carries its course and college; a typed one carries what the monitor wrote).

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qaat/api-gateway/internal/clock"
	"github.com/qaat/api-gateway/internal/middleware"
)

// PatrolReference serves the four dropdowns the manual form needs, in one round trip.
//
// One call, not four: the monitor opens this form in a corridor on a phone that may be about to
// lose signal, and a form that half-loads is a form that cannot be filled in. It is small enough
// (an institution's rooms, units, lecturers and colleges) to cache on the handset for the round.
func PatrolReference(pool *pgxpool.Pool) http.HandlerFunc {
	type room struct {
		VenueID  string `json:"venue_id"`
		Name     string `json:"name"`
		Building string `json:"building"`
	}
	type unit struct {
		UnitID     string `json:"unit_id"`
		Name       string `json:"unit_name"`
		CourseID   string `json:"course_id"`
		Department string `json:"department"`
		School     string `json:"school"`
		// So picking a unit can fill in the class/group without the monitor typing it.
		ClassGroup string `json:"class_group"`
	}
	type lecturer struct {
		StaffID    string `json:"staff_id"`
		FullName   string `json:"full_name"`
		Department string `json:"department"`
	}

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

		rooms := []room{}
		if rows, e := conn.Query(r.Context(), `
			SELECT v.venue_id, v.name, COALESCE(v.building,'')
			  FROM venues v
			 WHERE v.tenant_id = $1 AND COALESCE(v.is_active, true)
			 ORDER BY v.venue_id`, tenantID); e == nil {
			for rows.Next() {
				var x room
				if rows.Scan(&x.VenueID, &x.Name, &x.Building) == nil {
					rooms = append(rooms, x)
				}
			}
			rows.Close()
		}

		units := []unit{}
		if rows, e := conn.Query(r.Context(), `
			SELECT cu.unit_id, COALESCE(cu.name,''), COALESCE(cu.course_id,''),
			       COALESCE(c.department,''), COALESCE(c.school,''),
			       COALESCE(NULLIF(cu.year::text,'') || ':' || NULLIF(cu.semester::text,''), '')
			  FROM course_units cu
			  LEFT JOIN courses c ON c.course_id = cu.course_id
			 WHERE cu.tenant_id = $1
			 ORDER BY cu.unit_id`, tenantID); e == nil {
			for rows.Next() {
				var x unit
				if rows.Scan(&x.UnitID, &x.Name, &x.CourseID, &x.Department, &x.School, &x.ClassGroup) == nil {
					units = append(units, x)
				}
			}
			rows.Close()
		}

		lecturers := []lecturer{}
		if rows, e := conn.Query(r.Context(), `
			SELECT COALESCE(l.staff_id,''), COALESCE(l.full_name,''), COALESCE(l.department,'')
			  FROM lecturers l
			 WHERE l.tenant_id = $1 AND COALESCE(l.staff_id,'') <> ''
			 ORDER BY l.full_name`, tenantID); e == nil {
			for rows.Next() {
				var x lecturer
				if rows.Scan(&x.StaffID, &x.FullName, &x.Department) == nil {
					lecturers = append(lecturers, x)
				}
			}
			rows.Close()
		}

		schools := []string{}
		if rows, e := conn.Query(r.Context(),
			`SELECT name FROM schools WHERE tenant_id = $1 ORDER BY name`, tenantID); e == nil {
			for rows.Next() {
				var n string
				if rows.Scan(&n) == nil && strings.TrimSpace(n) != "" {
					schools = append(schools, n)
				}
			}
			rows.Close()
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"rooms": rooms, "units": units, "lecturers": lecturers, "schools": schools,
		})
	}
}

// manualEntry is one lecture a monitor found being taught with nothing on the timetable to tick.
type manualEntry struct {
	// Either RoomID (a venue picked from the list) or Room (typed). A picked room resolves to its
	// name for display and keeps its id for the room-level reports.
	RoomID string `json:"room_id"`
	Room   string `json:"room"`
	// Either UnitID (picked) or UnitName (typed). A picked unit fills in course, class/group and
	// college from the curriculum; a typed one keeps exactly what the monitor wrote.
	UnitID   string `json:"unit_id"`
	UnitName string `json:"unit_name"`
	// Either LecturerStaffID (picked or searched) or LecturerName (typed).
	LecturerStaffID string `json:"lecturer_staff_id"`
	LecturerName    string `json:"lecturer_name"`

	ClassGroup      string `json:"class_group"`
	School          string `json:"school"`
	Department      string `json:"department"`
	StudentsCounted int    `json:"students_counted"`

	// THE OTHER UNITS THIS ONE HOUR ALSO COVERS. One class, one lecturer, one room — and two or
	// three unit codes, because each programme codes the same taught content differently. Picking
	// only one leaves every student on the other codes with a lecture QA never saw. See migration
	// 089. Each entry is pick-or-type on the same terms as the primary unit.
	AlsoUnits []manualExtraUnit `json:"also_units"`

	SessionDate string `json:"session_date"` // YYYY-MM-DD, defaults to today
	TimeOfDay   string `json:"time_of_day"`  // HH:MM the lecture BEGAN
	EndTime     string `json:"end_time"`     // HH:MM it was due to end
	Taught      bool   `json:"taught"`
	Remarks     string `json:"remarks"`

	IsCompensation bool `json:"is_compensation"`
	// The date AND TIME of the lecture being made good — required whenever IsCompensation is set.
	// RFC3339 or "YYYY-MM-DD HH:MM"; free text is refused rather than stored, because a
	// compensation nobody can match to a missed lecture is a claim, not a record.
	CompensationForAt string `json:"compensation_for_at"`
	// Older clients sent a bare date here. Still read, so a handset that has not been updated can
	// file a compensation, but only as a fallback when compensation_for_at is absent.
	CompensationFor string `json:"compensation_for"`
}

// manualExtraUnit is one additional course unit the same lecture also delivered.
type manualExtraUnit struct {
	UnitID     string `json:"unit_id"`   // picked from the curriculum
	UnitName   string `json:"unit_name"` // or typed
	ClassGroup string `json:"class_group"`
	School     string `json:"school"`
	Department string `json:"department"`
}

// PatrolManualEntry files an observation that has no timetable slot behind it.
//
// POST /api/v1/patrol/manual
func PatrolManualEntry(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		userID := middleware.GetUserID(r.Context())

		var req manualEntry
		if err := decodeJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "malformed body"))
			return
		}
		req.UnitID = strings.TrimSpace(req.UnitID)
		req.UnitName = strings.TrimSpace(req.UnitName)
		req.LecturerStaffID = strings.TrimSpace(req.LecturerStaffID)
		req.LecturerName = strings.TrimSpace(req.LecturerName)

		// The two facts without which the record means nothing: WHICH lecture, and WHOSE.
		// Everything else has a defensible blank — a monitor who cannot read the room number off
		// the door still saw a lecture happen, and refusing the whole record to get a tidy room
		// column would lose the observation entirely.
		if req.UnitID == "" && req.UnitName == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST",
				"choose a course unit or type its name — the record has to say which lecture this was"))
			return
		}
		if req.LecturerStaffID == "" && req.LecturerName == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST",
				"choose a lecturer or type their name — the record has to say whose lecture this was"))
			return
		}
		if req.StudentsCounted < 0 {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "students cannot be negative"))
			return
		}

		// A COMPENSATION MUST SAY WHICH LECTURE IT COMPENSATES, to the hour.
		//
		// This was optional and free text, which made it decorative: "last week" and "" are both
		// unmatchable, so a compensation could be claimed against a missed lecture that was never
		// missed, and a genuinely missed Tuesday with two timetabled hours could not be told which
		// of the two had been made good. Refused rather than coerced — a stored value nobody can
		// match is worse than being asked again, because it reads as evidence.
		var compensationFor *time.Time
		if req.IsCompensation {
			at, ok := parseCompensationAt(req.CompensationForAt, req.CompensationFor)
			if !ok {
				writeJSON(w, http.StatusBadRequest, errBody("COMPENSATION_FOR_REQUIRED",
					"say which lecture this makes good — its date and start time. A compensation "+
						"that cannot be matched to a missed lecture cannot be counted as making it good."))
				return
			}
			if at.After(clock.Now().Add(2 * time.Minute)) {
				writeJSON(w, http.StatusBadRequest, errBody("COMPENSATION_IN_FUTURE",
					"the lecture being made good is in the future — check the date and time"))
				return
			}
			compensationFor = &at
		}

		// The server's clock decides the day, on the same rule as the round: a past date is kept
		// (an entry filed offline yesterday is legitimate), a future one is a wrong device clock.
		sessionDate := clock.Today()
		if d := strings.TrimSpace(req.SessionDate); d != "" {
			if parsed, err := clock.ParseDate(d); err == nil {
				days := int(clock.Now().Truncate(24*time.Hour).Sub(parsed.Truncate(24*time.Hour)).Hours() / 24)
				if days >= 0 && days <= 7 {
					sessionDate = d
				}
			}
		}
		// The OBSERVED time doubles as the slot key. Without it every manual entry for one unit on
		// one day would collide on ux_patrol_logs_slot and overwrite the last — two real lectures
		// an hour apart in different rooms, recorded as one.
		observedAt := strings.TrimSpace(req.TimeOfDay)
		if !validHHMM(observedAt) {
			observedAt = clock.Now().Format("15:04")
		}
		// WHEN IT ENDS, not just when it started. A monitor standing in the room knows the span the
		// class is running for, and the end is what the hour is worth in contact time. Kept blank
		// rather than guessed when it is not given or does not follow the start — an invented end
		// would be indistinguishable from an observed one.
		endTime := strings.TrimSpace(req.EndTime)
		if !validHHMM(endTime) || !endsAfter(observedAt, endTime) {
			endTime = ""
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
		if err := checkPatrolDevice(r, conn, tenantID, userID); err != nil {
			writeDeviceRefusal(w)
			return
		}

		// ── MANUAL IS A FALLBACK, AND THE SERVER IS WHERE THAT HOLDS ────────────────────────
		//
		// The round is search-first on purpose: the monitor looks a lecture up, gets its slot, and
		// ticks it — so a tick is the consequence of having gone to a room, and it is anchored to a
		// timetabled lecture that everyone else can see. Manual entry has neither property. It is a
		// typed record with nothing behind it, and it exists only for the lecture the timetable does
		// not know about.
		//
		// Offered as a PEER to the round it would quietly become the default: it is fewer taps than
		// searching, it never comes back empty, and it never disagrees with you. Within a term the
		// institution would be running on typed records and the timetable would stop being the
		// thing attendance is measured against — which is the whole apparatus.
		//
		// The app hides the option until a search returns nothing, but a UI rule is a suggestion:
		// the endpoint is reachable directly and the handset is the least trusted thing here. So the
		// rule lives where it cannot be bypassed. If the lecture the monitor is describing IS on the
		// round at this hour, they are sent back to it BY NAME.
		//
		// The test is deliberately narrow — same unit, same weekday, overlapping the observed time —
		// so the cases manual entry exists for still pass: a unit not in the curriculum, a lecturer
		// covering off-timetable, and a COMPENSATION, which by definition runs at an hour its unit
		// is not timetabled for.
		if req.UnitID != "" {
			var slotStart, slotRoom string
			if conn.QueryRow(r.Context(), `
				SELECT to_char(ts.start_time,'HH24:MI'),
				       COALESCE(NULLIF(ts.room,''), v.name, ts.venue_id, '')
				  FROM timetable_slots ts
				  LEFT JOIN venues v ON v.venue_id = ts.venue_id
				 WHERE ts.tenant_id = $1 AND ts.unit_id = $2
				   AND ts.day_of_week = $3
				   AND (EXTRACT(EPOCH FROM ts.start_time)/60) < (EXTRACT(EPOCH FROM $4::time)/60) + 1
				   AND (EXTRACT(EPOCH FROM $4::time)/60)
				         < LEAST((EXTRACT(EPOCH FROM ts.start_time)/60) + COALESCE(ts.duration_minutes,60), 1440)
				 ORDER BY ts.start_time LIMIT 1`,
				tenantID, req.UnitID, clock.ISOWeekday(), observedAt).Scan(&slotStart, &slotRoom) == nil {
				where := ""
				if slotRoom != "" {
					where = " in " + slotRoom
				}
				writeJSON(w, http.StatusConflict, map[string]interface{}{
					"error": "ON_THE_ROUND",
					"message": "This lecture is on your round — it is timetabled for " + slotStart +
						where + ". Search for it and tick it there, so the record is tied to the " +
						"timetabled lecture. Manual entry is only for lectures the timetable does not have.",
					"unit_id":        req.UnitID,
					"scheduled_time": slotStart,
					"room":           slotRoom,
				})
				return
			}
		}

		// Resolve what CAN be resolved, and keep what was typed when it cannot. A picked unit
		// supplies its course and college, so the monitor is not asked to retype what the
		// curriculum already knows — and a typed unit is stored exactly as written rather than
		// being silently matched to something that merely looks similar.
		unitID, unitName, courseCode := req.UnitID, req.UnitName, ""
		school, classGroup := strings.TrimSpace(req.School), strings.TrimSpace(req.ClassGroup)
		department := strings.TrimSpace(req.Department)
		if unitID != "" {
			var n, c, dept, sch, cg string
			if conn.QueryRow(r.Context(), `
				SELECT COALESCE(cu.name,''), COALESCE(cu.course_id,''), COALESCE(c.department,''),
				       COALESCE(c.school,''),
				       COALESCE(NULLIF(cu.year::text,'') || ':' || NULLIF(cu.semester::text,''), '')
				  FROM course_units cu
				  LEFT JOIN courses c ON c.course_id = cu.course_id
				 WHERE cu.unit_id = $1 AND cu.tenant_id = $2`,
				unitID, tenantID).Scan(&n, &c, &dept, &sch, &cg) == nil {
				if unitName == "" {
					unitName = n
				}
				courseCode = c
				if school == "" {
					school = sch // inherited from the course unit, as asked
				}
				// The DEPARTMENT matters more than the college when one lecture carries several unit
				// codes: those codes usually share a college and differ by department, so a record
				// that keeps only the college cannot say who owes whom the teaching.
				if department == "" {
					department = dept
				}
				if classGroup == "" {
					classGroup = cg
				}
			}
		} else {
			// A typed unit still needs a key. The name is what the monitor has, so it is what the
			// record is filed under — truncated to the column, and never blank.
			unitID = unitName
			if len(unitID) > 50 {
				unitID = unitID[:50]
			}
		}

		lecturerKey, lecturerName := req.LecturerStaffID, req.LecturerName
		if lecturerKey != "" {
			var n, dept string
			if conn.QueryRow(r.Context(),
				`SELECT COALESCE(full_name,''), COALESCE(department,'') FROM lecturers
				  WHERE tenant_id = $1 AND btrim(lower(staff_id)) = btrim(lower($2)) LIMIT 1`,
				tenantID, lecturerKey).Scan(&n, &dept) == nil && n != "" && lecturerName == "" {
				lecturerName = n
			}
		} else {
			// Typed name with no staff id: file it under the name so the reports still group the
			// same person's lectures together, rather than under an empty key that groups everyone.
			lecturerKey = lecturerName
			if len(lecturerKey) > 50 {
				lecturerKey = lecturerKey[:50]
			}
		}

		room := strings.TrimSpace(req.Room)
		if id := strings.TrimSpace(req.RoomID); id != "" {
			var n string
			if conn.QueryRow(r.Context(),
				`SELECT name FROM venues WHERE tenant_id = $1 AND venue_id = $2`, tenantID, id).Scan(&n) == nil {
				if room == "" {
					room = n
				}
			} else if room == "" {
				room = id
			}
		}

		var monitorName, monitorStaffID string
		_ = conn.QueryRow(r.Context(),
			`SELECT COALESCE(full_name,''), COALESCE(staff_id,'') FROM users WHERE user_id = $1::uuid`,
			userID).Scan(&monitorName, &monitorStaffID)

		var patrolID string
		err = conn.QueryRow(r.Context(), `
			INSERT INTO lecturer_patrol_logs
			  (tenant_id, unit_id, unit_name, course_code, lecturer_id, lecturer_name, room,
			   session_date, scheduled_time, taught, patroller_id, patroller_name, patroller_staff_id,
			   taken_at, patroller_device_hash, entry_method, remarks,
			   is_compensation, compensation_for, compensation_for_at,
			   students_counted, class_group, school, department, end_time)
			VALUES ($1,$2,NULLIF($3,''),NULLIF($4,''),$5,NULLIF($6,''),NULLIF($7,''),
			        $8::date, $9, $10, $11::uuid, $12, $13,
			        now(), $14, 'MANUAL', NULLIF($15,''),
			        $16, ($17::timestamptz)::date, $17::timestamptz,
			        NULLIF($18,0), NULLIF($19,''), NULLIF($20,''), NULLIF($21,''), NULLIF($22,''))
			ON CONFLICT (tenant_id, unit_id, session_date, scheduled_time,
			             COALESCE(offering_id, '00000000-0000-0000-0000-000000000000'::uuid))
			DO UPDATE SET taught = EXCLUDED.taught,
			              room = EXCLUDED.room,
			              lecturer_id = EXCLUDED.lecturer_id,
			              lecturer_name = EXCLUDED.lecturer_name,
			              patroller_id = EXCLUDED.patroller_id,
			              patroller_name = EXCLUDED.patroller_name,
			              patroller_staff_id = EXCLUDED.patroller_staff_id,
			              taken_at = EXCLUDED.taken_at,
			              remarks = EXCLUDED.remarks,
			              is_compensation = EXCLUDED.is_compensation,
			              compensation_for = EXCLUDED.compensation_for,
			              compensation_for_at = EXCLUDED.compensation_for_at,
			              students_counted = EXCLUDED.students_counted,
			              class_group = EXCLUDED.class_group,
			              school = EXCLUDED.school,
			              department = EXCLUDED.department,
			              end_time = EXCLUDED.end_time,
			              entry_method = 'MANUAL'
			RETURNING patrol_id::text`,
			tenantID, unitID, unitName, courseCode, lecturerKey, lecturerName, room,
			sessionDate, observedAt, req.Taught, userID, monitorName, monitorStaffID,
			deviceFingerprint(r), req.Remarks,
			req.IsCompensation, compensationFor,
			req.StudentsCounted, classGroup, school, department, endTime,
		).Scan(&patrolID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}

		// ── The other units this same hour covered ──────────────────────────────────────────
		//
		// Replaced, not merged. Re-filing the same lecture (the ON CONFLICT above updates rather
		// than inserting) has to be able to REMOVE a unit the monitor added by mistake; merging
		// would make the first mistake permanent.
		_, _ = conn.Exec(r.Context(), `DELETE FROM monitor_log_units WHERE patrol_id = $1::uuid`, patrolID)
		extras := writeExtraUnits(r, conn, tenantID, patrolID, unitID, school, req.AlsoUnits)

		// Every college this one lecture belongs to, named ONCE. Two unit codes in the same college
		// is the common case and repeating its name would read as two colleges; two codes in
		// different colleges is the case a reader has to be able to see at a glance.
		colleges := dedupeNonEmpty(append([]string{school}, collect(extras, func(e extraUnitOut) string { return e.School })...))
		departments := dedupeNonEmpty(append([]string{department}, collect(extras, func(e extraUnitOut) string { return e.Department })...))

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"patrol_id": patrolID, "status": "RECORDED", "entry_method": "MANUAL",
			"unit_id": unitID, "unit_name": unitName, "room": room,
			"lecturer_name": lecturerName, "class_group": classGroup, "school": school,
			"department":       department,
			"students_counted": req.StudentsCounted, "session_date": sessionDate,
			"time_of_day": observedAt, "end_time": endTime,
			"also_units": extras,
			// The de-duplicated rollups, so a screen showing "this lecture belongs to…" does not
			// have to work them out and risk working them out differently from the next screen.
			"schools":     colleges,
			"departments": departments,
			"compensation_for_at": func() string {
				if compensationFor == nil {
					return ""
				}
				return compensationFor.Format(time.RFC3339)
			}(),
		})
	}
}

// itoaOrBlank renders a counted headcount, leaving a genuine "not counted" empty rather than
// printing 0 — which would read as "nobody came".
func itoaOrBlank(n int) string {
	if n <= 0 {
		return ""
	}
	return strconv.Itoa(n)
}

// extraUnitOut is one additional unit as it was stored, echoed back so the phone can show exactly
// what the record now says rather than what it hoped it would say.
type extraUnitOut struct {
	UnitID     string `json:"unit_id"`
	UnitName   string `json:"unit_name"`
	CourseCode string `json:"course_code"`
	ClassGroup string `json:"class_group"`
	School     string `json:"school"`
	Department string `json:"department"`
	Resolved   bool   `json:"resolved"`
}

// writeExtraUnits stores the other course units one observed lecture also delivered.
//
// Each is resolved against the curriculum where it can be — which is what supplies its college and
// department, the two facts that distinguish the codes from one another — and kept exactly as typed
// where it cannot. A unit that repeats the primary one is dropped rather than stored twice: it is
// the obvious mis-tap, and counting one lecture twice against one unit would overstate delivery.
func writeExtraUnits(r *http.Request, conn *pgxpool.Conn, tenantID, patrolID, primaryUnit, primarySchool string,
	in []manualExtraUnit) []extraUnitOut {

	out := make([]extraUnitOut, 0, len(in))
	seen := map[string]bool{strings.ToLower(strings.TrimSpace(primaryUnit)): true}

	for _, e := range in {
		x := extraUnitOut{
			UnitID:     strings.TrimSpace(e.UnitID),
			UnitName:   strings.TrimSpace(e.UnitName),
			ClassGroup: strings.TrimSpace(e.ClassGroup),
			School:     strings.TrimSpace(e.School),
			Department: strings.TrimSpace(e.Department),
		}
		if x.UnitID == "" && x.UnitName == "" {
			continue
		}
		if x.UnitID != "" {
			var n, c, dept, sch, cg string
			if conn.QueryRow(r.Context(), `
				SELECT COALESCE(cu.name,''), COALESCE(cu.course_id,''), COALESCE(c.department,''),
				       COALESCE(c.school,''),
				       COALESCE(NULLIF(cu.year::text,'') || ':' || NULLIF(cu.semester::text,''), '')
				  FROM course_units cu
				  LEFT JOIN courses c ON c.course_id = cu.course_id
				 WHERE cu.unit_id = $1 AND cu.tenant_id = $2`,
				x.UnitID, tenantID).Scan(&n, &c, &dept, &sch, &cg) == nil {
				x.Resolved = true
				if x.UnitName == "" {
					x.UnitName = n
				}
				x.CourseCode = c
				if x.School == "" {
					x.School = sch
				}
				if x.Department == "" {
					x.Department = dept
				}
				if x.ClassGroup == "" {
					x.ClassGroup = cg
				}
			}
		} else {
			// Same rule as the primary unit: a typed unit is filed under what was written.
			x.UnitID = x.UnitName
			if len(x.UnitID) > 50 {
				x.UnitID = x.UnitID[:50]
			}
		}
		// A second code in the same college inherits it rather than being left blank — the college
		// is a property of the lecture as much as of the unit, and a blank would read as unknown.
		if x.School == "" {
			x.School = primarySchool
		}

		key := strings.ToLower(x.UnitID)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true

		if _, err := conn.Exec(r.Context(), `
			INSERT INTO monitor_log_units
			  (patrol_id, unit_id, unit_name, course_code, class_group, school, department, resolved)
			VALUES ($1::uuid, $2, NULLIF($3,''), NULLIF($4,''), NULLIF($5,''), NULLIF($6,''), NULLIF($7,''), $8)
			ON CONFLICT (patrol_id, unit_id) DO NOTHING`,
			patrolID, x.UnitID, x.UnitName, x.CourseCode, x.ClassGroup,
			x.School, x.Department, x.Resolved); err != nil {
			continue
		}
		out = append(out, x)
	}
	return out
}

// parseCompensationAt reads the date and time of the lecture being made good.
//
// Three shapes are accepted because three clients send them: RFC3339 from a date-time picker,
// "YYYY-MM-DD HH:MM" from a form, and a bare date from a handset that predates this field — the
// last one at midnight, which is honest about being imprecise rather than inventing an hour.
// Anything else is refused: free text here is what made the old column decorative.
func parseCompensationAt(at, dateOnly string) (time.Time, bool) {
	at = strings.TrimSpace(at)
	loc := clock.Now().Location()
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04", "2006-01-02 15:04", "2006-01-02T15:04:05"} {
		if t, err := time.ParseInLocation(layout, at, loc); err == nil {
			return t, true
		}
	}
	if d := strings.TrimSpace(dateOnly); d != "" {
		if t, err := time.ParseInLocation("2006-01-02", d, loc); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// endsAfter reports whether end is later in the day than start. A class that "ends" before it began
// is a typo, and storing it would put a negative hour into contact-time reporting.
func endsAfter(start, end string) bool {
	s, okS := hhmmMinutes(start)
	e, okE := hhmmMinutes(end)
	return okS && okE && e > s
}

// dedupeNonEmpty keeps the first occurrence of each value, case-insensitively, dropping blanks.
// "One college named once" is the requirement; two codes in the same college must not make it look
// like two colleges.
func dedupeNonEmpty(in []string) []string {
	out := make([]string, 0, len(in))
	seen := map[string]bool{}
	for _, v := range in {
		v = strings.TrimSpace(v)
		k := strings.ToLower(v)
		if v == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, v)
	}
	return out
}

func collect[T any](in []T, f func(T) string) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		out = append(out, f(v))
	}
	return out
}
