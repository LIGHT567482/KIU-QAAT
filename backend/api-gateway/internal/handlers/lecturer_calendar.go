package handlers

// The lecturer's UNIT-CENTRIC calendar.
//
//	GET /api/v1/lecturer/calendar?from=YYYY-MM-DD&to=YYYY-MM-DD[&unit_id=]
//
// The organising unit of this calendar is the COURSE UNIT, not the cohort. A lecturer thinks
// "when do I teach CSE 2420?", and a unit shared across the Day and Weekend runs of a programme
// is still one thing they teach. So each calendar entry is a unit session, and the cohorts
// attending it are sub-tags on that entry rather than separate entries.
//
// Entries are generated from the timetable (`timetable_slots`, one row per cohort per weekly
// slot) expanded across the requested range, then enriched with what actually happened: the
// `sessions` row if one was held, and the attendance behind it broken down PER COHORT — which is
// what reveals that Cohort A is at 95% while Cohort B is at 89% on the very same unit.

import (
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qaat/api-gateway/internal/middleware"
)

// GET /api/v1/lecturer/calendar
func LecturerCalendar(adminPool *pgxpool.Pool) http.HandlerFunc {
	// One cohort's slice of a unit session.
	type cohortStat struct {
		OfferingID  string  `json:"offering_id"`
		Label       string  `json:"label"`
		SessionType string  `json:"session_type"`
		Coordinator string  `json:"coordinator_name"`
		Enrolled    int     `json:"enrolled"`
		Present     int     `json:"present"`
		Pct         float64 `json:"pct"`
	}
	type event struct {
		Date      string       `json:"date"` // YYYY-MM-DD
		UnitID    string       `json:"unit_id"`
		UnitName  string       `json:"unit_name"`
		StartTime string       `json:"start_time"`
		Minutes   int          `json:"duration_minutes"`
		Room      string       `json:"room"`
		Held      bool         `json:"held"`   // a session actually ran
		Status    string       `json:"status"` // session_status when held
		Enrolled  int          `json:"enrolled"`
		Present   int          `json:"present"`
		Pct       float64      `json:"pct"`
		Cohorts   []cohortStat `json:"cohorts"`
		// THE LECTURER'S OWN presence at this slot, which is the thing their calendar is actually
		// marking — distinct from Held (a session existed) and from Pct (how many students came).
		// Sourced from lecturer_attendance_logs, which is only written when the lecturer physically
		// STARTed at the coordinator's gate, so it cannot be satisfied by a session merely being
		// opened around them.
		LecturerPresent bool    `json:"lecturer_present"`
		ContactHours    float64 `json:"contact_hours"`
	}
	type unitOpt struct {
		UnitID   string `json:"unit_id"`
		UnitName string `json:"unit_name"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		userID := middleware.GetUserID(r.Context())
		lecturerID, ok := resolveLecturerID(adminPool, r, tenantID, userID)
		if !ok {
			writeJSON(w, http.StatusOK, map[string]interface{}{"events": []any{}, "units": []any{}})
			return
		}

		// Default to the current month when no range is given.
		now := time.Now()
		from := parseDateOr(r.URL.Query().Get("from"), time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC))
		to := parseDateOr(r.URL.Query().Get("to"), from.AddDate(0, 1, -1))
		if to.Before(from) {
			to = from
		}
		// A generous but bounded window — a year of daily expansion is plenty and keeps the
		// response from exploding if a client sends a silly range.
		if to.Sub(from) > 366*24*time.Hour {
			to = from.AddDate(1, 0, 0)
		}
		unitFilter := strings.TrimSpace(r.URL.Query().Get("unit_id"))

		// ── The lecturer's units — the master filter's options ──────────────────
		units := []unitOpt{}
		uRows, err := adminPool.Query(r.Context(), `
			SELECT DISTINCT cu.unit_id, COALESCE(cu.name, cu.unit_id)
			FROM lecturer_assignments la
			JOIN course_units cu ON cu.unit_id = la.unit_id AND cu.tenant_id = la.tenant_id
			WHERE la.tenant_id = $1 AND la.lecturer_id = $2::uuid
			ORDER BY 2`, tenantID, lecturerID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		for uRows.Next() {
			var u unitOpt
			if uRows.Scan(&u.UnitID, &u.UnitName) == nil {
				units = append(units, u)
			}
		}
		uRows.Close()

		// ── Timetable slots for those units, one row per cohort ─────────────────
		type slotRow struct {
			unitID, unitName, offeringID, sessionType, level, intake, coordinator, room, start string
			day, minutes                                                                       int
		}
		slotArgs := []interface{}{tenantID, lecturerID}
		slotWhere := ""
		if unitFilter != "" {
			slotArgs = append(slotArgs, unitFilter)
			slotWhere = " AND ts.unit_id = $3"
		}
		slots := []slotRow{}
		sRows, err := adminPool.Query(r.Context(), `
			SELECT ts.unit_id, COALESCE(cu.name, ts.unit_id),
			       COALESCE(ts.offering_id::text,''), COALESCE(o.session_type,''),
			       COALESCE(c.level,''), COALESCE(se_intake.intake,''),
			       COALESCE(u.full_name,''),
			       COALESCE(NULLIF(ts.room,''), COALESCE(ts.venue_id,'')),
			       COALESCE(to_char(ts.start_time,'HH24:MI'),''),
			       COALESCE(ts.day_of_week,0), COALESCE(ts.duration_minutes,0)
			FROM timetable_slots ts
			LEFT JOIN course_units cu     ON cu.unit_id = ts.unit_id AND cu.tenant_id = ts.tenant_id
			LEFT JOIN course_offerings o  ON o.offering_id = ts.offering_id
			LEFT JOIN courses c           ON c.course_id = o.course_id AND c.tenant_id = o.tenant_id
			LEFT JOIN users u             ON u.user_id::text = o.coordinator_id AND u.tenant_id = o.tenant_id
			LEFT JOIN LATERAL (
			    SELECT MIN(s2.intake_session) AS intake FROM students_extended s2
			    WHERE s2.offering_id = o.offering_id AND s2.tenant_id = ts.tenant_id
			) se_intake ON true
			WHERE ts.tenant_id = $1
			  AND EXISTS (SELECT 1 FROM lecturer_assignments la
			              WHERE la.unit_id = ts.unit_id AND la.tenant_id = ts.tenant_id
			                AND la.lecturer_id = $2::uuid)`+slotWhere+`
			ORDER BY ts.day_of_week, ts.start_time`, slotArgs...)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		for sRows.Next() {
			var sl slotRow
			if sRows.Scan(&sl.unitID, &sl.unitName, &sl.offeringID, &sl.sessionType, &sl.level,
				&sl.intake, &sl.coordinator, &sl.room, &sl.start, &sl.day, &sl.minutes) == nil {
				slots = append(slots, sl)
			}
		}
		sRows.Close()

		// ── Enrolment per cohort, for the denominators ──────────────────────────
		enrolled := map[string]int{}
		eRows, _ := adminPool.Query(r.Context(), `
			SELECT COALESCE(offering_id::text,''), COUNT(*)
			FROM students_extended
			WHERE tenant_id = $1 AND enrollment_status = 'ACTIVE'
			GROUP BY 1`, tenantID)
		if eRows != nil {
			for eRows.Next() {
				var oid string
				var n int
				if eRows.Scan(&oid, &n) == nil {
					enrolled[oid] = n
				}
			}
			eRows.Close()
		}

		// ── What actually happened: sessions + attendance in range ──────────────
		type actual struct {
			status  string
			present int
		}
		held := map[string]actual{} // key: unit|date|offering
		aRows, _ := adminPool.Query(r.Context(), `
			SELECT s.unit_id, s.session_date::text, COALESCE(s.offering_id::text,''),
			       s.session_status::text, COUNT(al.log_id)
			FROM sessions s
			LEFT JOIN attendance_logs al ON al.session_id = s.session_id
			WHERE s.tenant_id = $1 AND s.session_date BETWEEN $3::date AND $4::date
			  AND EXISTS (SELECT 1 FROM lecturer_assignments la
			              WHERE la.unit_id = s.unit_id AND la.tenant_id = s.tenant_id
			                AND la.lecturer_id = $2::uuid)
			GROUP BY s.unit_id, s.session_date, s.offering_id, s.session_status`,
			tenantID, lecturerID, from.Format("2006-01-02"), to.Format("2006-01-02"))
		if aRows != nil {
			for aRows.Next() {
				var unitID, date, off, status string
				var present int
				if aRows.Scan(&unitID, &date, &off, &status, &present) == nil {
					held[unitID+"|"+date+"|"+off] = actual{status, present}
				}
			}
			aRows.Close()
		}

		// ── Was the LECTURER there? ─────────────────────────────────────────────
		// One row per unit+date they actually gated in on. This is what turns a day on the grid
		// from "a class was timetabled" into "I taught it" or "I missed it", and no other table
		// answers that: `sessions` says a coordinator opened a room, not that the lecturer came.
		type presence struct{ hours float64 }
		taught := map[string]presence{} // key: unit|date
		pRows, _ := adminPool.Query(r.Context(), `
			SELECT lal.unit_id, lal.session_date::text, COALESCE(SUM(lal.contact_hours),0)
			FROM lecturer_attendance_logs lal
			WHERE lal.tenant_id = $1 AND lal.lecturer_id = $2
			  AND lal.session_date BETWEEN $3::date AND $4::date
			GROUP BY lal.unit_id, lal.session_date`,
			tenantID, lecturerID, from.Format("2006-01-02"), to.Format("2006-01-02"))
		if pRows != nil {
			for pRows.Next() {
				var unitID, date string
				var hours float64
				if pRows.Scan(&unitID, &date, &hours) == nil {
					taught[unitID+"|"+date] = presence{hours}
				}
			}
			pRows.Close()
		}

		// ── Expand the weekly slots across the range, grouped by (unit, date, time) ──
		byKey := map[string]*event{}
		seenCohort := map[string]bool{}
		order := []string{}
		for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
			// time.Weekday is Sunday=0; timetable_slots uses Monday=1..Sunday=7.
			dow := int(d.Weekday())
			if dow == 0 {
				dow = 7
			}
			date := d.Format("2006-01-02")
			for _, sl := range slots {
				if sl.day != dow {
					continue
				}
				key := sl.unitID + "|" + date + "|" + sl.start
				ev, seen := byKey[key]
				if !seen {
					ev = &event{
						Date: date, UnitID: sl.unitID, UnitName: sl.unitName,
						StartTime: sl.start, Minutes: sl.minutes, Room: sl.room,
						Cohorts: []cohortStat{},
					}
					if p, ok := taught[sl.unitID+"|"+date]; ok {
						ev.LecturerPresent = true
						ev.ContactHours = round1(p.hours)
					}
					byKey[key] = ev
					order = append(order, key)
				}
				// Belt and braces: one cohort is tagged once per block, even if the timetable
				// itself holds a duplicate slot for it.
				if seenCohort[key+"|"+sl.offeringID] {
					continue
				}
				seenCohort[key+"|"+sl.offeringID] = true
				cs := cohortStat{
					OfferingID:  sl.offeringID,
					SessionType: sl.sessionType,
					Coordinator: sl.coordinator,
					Label:       joinNonEmpty(" · ", sl.sessionType, sl.level, sl.intake),
					Enrolled:    enrolled[sl.offeringID],
				}
				if a, ok := held[sl.unitID+"|"+date+"|"+sl.offeringID]; ok {
					ev.Held = true
					ev.Status = a.status
					cs.Present = a.present
				}
				if cs.Enrolled > 0 {
					cs.Pct = round1(float64(cs.Present) / float64(cs.Enrolled) * 100)
				}
				ev.Enrolled += cs.Enrolled
				ev.Present += cs.Present
				ev.Cohorts = append(ev.Cohorts, cs)
			}
		}

		events := make([]event, 0, len(order))
		for _, k := range order {
			ev := byKey[k]
			if ev.Enrolled > 0 {
				ev.Pct = round1(float64(ev.Present) / float64(ev.Enrolled) * 100)
			}
			events = append(events, *ev)
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"from": from.Format("2006-01-02"), "to": to.Format("2006-01-02"),
			"units": units, "events": events,
		})
	}
}

func parseDateOr(s string, def time.Time) time.Time {
	if t, err := time.Parse("2006-01-02", strings.TrimSpace(s)); err == nil {
		return t
	}
	return def
}

func round1(f float64) float64 { return float64(int(f*10+0.5)) / 10 }
