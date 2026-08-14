package handlers

// Which rooms are free, right now.
//
//	GET /api/v1/rooms/free?at=HH:MM&minutes=60   — every room in the institution, free or busy
//
// WHY A COORDINATOR NEEDS THIS. A lecture is timetabled into a room; on the day that room is
// unusable — double-booked, being repainted, taken by an exam, projector dead. The class is
// standing in the corridor and the coordinator has to find somewhere else in the next two minutes.
// What they had was the room list: every room the institution owns, with nothing to say which of
// them has a lecture in it at this moment. So the choice was made by walking the corridor and
// looking through doors, and the room that was found was often one that a class from another
// college was about to walk into.
//
// SO IT SEARCHES THE WHOLE INSTITUTION. Every school, every department, every block — not the
// coordinator's own building. The room next door belongs to another college more often than not,
// and a scope that stopped at the department boundary would hide exactly the rooms that are free.
// The results are GROUPED by building so a coordinator reads "what is free near me" at a glance,
// and each row names the school and department the room belongs to, because borrowing another
// college's room is a thing you should know you are doing.
//
// WHAT "BUSY" MEANS. Two independent claims on a room, and both count:
//
//   1. THE TIMETABLE — a slot for today's weekday whose [start, start+duration) covers the moment
//      asked about. This is the planned truth.
//   2. A LIVE SESSION — a session open in that room right now, including one that is itself a
//      provision. This is the actual truth, and it can disagree with the timetable in both
//      directions: a timetabled lecture that was cancelled leaves the room genuinely free, and a
//      provision leaves a room occupied that the timetable calls empty.
//
// A room is offered as free only when NEITHER claims it. Where they disagree the reason is shown,
// so the coordinator is choosing with the facts rather than being handed a verdict.

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qaat/api-gateway/internal/clock"
	"github.com/qaat/api-gateway/internal/middleware"
)

type freeRoom struct {
	VenueID    string `json:"venue_id"`
	Name       string `json:"name"`
	Building   string `json:"building"`
	Floor      int    `json:"floor"`
	Capacity   int    `json:"capacity"`
	RoomType   string `json:"room_type"`
	School     string `json:"school"`
	Department string `json:"department"`

	Free bool `json:"free"`
	// When busy: what has it, and until when. Empty for a free room.
	OccupiedBy    string `json:"occupied_by"`    // unit name / code
	OccupiedUntil string `json:"occupied_until"` // "HH:MM"
	OccupiedKind  string `json:"occupied_kind"`  // TIMETABLE | LIVE_SESSION
	OccupiedNote  string `json:"occupied_note"`  // e.g. the cohort, or "provision"
}

