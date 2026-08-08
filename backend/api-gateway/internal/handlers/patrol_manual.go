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
		VenueID string `json:"venue_id"`
		Name    string `json:"name"`
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
			  LEFT JOIN courses c ON c.course_id = cu.course_id AND c.tenant_id = cu.tenant_id
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
	StudentsCounted int    `json:"students_counted"`

	SessionDate string `json:"session_date"` // YYYY-MM-DD, defaults to today
	TimeOfDay   string `json:"time_of_day"`  // HH:MM the lecture was observed, defaults to now
	Taught      bool   `json:"taught"`
	Remarks     string `json:"remarks"`

	IsCompensation  bool   `json:"is_compensation"`
	CompensationFor string `json:"compensation_for"`
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

		// Resolve what CAN be resolved, and keep what was typed when it cannot. A picked unit
		// supplies its course and college, so the monitor is not asked to retype what the
		// curriculum already knows — and a typed unit is stored exactly as written rather than
		// being silently matched to something that merely looks similar.
		unitID, unitName, courseCode, school, classGroup := req.UnitID, req.UnitName, "", strings.TrimSpace(req.School), strings.TrimSpace(req.ClassGroup)
		if unitID != "" {
			var n, c, dept, sch, cg string
			if conn.QueryRow(r.Context(), `
				SELECT COALESCE(cu.name,''), COALESCE(cu.course_id,''), COALESCE(c.department,''),
				       COALESCE(c.school,''),
				       COALESCE(NULLIF(cu.year::text,'') || ':' || NULLIF(cu.semester::text,''), '')
				  FROM course_units cu
				  LEFT JOIN courses c ON c.course_id = cu.course_id AND c.tenant_id = cu.tenant_id
				 WHERE cu.unit_id = $1 AND cu.tenant_id = $2`,
				unitID, tenantID).Scan(&n, &c, &dept, &sch, &cg) == nil {
				if unitName == "" {
					unitName = n
				}
				courseCode = c
				if school == "" {
					school = sch // inherited from the course unit, as asked
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
			   is_compensation, compensation_for, students_counted, class_group, school)
			VALUES ($1,$2,NULLIF($3,''),NULLIF($4,''),$5,NULLIF($6,''),NULLIF($7,''),
			        $8::date, $9, $10, $11::uuid, $12, $13,
			        now(), $14, 'MANUAL', NULLIF($15,''),
			        $16, NULLIF($17,'')::date, NULLIF($18,0), NULLIF($19,''), NULLIF($20,''))
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
			              students_counted = EXCLUDED.students_counted,
			              class_group = EXCLUDED.class_group,
			              school = EXCLUDED.school,
			              entry_method = 'MANUAL'
			RETURNING patrol_id::text`,
			tenantID, unitID, unitName, courseCode, lecturerKey, lecturerName, room,
			sessionDate, observedAt, req.Taught, userID, monitorName, monitorStaffID,
			deviceFingerprint(r), req.Remarks,
			req.IsCompensation, strings.TrimSpace(req.CompensationFor),
			req.StudentsCounted, classGroup, school,
		).Scan(&patrolID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"patrol_id": patrolID, "status": "RECORDED", "entry_method": "MANUAL",
			"unit_id": unitID, "unit_name": unitName, "room": room,
			"lecturer_name": lecturerName, "class_group": classGroup, "school": school,
			"students_counted": req.StudentsCounted, "session_date": sessionDate,
			"time_of_day": observedAt,
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
