package handlers

// "I WAS THERE AND I TAUGHT IT" — offered as the lecture ends, not when the accusation arrives.
//
//	GET /api/v1/lecturer/unrecorded
//
// A lecture ends. Nothing in the system says the lecturer was in the room: they arrived to a
// coordinator whose phone had no battery, the gate QR would not scan, the hotspot was down, or the
// coordinator simply never opened a session. Later — sometimes a day later, because a monitor's
// round syncs from a handset that may have been offline — a "not taught" tick lands against them.
//
// By then the lecturer is answering "where were you at eleven on Tuesday" from memory, against a
// record made at the time by somebody else. That asymmetry is the injustice, and no amount of
// appeal process fixes it after the fact: one account is contemporaneous and the other is a
// recollection produced under challenge.
//
// So the lecturer is asked FIRST. The moment a timetabled lecture's time has elapsed with no gate
// record against it, it appears here, and they can put their own account on file while they still
// remember and usually before any tick exists to argue with. What they file is the existing
// presence claim (migration 081) — a timestamped, located statement — which is exactly the kind of
// evidence the tick is.
//
// THIS DOES NOT MARK ANYBODY PRESENT. A claim is the lecturer's account, filed beside the monitor's,
// for QA to weigh. Letting a lecturer clear their own absence would make the whole apparatus
// decorative; letting them speak at the same time as the person accusing them is simply fair.

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qaat/api-gateway/internal/clock"
	"github.com/qaat/api-gateway/internal/middleware"
)

