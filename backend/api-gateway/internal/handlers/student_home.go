package handlers

// The student app's Home tab: who they are, the units they are taking, and their weekly
// timetable — everything the landing screen greets them with.
//
//	GET /api/v1/student/home   (STUDENT)
//
// Scoped to the student's OWN cohort (`students_extended.offering_id`), so a Weekend student sees
// the Weekend timetable for a unit shared with the Day cohort, never the Day one. See the cohort
// isolation note in AGENTS.md.

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qaat/api-gateway/internal/middleware"
)

// GET /api/v1/student/home
func StudentHome(adminPool *pgxpool.Pool) http.HandlerFunc {
	type slot struct {
		UnitID    string `json:"unit_id"`
		UnitName  string `json:"unit_name"`
		DayOfWeek int    `json:"day_of_week"` // 1 = Monday
		StartTime string `json:"start_time"`  // "HH:MM"
		Minutes   int    `json:"duration_minutes"`
		Room      string `json:"room"`
		Lecturer  string `json:"lecturer_name"`
	}
	type unit struct {
		UnitID   string `json:"unit_id"`
		UnitName string `json:"unit_name"`
		Lecturer string `json:"lecturer_name"`
		Year     int    `json:"year"`
		Semester int    `json:"semester"`
		Current  bool   `json:"current"` // belongs to the year/semester the student is sitting now
	}

	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		userID := middleware.GetUserID(r.Context())

		// Resolve the student from their signed-in account.
		var studentID, fullName, email, course, level, intake, sessionType, academicYear string
		var year, semester int
		var offeringID *string
		err := adminPool.QueryRow(r.Context(), `
			SELECT se.student_id, COALESCE(se.full_name,''), COALESCE(se.email,''),
			       COALESCE(c.name,''), COALESCE(c.level,''), COALESCE(se.intake_session,''),
			       COALESCE(o.session_type,''), COALESCE(se.academic_year,''),
			       COALESCE(se.current_year,0), COALESCE(se.semester,0), se.offering_id::text
			FROM users u
			JOIN students_extended se ON se.email = u.email AND se.tenant_id = u.tenant_id
			LEFT JOIN courses c           ON c.course_id  = se.course_id  AND c.tenant_id = se.tenant_id
			LEFT JOIN course_offerings o  ON o.offering_id = se.offering_id
			WHERE u.user_id = $1::uuid AND u.tenant_id = $2`, userID, tenantID).
			Scan(&studentID, &fullName, &email, &course, &level, &intake, &sessionType,
				&academicYear, &year, &semester, &offeringID)
		if err != nil {
			writeJSON(w, http.StatusNotFound, errBody("NOT_FOUND", "no student profile for this account"))
			return
		}

		// Every unit on this student's programme, each tagged with the year/semester it belongs to
		// and whether it is the one they are sitting now.
		//
		// This deliberately does NOT filter to the current year/semester. It used to, and the
		// result was a blank Units list for any student whose year/semester combination had no
		// units tagged against it yet — a student on a programme with thirteen units was told they
		// had none. The student's own roadmap is not a secret, and seeing next semester's units
		// greyed behind this semester's is far more useful than seeing nothing. The `current` flag
		// carries the distinction the filter used to enforce, and the client renders on it.
		units := []unit{}
		uRows, err := adminPool.Query(r.Context(), `
			SELECT DISTINCT cu.unit_id, cu.name, COALESCE(l.full_name,''),
			       COALESCE(cu.year,0), COALESCE(cu.semester,0)
			FROM course_units cu
			LEFT JOIN lecturer_assignments la ON la.unit_id = cu.unit_id AND la.tenant_id = cu.tenant_id
			LEFT JOIN lecturers l ON l.lecturer_id = la.lecturer_id AND l.tenant_id = la.tenant_id
			WHERE cu.tenant_id = $1
			  AND cu.course_id = (SELECT course_id FROM students_extended WHERE student_id = $2 AND tenant_id = $1)
			ORDER BY 4, 5, cu.name`, tenantID, studentID)
		if err == nil {
			for uRows.Next() {
				var u unit
				if uRows.Scan(&u.UnitID, &u.UnitName, &u.Lecturer, &u.Year, &u.Semester) == nil {
					u.Current = (year == 0 || u.Year == year) && (semester == 0 || u.Semester == semester)
					units = append(units, u)
				}
			}
			uRows.Close()
		}

		// The weekly grid for THIS cohort only.
		//
		// Two things went wrong here before. First, the lecturer join compared `lecturer_id::text`
		// to `ts.lecturer_id`, which is itself a uuid — Postgres has no `text = uuid` operator, so
		// the whole query failed with a type error that this handler then swallowed, leaving every
		// student with an empty timetable and no clue why. Second, `timetable_slots.lecturer_id` is
		// nullable and in practice usually null: a slot says when and where, and who teaches it
		// comes from the unit's assignment. So resolve the name the way the patrol manifest does —
		// the slot's own lecturer when set, otherwise the unit's most recent assignment.
		slots := []slot{}
		if offeringID != nil && *offeringID != "" {
			sRows, err := adminPool.Query(r.Context(), `
				SELECT ts.unit_id, COALESCE(cu.name, ts.unit_id),
				       COALESCE(ts.day_of_week,0), COALESCE(to_char(ts.start_time,'HH24:MI'),''),
				       COALESCE(ts.duration_minutes,0),
				       COALESCE(NULLIF(ts.room,''), COALESCE(ts.venue_id,'')),
				       COALESCE(lec.full_name,'')
				FROM timetable_slots ts
				LEFT JOIN course_units cu ON cu.unit_id = ts.unit_id AND cu.tenant_id = ts.tenant_id
				LEFT JOIN LATERAL (
				    SELECT l.full_name
				    FROM lecturers l
				    WHERE l.tenant_id = ts.tenant_id
				      AND ( l.lecturer_id = ts.lecturer_id
				         OR ( ts.lecturer_id IS NULL AND l.lecturer_id = (
				               SELECT la.lecturer_id FROM lecturer_assignments la
				               WHERE la.unit_id = ts.unit_id AND la.tenant_id = ts.tenant_id
				               ORDER BY la.academic_year DESC LIMIT 1) ) )
				    LIMIT 1
				) lec ON true
				WHERE ts.tenant_id = $1 AND ts.offering_id = $2::uuid
				ORDER BY ts.day_of_week, ts.start_time`, tenantID, *offeringID)
			if err == nil {
				for sRows.Next() {
					var s slot
					if sRows.Scan(&s.UnitID, &s.UnitName, &s.DayOfWeek, &s.StartTime,
						&s.Minutes, &s.Room, &s.Lecturer) == nil {
						slots = append(slots, s)
					}
				}
				sRows.Close()
			}
		}

		cohort := joinNonEmpty(" · ", course, sessionType, level, "Year "+itoa(year), "Sem "+itoa(semester), intake)

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"student_id":    studentID,
			"full_name":     fullName,
			"email":         email,
			"course":        course,
			"level":         level,
			"intake":        intake,
			"session_type":  sessionType,
			"academic_year": academicYear,
			"year":          year,
			"semester":      semester,
			"cohort":        cohort,
			"units":         units,
			"timetable":     slots,
		})
	}
}

// joinNonEmpty joins the parts that actually have content, with sep.
func joinNonEmpty(sep string, parts ...string) string {
	out := ""
	for _, p := range parts {
		if p == "" || p == "Year 0" || p == "Sem 0" {
			continue
		}
		if out != "" {
			out += sep
		}
		out += p
	}
	return out
}
