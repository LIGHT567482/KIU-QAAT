package handlers

// HOW MUCH OF THE TIMETABLE THE QA ROUND ACTUALLY REACHED.
//
// The patrol round is the institution's only independent record of whether teaching happened: a QA
// monitor walks a corridor and ticks taught / not-taught against the timetable. Every report built
// on those ticks — the teaching record, the no-show follow-ups, the lecturer's own presence
// disputes — inherits a question nobody could answer: how much of the week did the round SEE?
//
// A 90% "taught" rate means one thing if the monitors covered every slot and something else
// entirely if they covered a fifth of them and those were the easy ones on the ground floor. Until
// now the directorate had the numerator and not the denominator.
//
// THE DENOMINATOR IS THE PUBLISHED TIMETABLE, expanded across real dates. A weekly slot is a
// pattern (Tuesday, 14:00, CS201, Block C); the round happens on days. So the slots are crossed
// with the dates in the window, matched on weekday, and each resulting slot-instance is looked for
// in the patrol log. What comes back is three different kinds of silence, which the report keeps
// apart because they call for different action:
//
//	PATROLLED     — a monitor reached it and recorded a finding, taught or not.
//	NOT PATROLLED — the slot existed and nobody came. This is the coverage gap.
//	NOT TAUGHT    — a monitor reached it and found no lecture. This is a teaching gap.
//
// Conflating the last two is the specific mistake this page exists to prevent: a room the round
// never visits looks exactly like a room where nothing is ever taught, and the difference is the
// difference between a monitor's rota problem and a lecturer's.

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qaat/api-gateway/internal/middleware"
)

