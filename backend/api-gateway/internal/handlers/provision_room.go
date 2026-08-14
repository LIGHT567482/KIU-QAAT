package handlers

// Telling everyone that a lecture moved room.
//
// A provision room is a substitution made minutes before a class, and the whole value of recording
// it is that the people who would otherwise go to the WRONG room find out in time. So this is not
// an audit entry written for later reading — it is a message, sent the moment the session opens.
//
// WHO IS TOLD, AND WHY EACH ONE:
//
//   QA MONITORS   The reason this exists. A monitor's round is built from the timetable, so they
//                 walk to the timetabled room, find it empty, and file "not taught" against a
//                 lecturer who is teaching thirty metres away. The monitor recorded what they saw
//                 and is not at fault; the lecturer has no way to answer it. Redirecting the round
//                 before the visit is the only fix that prevents the false record rather than
//                 arguing about it afterwards.
//
//   THE LECTURER  They are about to be observed in a room the system did not expect. If a tick has
//                 already been filed against them they need to know why.
//
//   QA OFFICE     Substitutions are the symptom of an estate problem — a room unusable every
//                 Tuesday is one repair nobody has done, not eight unrelated incidents — and it is
//                 only visible to whoever sees all of them.
//
// Best-effort throughout: a session that cannot be announced is still a session, and failing the
// open because a notification did not send would take a working class off the air for a courtesy.

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// announceProvisionRoom tells the monitors, the lecturer and the QA office that this session is
// running somewhere other than its timetabled room.
func announceProvisionRoom(ctx context.Context, conn *pgxpool.Conn, tenantID, sessionID,
	unitID, venueID, note, coordinatorName string) {

	// The room's own name, and the room the timetable expected — the message is useless without
	// both, since "moved to LR7" means nothing to someone who does not know where it moved FROM.
	var roomName, plannedRoom, unitName string
	_ = conn.QueryRow(ctx, `
		SELECT COALESCE(v.name, $3),
		       COALESCE((SELECT COALESCE(NULLIF(ts.room,''), v2.name, ts.venue_id)
		                   FROM timetable_slots ts
		                   LEFT JOIN venues v2 ON v2.venue_id = ts.venue_id AND v2.tenant_id = ts.tenant_id
		                  WHERE ts.tenant_id = $1 AND ts.unit_id = $2
		                  ORDER BY ts.day_of_week LIMIT 1), ''),
		       COALESCE((SELECT cu.name FROM course_units cu
		                  WHERE cu.unit_id = $2 AND cu.tenant_id = $1), $2)
		  FROM venues v
		 WHERE v.tenant_id = $1 AND v.venue_id = $3`,
		tenantID, unitID, venueID).Scan(&roomName, &plannedRoom, &unitName)
	if roomName == "" {
		roomName = venueID
	}
	if unitName == "" {
		unitName = unitID
	}

	subject := "Room change — " + unitName + " is in " + roomName
	body := unitName + " is being taught in " + roomName + " today."
	if plannedRoom != "" && plannedRoom != roomName {
		body += " The timetable says " + plannedRoom + "."
	}
	if note != "" {
		body += "\n\nReason given: " + note
	}
	if coordinatorName != "" {
		body += "\n\nRecorded by " + coordinatorName + " when the session was opened."
	}
	body += "\n\nIf you are visiting this lecture, go to " + roomName + " — a visit to the " +
		"timetabled room will find it empty and the record will say the lecture did not happen."

	// One notification, many recipients: the same text reaches everyone who needs it, and each
	// reader dismisses their own copy without affecting anyone else's.
	var nid string
	if conn.QueryRow(ctx, `
		INSERT INTO app_notifications (tenant_id, sender_id, sender_name, sender_role, audience, unit_id, subject, body)
		VALUES ($1, NULL, 'QAAT', 'SYSTEM', 'DIRECT', $2, $3, $4)
		RETURNING notification_id::text`,
		tenantID, unitID, subject, body).Scan(&nid) != nil || nid == "" {
		return
	}

	// Every active monitor, the unit's lecturer, and the QA office — resolved in one statement so
	// a change of mind about the audience is a change in one place.
	_, _ = conn.Exec(ctx, `
		INSERT INTO notification_recipients (notification_id, tenant_id, recipient_user_id)
		SELECT $1, $2, u.user_id
		  FROM users u
		 WHERE u.tenant_id = $2 AND COALESCE(u.is_active, true)
		   AND ( u.role IN ('QA_PATROLLER','QA_OFFICER','DQA_DIRECTOR')
		      OR u.user_id IN (SELECT l.user_id
		                         FROM lecturers l
		                         JOIN lecturer_assignments la ON la.lecturer_id = l.lecturer_id
		                                                     AND la.tenant_id   = l.tenant_id
		                        WHERE l.tenant_id = $2 AND la.unit_id = $3 AND l.user_id IS NOT NULL) )
		ON CONFLICT DO NOTHING`, nid, tenantID, unitID)
}