// GET /api/v1/lecturer/unrecorded
func LecturerUnrecorded(adminPool *pgxpool.Pool) http.HandlerFunc {
	type lecture struct {
		UnitID    string `json:"unit_id"`
		UnitName  string `json:"unit_name"`
		Cohort    string `json:"cohort"`
		Room      string `json:"room"`
		Date      string `json:"session_date"`
		StartTime string `json:"start_time"`
		EndTime   string `json:"end_time"`
		Minutes   int    `json:"duration_minutes"`

		// A session existed but the lecturer never gated in — a materially different situation
		// from no session at all, and the one where "the QR would not scan" is the likely story.
		SessionOpened bool `json:"session_opened"`
		// A monitor has already filed a verdict. When they have and it is "not taught", this is no
		// longer a courtesy: it is a reply to something on the record.
		MonitorVerdict string `json:"monitor_verdict"` // "" | TAUGHT | NOT_TAUGHT
		// The key a claim is filed against, so the phone and the notification agree on which
		// lecture is being talked about.
		Ref string `json:"ref"` // unit|date|HH:MM
		// Whether the lecturer has already put an account on file for this lecture.
		Claimed bool `json:"claimed"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		userID := middleware.GetUserID(r.Context())
		lecturerID, ok := resolveLecturerID(adminPool, r, tenantID, userID)
		if !ok {
			writeJSON(w, http.StatusOK, map[string]interface{}{"lectures": []any{}})
			return
		}

		// How far back to look. Two days by default: long enough that a lecture taught on Friday
		// evening can still be accounted for on Monday morning, short enough that the list stays a
		// list of things to do rather than a backlog nobody reads.
		days := 2
		if d := r.URL.Query().Get("days"); d != "" {
			if n := atoiSafe(d); n >= 1 && n <= 14 {
				days = n
			}
		}
		from := clock.Now().AddDate(0, 0, -(days - 1)).Format("2006-01-02")

		// ELAPSED, on the institution's clock. A lecture still running is not unrecorded — it is in
		// progress, and offering "you missed this" to somebody currently teaching it would be both
		// wrong and insulting. The comparison is in minutes since midnight, with the end clamped to
		// the day, for the same reason the free-room search is: start + interval wraps past
		// midnight and a 22:00 three-hour lecture would otherwise read as having ended at 01:00.
		rows, err := adminPool.Query(r.Context(), `
			WITH days AS (
			    SELECT d::date AS the_date, EXTRACT(ISODOW FROM d)::int AS dow
			      FROM generate_series($3::date, $4::date, interval '1 day') d
			)
			SELECT ts.unit_id, COALESCE(cu.name, ts.unit_id),
			       COALESCE(NULLIF(CONCAT_WS(' · ', c.name, o.session_type,
			                'Yr' || o.study_year, 'Sem' || o.semester, NULLIF(o.intake,'')), ''), ''),
			       COALESCE(NULLIF(ts.room,''), v.name, ts.venue_id, ''),
			       days.the_date::text,
			       to_char(ts.start_time,'HH24:MI'),
			       to_char((ts.start_time + make_interval(mins => LEAST(COALESCE(ts.duration_minutes,60),
			                                                           1439 - (EXTRACT(EPOCH FROM ts.start_time)/60)::int))),
			               'HH24:MI'),
			       COALESCE(ts.duration_minutes,60),
			       EXISTS (SELECT 1 FROM sessions s
			                WHERE s.unit_id = ts.unit_id
			                  AND s.session_date = days.the_date),
			       COALESCE((SELECT CASE WHEN pl.taught THEN 'TAUGHT' ELSE 'NOT_TAUGHT' END
			                   FROM lecturer_patrol_logs pl
			                  WHERE pl.unit_id = ts.unit_id
			                    AND pl.session_date = days.the_date
			                  ORDER BY pl.taken_at DESC LIMIT 1), ''),
			       EXISTS (SELECT 1 FROM lecturer_presence_claims pc
			                WHERE pc.lecturer_user_id = $7::uuid
			                  AND pc.unit_id = ts.unit_id AND pc.session_date = days.the_date)
			  FROM days
			  JOIN timetable_slots ts ON ts.day_of_week = days.dow AND ts.tenant_id = $1
			  JOIN course_units cu ON cu.unit_id = ts.unit_id
			  LEFT JOIN venues v ON v.venue_id = ts.venue_id
			  LEFT JOIN course_offerings o ON o.offering_id = ts.offering_id
			  LEFT JOIN courses c ON c.course_id = o.course_id
			 WHERE ( ts.lecturer_id = $2::uuid
			      OR ( ts.lecturer_id IS NULL
			           AND EXISTS (SELECT 1 FROM lecturer_assignments la
			                        WHERE la.unit_id = ts.unit_id
			                          AND la.lecturer_id = $2::uuid) ) )
			   -- An online cohort has no room and is started by the lecturer themselves, so there is
			   -- nothing here for them to account for that they did not already control.
			   AND COALESCE(o.delivery_mode,'IN_PERSON') <> 'ONLINE'
			   -- The lecture's time has passed.
			   AND ( days.the_date < $5::date
			      OR ( days.the_date = $5::date
			           AND (EXTRACT(EPOCH FROM ts.start_time)/60)
			                 + LEAST(COALESCE(ts.duration_minutes,60), 1440) <= $6::int ) )
			   -- And nothing says the lecturer was there.
			   AND NOT EXISTS (
			         SELECT 1 FROM lecturer_attendance_logs lal
			          WHERE lal.unit_id = ts.unit_id
			            AND lal.session_date = days.the_date
			            AND lal.lecturer_scanned_at IS NOT NULL )
			 ORDER BY days.the_date DESC, ts.start_time DESC
			 LIMIT 60`,
			tenantID, lecturerID, from, clock.Today(), clock.Today(), minutesToday(), userID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		out := make([]lecture, 0)
		for rows.Next() {
			var x lecture
			if rows.Scan(&x.UnitID, &x.UnitName, &x.Cohort, &x.Room, &x.Date, &x.StartTime,
				&x.EndTime, &x.Minutes, &x.SessionOpened, &x.MonitorVerdict, &x.Claimed) != nil {
				continue
			}
			x.Ref = x.UnitID + "|" + x.Date + "|" + x.StartTime
			out = append(out, x)
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"lectures": out,
			"today":    clock.Today(),
		})
	}
}

// minutesToday is the institution's local time as minutes since midnight — the clock a lecture is
// judged to have finished against. Postgres runs in UTC, so comparing there would put Kampala three
// hours in the past and offer a lecturer a lecture they are still delivering.
func minutesToday() int {
	n := clock.Now()
	return n.Hour()*60 + n.Minute()
}
