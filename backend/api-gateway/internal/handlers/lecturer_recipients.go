package handlers

// WHO A LECTURER CAN WRITE TO, BY NAME.
//
//	GET /api/v1/lecturer/recipients
//
// A lecturer's message to "the coordinator" went to EVERY coordinator of every cohort of every
// course containing a unit they are assigned to. That is wrong twice over.
//
// It is wrong in reach: one unit is commonly taught to a Day, an Evening, a Weekend and an
// e-learning cohort, each with its own coordinator, and the lecturer very often means exactly one
// of them — "my Saturday class has no projector". Sending that to four coordinators leaves three
// wondering which of their rooms is meant, and the one who should act assuming somebody else will.
//
// And it is wrong in scope: the old query joined offerings on the COURSE, so it reached coordinators
// of cohorts the lecturer does not teach at all. Same defect the roster had, same fix — the cohorts
// a lecturer teaches are the ones with a timetable slot naming them, or a slot naming nobody on a
// unit they hold the assignment to.
//
// So this returns the people themselves: each coordinator WITH the cohort they run, and each
// student WITH the cohort they are in — because "Mukasa" is not an address when three of them are
// enrolled, and a coordinator's name means nothing to a lecturer who knows them as "the Saturday
// one".

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qaat/api-gateway/internal/middleware"
)

// lecturerNiche is the scoping clause shared by every "what does this lecturer actually teach"
// query. It lives here rather than being retyped because the answer has to be the same one on the
// roster, in the timetable and in this picker — a lecturer who can message a coordinator they
// cannot see on their own timetable would be a puzzle nobody could resolve from the screen.
//
// $1 tenant, $2 lecturer_id. Assumes `o` (course_offerings) and `cu` (course_units) are in scope.
const lecturerNiche = `
	AND ( EXISTS (
	        SELECT 1 FROM timetable_slots ts
	         WHERE ts.offering_id = o.offering_id
	           AND ts.unit_id = cu.unit_id
	           AND ( ts.lecturer_id = $2::uuid
	              OR ( ts.lecturer_id IS NULL
	                   AND EXISTS (SELECT 1 FROM lecturer_assignments la2
	                                WHERE la2.tenant_id = ts.tenant_id
	                                  AND la2.unit_id = ts.unit_id
	                                  AND la2.lecturer_id = $2::uuid) ) ) )
	   -- A unit with no timetable slot ANYWHERE has no cohorts to narrow to, and a lecturer who
	   -- cannot reach anybody about it is worse off than one who can reach a few people too many.
	   OR NOT EXISTS (
	        SELECT 1 FROM timetable_slots ts2
	         WHERE ts2.tenant_id = cu.tenant_id AND ts2.unit_id = cu.unit_id) )`

// GET /api/v1/lecturer/recipients
func LecturerRecipients(adminPool *pgxpool.Pool) http.HandlerFunc {
	type coordinator struct {
		UserID string `json:"user_id"` // what target_id must carry
		Name   string `json:"full_name"`
		Code   string `json:"coordinator_code"`
		Cohort string `json:"cohort"` // which class they run — the thing that tells two apart
		Course string `json:"course_name"`
		Units  string `json:"units"` // the lecturer's units in that cohort
	}
	type student struct {
		StudentID string `json:"student_id"` // target_id for a student is their registration number
		Name      string `json:"full_name"`
		Cohort    string `json:"cohort"`
		UnitID    string `json:"unit_id"`
		UnitName  string `json:"unit_name"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		userID := middleware.GetUserID(r.Context())
		lecturerID, ok := resolveLecturerID(adminPool, r, tenantID, userID)
		if !ok {
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"coordinators": []any{}, "students": []any{},
			})
			return
		}

		coords := []coordinator{}
		if rows, err := adminPool.Query(r.Context(), `
			SELECT DISTINCT o.coordinator_id, COALESCE(u.full_name,''), COALESCE(u.coordinator_code,''),
			       COALESCE(NULLIF(CONCAT_WS(' · ', o.session_type,
			                'Yr' || o.study_year, 'Sem' || o.semester, NULLIF(o.intake,'')), ''), ''),
			       COALESCE(c.name, o.course_id),
			       (SELECT string_agg(DISTINCT cu2.unit_id, ', ' ORDER BY cu2.unit_id)
			          FROM lecturer_assignments la2
			          JOIN course_units cu2 ON cu2.unit_id = la2.unit_id AND cu2.tenant_id = la2.tenant_id
			         WHERE la2.tenant_id = o.tenant_id AND la2.lecturer_id = $2::uuid
			           AND cu2.course_id = o.course_id)
			  FROM lecturer_assignments la
			  JOIN course_units cu    ON cu.unit_id = la.unit_id
			  JOIN course_offerings o ON o.course_id = cu.course_id
			  LEFT JOIN courses c     ON c.course_id = o.course_id
			  LEFT JOIN users u       ON u.user_id::text = o.coordinator_id
			 WHERE la.tenant_id = $1 AND la.lecturer_id = $2::uuid
			   AND COALESCE(o.coordinator_id,'') <> ''
			   AND COALESCE(u.is_active, true)`+lecturerNiche+`
			 ORDER BY 5, 4`, tenantID, lecturerID); err == nil {
			for rows.Next() {
				var x coordinator
				if rows.Scan(&x.UserID, &x.Name, &x.Code, &x.Cohort, &x.Course, &x.Units) == nil {
					coords = append(coords, x)
				}
			}
			rows.Close()
		}

		students := []student{}
		if rows, err := adminPool.Query(r.Context(), `
			SELECT DISTINCT s.student_id, s.full_name,
			       COALESCE(NULLIF(CONCAT_WS(' · ', o.session_type,
			                'Yr' || o.study_year, 'Sem' || o.semester, NULLIF(o.intake,'')), ''), ''),
			       cu.unit_id, COALESCE(cu.name, cu.unit_id)
			  FROM lecturer_assignments la
			  JOIN course_units cu     ON cu.unit_id = la.unit_id
			  JOIN course_offerings o  ON o.course_id = cu.course_id
			  JOIN students_extended s ON s.offering_id = o.offering_id
			                          AND s.enrollment_status = 'ACTIVE'
			 WHERE la.tenant_id = $1 AND la.lecturer_id = $2::uuid`+lecturerNiche+`
			 ORDER BY 2 LIMIT 2000`, tenantID, lecturerID); err == nil {
			for rows.Next() {
				var x student
				if rows.Scan(&x.StudentID, &x.Name, &x.Cohort, &x.UnitID, &x.UnitName) == nil {
					students = append(students, x)
				}
			}
			rows.Close()
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"coordinators": coords,
			"students":     students,
		})
	}
}