// FreeRooms answers "what is empty at this moment", across the whole institution.
func FreeRooms(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())

		// Default to NOW, because that is the question being asked 99 times out of 100. `at` is
		// there so a coordinator can look ahead to the hour their next class starts rather than
		// discovering at the door that the room they picked is taken from the half-hour.
		at := strings.TrimSpace(r.URL.Query().Get("at"))
		if !validHHMM(at) {
			at = clock.Now().Format("15:04")
		}
		minutes := 60
		if v, err := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("minutes"))); err == nil && v > 0 && v <= 600 {
			minutes = v
		}
		day := clock.ISOWeekday()

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

		// One query, two claims, LEFT JOINed so a room with neither comes back free. The overlap
		// test is the standard one — each range starts before the other ends — because a room is
		// taken by a lecture that merely OVERLAPS the wanted window, not only by one that starts
		// inside it.
		rows, err := conn.Query(r.Context(), `
			SELECT v.venue_id, v.name, COALESCE(v.building,''), COALESCE(v.floor,0)::int,
			       COALESCE(v.capacity,0)::int, COALESCE(v.room_type,''),
			       COALESCE(s.name,''), COALESCE(d.name,''),
			       COALESCE(tt.label,''), COALESCE(tt.ends,''),
			       COALESCE(live.label,''), COALESCE(live.is_provision, false)
			  FROM venues v
			  LEFT JOIN schools     s ON s.school_id     = v.school_id    
			  LEFT JOIN departments d ON d.department_id = v.department_id

			  -- (1) The timetable's claim on this room for today.
			  LEFT JOIN LATERAL (
			      SELECT COALESCE(NULLIF(cu.name,''), ts.unit_id) ||
			             COALESCE(' · ' || NULLIF(o.session_type,''), '')                AS label,
			             to_char(make_interval(mins => LEAST(
			                        (EXTRACT(EPOCH FROM ts.start_time)/60)::int + COALESCE(ts.duration_minutes,60),
			                        1439)), 'HH24:MI')                                    AS ends
			        FROM timetable_slots ts
			        LEFT JOIN course_units    cu ON cu.unit_id     = ts.unit_id    
			        LEFT JOIN course_offerings o ON o.offering_id  = ts.offering_id AND o.tenant_id  = ts.tenant_id
			       WHERE ts.tenant_id  = v.tenant_id
			         AND ts.venue_id   = v.venue_id
			         AND ts.day_of_week = $2
			         -- OVERLAP IN MINUTES-SINCE-MIDNIGHT, not in time-type arithmetic.
			         --
			         -- "start_time + interval" WRAPS: a three-hour lecture starting at 22:00 ends at
			         -- 01:00, and "is 23:00 before 01:00?" is false — so the room showed as FREE for
			         -- the second half of a lecture that was still running, which is the one answer
			         -- this endpoint must never give. Clamping the end to the end of the day keeps a
			         -- late lecture occupying its room until midnight.
			         AND (EXTRACT(EPOCH FROM ts.start_time)/60)
			               < (EXTRACT(EPOCH FROM $3::time)/60) + $4
			         AND (EXTRACT(EPOCH FROM $3::time)/60)
			               < LEAST((EXTRACT(EPOCH FROM ts.start_time)/60) + COALESCE(ts.duration_minutes,60), 1440)
			       ORDER BY ts.start_time
			       LIMIT 1
			  ) tt ON true

			  -- (2) A session actually running in it now, which the timetable may know nothing about.
			  LEFT JOIN LATERAL (
			      SELECT COALESCE(NULLIF(cu2.name,''), ses.unit_id) AS label,
			             ses.room_is_provision                      AS is_provision
			        FROM sessions ses
			        LEFT JOIN course_units cu2 ON cu2.unit_id = ses.unit_id AND cu2.tenant_id = ses.tenant_id
			       WHERE ses.tenant_id    = v.tenant_id
			         AND ses.venue_id     = v.venue_id
			         AND ses.session_date = $5::date
			         AND ses.session_status IN ('ACTIVE','PENDING_LECTURER')
			       ORDER BY ses.gate_open_time DESC
			       LIMIT 1
			  ) live ON true

			 WHERE v.tenant_id = $1 AND COALESCE(v.is_active, true)
			 ORDER BY COALESCE(v.building,''), v.name`,
			tenantID, day, at, minutes, clock.Today())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		out := []freeRoom{}
		freeCount := 0
		for rows.Next() {
			var x freeRoom
			var ttLabel, ttEnds, liveLabel string
			var liveProvision bool
			if rows.Scan(&x.VenueID, &x.Name, &x.Building, &x.Floor, &x.Capacity, &x.RoomType,
				&x.School, &x.Department, &ttLabel, &ttEnds, &liveLabel, &liveProvision) != nil {
				continue
			}
			switch {
			// A live session outranks the timetable in the description, because it is what is
			// physically in the room — the thing the coordinator would walk in on.
			case liveLabel != "":
				x.Free, x.OccupiedBy, x.OccupiedKind = false, liveLabel, "LIVE_SESSION"
				if liveProvision {
					x.OccupiedNote = "using this room as a provision"
				}
			case ttLabel != "":
				x.Free, x.OccupiedBy, x.OccupiedKind, x.OccupiedUntil = false, ttLabel, "TIMETABLE", ttEnds
			default:
				x.Free = true
				freeCount++
			}
			out = append(out, x)
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"at": at, "minutes": minutes, "day_of_week": day,
			"total": len(out), "free": freeCount,
			"rooms": out,
		})
	}
}