// GET /api/v1/dashboard/dqa/patrol-coverage?days=30
func PatrolCoverage(pool *pgxpool.Pool) http.HandlerFunc {
	type scopeRow struct {
		Name      string  `json:"name"`
		Expected  int     `json:"expected"`
		Patrolled int     `json:"patrolled"`
		NotTaught int     `json:"not_taught"`
		Pct       float64 `json:"coverage_pct"`
	}
	type gapRow struct {
		UnitID   string `json:"unit_id"`
		UnitName string `json:"unit_name"`
		Room     string `json:"room"`
		School   string `json:"school"`
		Day      int    `json:"day_of_week"`
		Time     string `json:"scheduled_time"`
		Missed   int    `json:"missed"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())

		// A month by default: long enough that a single quiet week does not read as a collapse in
		// coverage, short enough to still be about the round as it is being run now.
		days := 30
		if v := strings.TrimSpace(r.URL.Query().Get("days")); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 365 {
				days = n
			}
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

		// Every slot-instance the round SHOULD have seen, with the tick that found it if there was
		// one. Shared by all three result sets below, so they cannot disagree about the denominator.
		//
		// Dates stop at CURRENT_DATE: a slot later this week has not been missed, it has not
		// happened, and counting it as a gap would make every Monday report look like a failure.
		//
		// The patrol log is keyed by (unit_id, session_date, scheduled_time) — its own unique
		// constraint — so the join is on exactly that, with the slot's TIME rendered the way the
		// round records it.
		const expanded = `
			WITH days AS (
			    SELECT generate_series(CURRENT_DATE - ($2::int - 1), CURRENT_DATE, INTERVAL '1 day')::date AS d
			),
			slots AS (
			    SELECT ts.unit_id,
			           COALESCE(cu.name, ts.unit_id)          AS unit_name,
			           COALESCE(NULLIF(ts.room,''), '—')      AS room,
			           COALESCE(NULLIF(c.school,''), 'Unassigned')     AS school,
			           COALESCE(NULLIF(c.department,''), 'Unassigned') AS department,
			           ts.day_of_week,
			           to_char(ts.start_time, 'HH24:MI')      AS hhmm
			    FROM timetable_slots ts
			    JOIN course_units cu ON cu.unit_id = ts.unit_id
			    LEFT JOIN courses c  ON c.course_id = cu.course_id
			    WHERE ts.tenant_id = $1
			),
			instances AS (
			    SELECT s.*, d.d AS session_date,
			           p.patrol_id, p.taught
			    FROM slots s
			    JOIN days d ON EXTRACT(ISODOW FROM d.d)::int = s.day_of_week
			    LEFT JOIN lecturer_patrol_logs p
			           ON p.tenant_id = $1
			          AND p.unit_id = s.unit_id
			          AND p.session_date = d.d
			          AND COALESCE(p.scheduled_time,'') = s.hhmm
			)`

		// ── Headline ────────────────────────────────────────────────────────
		var expected, patrolled, taught, notTaught int
		err = conn.QueryRow(r.Context(), expanded+`
			SELECT count(*),
			       count(patrol_id),
			       count(*) FILTER (WHERE taught IS TRUE),
			       count(*) FILTER (WHERE taught IS FALSE)
			FROM instances`, tenantID, days).Scan(&expected, &patrolled, &taught, &notTaught)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}

		// ── By college ──────────────────────────────────────────────────────
		bySchool := []scopeRow{}
		if rows, qErr := conn.Query(r.Context(), expanded+`
			SELECT school, count(*), count(patrol_id), count(*) FILTER (WHERE taught IS FALSE)
			FROM instances GROUP BY school ORDER BY count(*) DESC`, tenantID, days); qErr == nil {
			for rows.Next() {
				var s scopeRow
				if rows.Scan(&s.Name, &s.Expected, &s.Patrolled, &s.NotTaught) == nil {
					s.Pct = pct(s.Patrolled, s.Expected)
					bySchool = append(bySchool, s)
				}
			}
			rows.Close()
		}

		// ── The gaps themselves ─────────────────────────────────────────────
		// Ranked by how many times the same weekly slot went unvisited, because that is the
		// actionable shape: one missed Tuesday is a busy day, nine missed Tuesdays is a rota that
		// never goes to Block C after lunch.
		gaps := []gapRow{}
		if rows, qErr := conn.Query(r.Context(), expanded+`
			SELECT unit_id, unit_name, room, school, day_of_week, hhmm, count(*) AS missed
			FROM instances
			WHERE patrol_id IS NULL
			GROUP BY unit_id, unit_name, room, school, day_of_week, hhmm
			ORDER BY missed DESC, unit_name
			LIMIT 50`, tenantID, days); qErr == nil {
			for rows.Next() {
				var g gapRow
				if rows.Scan(&g.UnitID, &g.UnitName, &g.Room, &g.School, &g.Day, &g.Time, &g.Missed) == nil {
					gaps = append(gaps, g)
				}
			}
			rows.Close()
		}

		// ── Who is walking the rounds ───────────────────────────────────────
		// Named because a coverage figure is a question about a rota, and the answer to "why is
		// nothing in Block C patrolled" is usually that one person covers it and was on leave.
		type monitorRow struct {
			Name   string `json:"name"`
			Ticks  int    `json:"ticks"`
			Rooms  int    `json:"rooms"`
			LastAt string `json:"last_at"`
		}
		monitors := []monitorRow{}
		if rows, qErr := conn.Query(r.Context(), `
			SELECT COALESCE(NULLIF(patroller_name,''), COALESCE(patroller_staff_id,'—')),
			       count(*), count(DISTINCT COALESCE(room,'')), max(taken_at)
			FROM lecturer_patrol_logs
			WHERE tenant_id = $1 AND session_date >= CURRENT_DATE - ($2::int - 1)
			GROUP BY 1 ORDER BY count(*) DESC LIMIT 25`, tenantID, days); qErr == nil {
			for rows.Next() {
				var m monitorRow
				var last time.Time
				if rows.Scan(&m.Name, &m.Ticks, &m.Rooms, &last) == nil {
					m.LastAt = last.Format(time.RFC3339)
					monitors = append(monitors, m)
				}
			}
			rows.Close()
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"days":         days,
			"expected":     expected,
			"patrolled":    patrolled,
			"coverage_pct": pct(patrolled, expected),
			"taught":       taught,
			"not_taught":   notTaught,
			"by_school":    bySchool,
			"gaps":         gaps,
			"monitors":     monitors,
		})
	}
}

// pct is a percentage that refuses to divide by zero. An institution with no timetable published
// yet has no coverage to report, and 0% would read as a failure of the round rather than an absence
// of anything to walk.
func pct(part, whole int) float64 {
	if whole <= 0 {
		return 0
	}
	return float64(part) * 100.0 / float64(whole)
}
