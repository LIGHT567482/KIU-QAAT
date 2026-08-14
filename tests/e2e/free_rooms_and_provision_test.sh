#!/usr/bin/env bash
#
# The room that was not on the timetable — end to end against a running stack.
#
# THE SITUATION THIS EXISTS FOR. A lecture is timetabled into a room; on the day that room is
# unusable and the class still has to happen. The coordinator finds an empty one and runs it there.
# Before this, the system knew nothing about it, and the consequence was not untidiness: the QA
# monitor walked to the TIMETABLED room, found it empty, and filed "not taught" against a lecturer
# who was teaching thirty metres away — an accusation nobody could answer.
#
# So the assertions below are mostly about the CONSEQUENCES, not the plumbing:
#   · a room with a lecture in it is not offered as free, whether the claim is the timetable's or
#     a session actually running
#   · the search crosses every school, department and block — the free room is usually somebody
#     else's
#   · choosing one records it as a provision ON THE ATTENDANCE LOG, where the dispute is settled
#   · and the monitors are told BEFORE the visit, which is the only timing that helps
#
# Usage:  ./free_rooms_and_provision_test.sh        (against https://localhost:8443)
# Seeds and removes its own FRT- fixtures via the postgres container.
set -uo pipefail
BASE="${1:-https://localhost:8443}"; PG="${PG_CONTAINER:-infra-postgres-1}"; pass=0; fail=0
check(){ if [ "$2" = "$3" ]; then echo "   ✓ $1"; pass=$((pass+1)); else echo "   ✗ $1 (got '$2' want '$3')"; fail=$((fail+1)); fi; }
has(){ if grep -qF -- "$2" <<<"$3"; then echo "   ✓ $1"; pass=$((pass+1)); else echo "   ✗ $1 — '$2' not in: ${3:0:400}"; fail=$((fail+1)); fi; }
hasnt(){ if grep -qF -- "$2" <<<"$3"; then echo "   ✗ $1 — found '$2'"; fail=$((fail+1)); else echo "   ✓ $1"; pass=$((pass+1)); fi; }
sql(){ docker exec -i "$PG" psql -U qaat -d qaat -tAc "$1"; }

cleanup(){ docker exec -i "$PG" psql -U qaat -d qaat -q >/dev/null 2>&1 <<'SQL'
DELETE FROM notification_recipients WHERE notification_id IN (SELECT notification_id FROM app_notifications WHERE unit_id LIKE 'FRT-%');
DELETE FROM app_notifications   WHERE unit_id LIKE 'FRT-%';
DELETE FROM lecturer_attendance_logs WHERE unit_id LIKE 'FRT-%';
DELETE FROM attendance_logs     WHERE session_id IN (SELECT session_id FROM sessions WHERE unit_id LIKE 'FRT-%');
DELETE FROM sessions            WHERE unit_id LIKE 'FRT-%';
DELETE FROM timetable_slots     WHERE unit_id LIKE 'FRT-%';
DELETE FROM offering_unit_schedules WHERE unit_id LIKE 'FRT-%';
DELETE FROM lecturer_assignments WHERE unit_id LIKE 'FRT-%';
DELETE FROM course_units        WHERE unit_id LIKE 'FRT-%';
DELETE FROM course_offerings    WHERE course_id LIKE 'FRT-%';
DELETE FROM courses             WHERE course_id LIKE 'FRT-%';
DELETE FROM lecturers           WHERE staff_id LIKE 'FRT-%';
DELETE FROM venues              WHERE venue_id LIKE 'FRT-%';
DELETE FROM departments         WHERE name LIKE 'FRTEST%';
DELETE FROM schools             WHERE name LIKE 'FRTEST%';
DELETE FROM users               WHERE email LIKE 'frt.%';
SQL
}
# OPEN THE SESSION WINDOW for the duration, and put it back exactly as it was.
#
# Opening a session is refused outside the institution's lecture hours, so run in the evening this
# whole test would say nothing about rooms — it would only re-prove that 19:00 is not a teaching
# hour. Saved and restored rather than left wide, because the window is a real policy and a test
# that quietly disables one is worse than a test that skips.
SAVED_WINDOW=$(docker exec -i "$PG" psql -U qaat -d qaat -tAc \
  "SELECT to_char(session_window_start,'HH24:MI')||'|'||to_char(session_window_end,'HH24:MI')||'|'||session_active_days::text
     FROM tenants WHERE tenant_id <> '00000000-0000-0000-0000-000000000000' ORDER BY created_at LIMIT 1")
restore_window(){
  [ -n "${SAVED_WINDOW:-}" ] || return 0
  local st=${SAVED_WINDOW%%|*}; local rest=${SAVED_WINDOW#*|}
  local en=${rest%%|*}; local days=${rest#*|}
  docker exec -i "$PG" psql -U qaat -d qaat -q -c \
    "UPDATE tenants SET session_window_start='$st'::time, session_window_end='$en'::time,
                        session_active_days='$days'
      WHERE tenant_id = (SELECT tenant_id FROM tenants WHERE tenant_id <> '00000000-0000-0000-0000-000000000000' ORDER BY created_at LIMIT 1)" >/dev/null 2>&1
}
open_window(){
  docker exec -i "$PG" psql -U qaat -d qaat -q -c \
    "UPDATE tenants SET session_window_start='00:00'::time, session_window_end='23:59'::time,
                        session_active_days=ARRAY[1,2,3,4,5,6,7]::smallint[]
      WHERE tenant_id = (SELECT tenant_id FROM tenants WHERE tenant_id <> '00000000-0000-0000-0000-000000000000' ORDER BY created_at LIMIT 1)" >/dev/null 2>&1
}
trap 'restore_window; cleanup' EXIT
cleanup
open_window

TEN=$(sql "SELECT tenant_id FROM tenants WHERE tenant_id <> '00000000-0000-0000-0000-000000000000' ORDER BY created_at LIMIT 1")
OFF='bb444444-0000-4000-8000-0000000bb001'
INST_NOW=$(sql "SELECT to_char(now() AT TIME ZONE 'Africa/Kampala','HH24:MI')")
DOW=$(sql "SELECT EXTRACT(ISODOW FROM (now() AT TIME ZONE 'Africa/Kampala'))::int")
INST_DATE=$(sql "SELECT (now() AT TIME ZONE 'Africa/Kampala')::date")
# Starts 30 minutes ago and runs 90, so it certainly covers "now" and certainly does NOT wrap past
# midnight — the wrap is its own case and is not what these assertions are about.
BUSY_FROM=$(sql "SELECT to_char((now() AT TIME ZONE 'Africa/Kampala') - interval '30 minutes','HH24:MI')")

docker exec -i "$PG" psql -U qaat -d qaat -q -v ON_ERROR_STOP=1 <<SQL
CREATE EXTENSION IF NOT EXISTS pgcrypto;
-- TWO colleges and two departments, because the point is that the free room is usually somebody
-- else's: a search that stopped at the coordinator's own department would hide it.
INSERT INTO schools (tenant_id, name, abbreviation) VALUES
 ('$TEN','FRTEST College of Computing','FRTC'),
 ('$TEN','FRTEST College of Nursing','FRTN');
INSERT INTO departments (tenant_id, school_id, name)
 SELECT '$TEN', school_id, 'FRTEST Software' FROM schools WHERE name='FRTEST College of Computing';
INSERT INTO departments (tenant_id, school_id, name)
 SELECT '$TEN', school_id, 'FRTEST Midwifery' FROM schools WHERE name='FRTEST College of Nursing';
-- Three rooms in two different blocks, owned by the two different colleges.
INSERT INTO venues (venue_id, tenant_id, name, building, capacity, school_id)
 SELECT 'FRT-BUSY-TT','$TEN','FRTEST Room Timetabled','Block A',80, school_id FROM schools WHERE name='FRTEST College of Computing';
INSERT INTO venues (venue_id, tenant_id, name, building, capacity, school_id)
 SELECT 'FRT-FREE-N','$TEN','FRTEST Room Free Nursing','Block N',120, school_id FROM schools WHERE name='FRTEST College of Nursing';
INSERT INTO venues (venue_id, tenant_id, name, building, capacity, school_id)
 SELECT 'FRT-BUSY-LIVE','$TEN','FRTEST Room Live','Block A',60, school_id FROM schools WHERE name='FRTEST College of Computing';
INSERT INTO courses (course_id, tenant_id, name, department, school)
 VALUES ('FRT-CS','$TEN','FRTEST Computing','FRTEST Software','FRTEST College of Computing');
INSERT INTO course_units (unit_id, tenant_id, course_id, name, year, semester)
 VALUES ('FRT-UNIT','$TEN','FRT-CS','FRTEST Displaced Unit',2,1),
        ('FRT-OTHER','$TEN','FRT-CS','FRTEST Other Unit',2,1);
INSERT INTO course_offerings (offering_id, tenant_id, course_id, session_type, study_year, semester, coordinator_id)
 VALUES ('$OFF','$TEN','FRT-CS','Day',2,1,NULL);
INSERT INTO users (tenant_id, email, password_hash, role, full_name, is_active, staff_id, force_password_change) VALUES
 ('$TEN','frt.coord@kiu.ac.ug',  crypt('CooPass12345', gen_salt('bf',10)),'COORDINATOR', 'FRT Coordinator', true,'FRT-CRD',false),
 ('$TEN','frt.coord2@kiu.ac.ug', crypt('CooPass12345', gen_salt('bf',10)),'COORDINATOR', 'FRT Coordinator Two', true,'FRT-CRD2',false),
 ('$TEN','frt.monitor@kiu.ac.ug',crypt('MonPass12345', gen_salt('bf',10)),'QA_PATROLLER','FRT Monitor',     true,'FRT-MON',false),
 ('$TEN','frt.qa@kiu.ac.ug',     crypt('QaPass12345',  gen_salt('bf',10)),'QA_OFFICER',  'FRT QA',          true,'FRT-QA',false),
 ('$TEN','frt.lect@kiu.ac.ug',   crypt('LecPass12345', gen_salt('bf',10)),'LECTURER',    'FRT Lecturer',    true,'FRT-LEC',false);
INSERT INTO lecturers (tenant_id, full_name, email, staff_id, user_id)
 SELECT '$TEN','FRT Lecturer','frt.lect@kiu.ac.ug','FRT-LEC', user_id FROM users WHERE email='frt.lect@kiu.ac.ug';
UPDATE course_offerings SET coordinator_id = (SELECT user_id::text FROM users WHERE email='frt.coord@kiu.ac.ug')
 WHERE offering_id='$OFF';
-- The lecture is timetabled into FRT-BUSY-TT for a window that covers right now.
INSERT INTO timetable_slots (tenant_id, offering_id, unit_id, day_of_week, start_time, duration_minutes, room, venue_id)
 VALUES ('$TEN','$OFF','FRT-UNIT',$DOW,'$BUSY_FROM'::time,90,'FRTEST Room Timetabled','FRT-BUSY-TT');
INSERT INTO lecturer_assignments (tenant_id, lecturer_id, unit_id, course_id, academic_year, year, semester, intake_session)
 SELECT '$TEN', lecturer_id, 'FRT-UNIT','FRT-CS','2025/2026',2,1,'Morning' FROM lecturers WHERE staff_id='FRT-LEC';
-- A DIFFERENT session is physically running in FRT-BUSY-LIVE, which the timetable says nothing about.
INSERT INTO sessions (tenant_id, coordinator_id, unit_id, venue_id, session_date, session_status, gate_open_time)
 SELECT '$TEN', user_id::text, 'FRT-OTHER','FRT-BUSY-LIVE', '$INST_DATE'::date, 'ACTIVE', now()
   FROM users WHERE email='frt.coord2@kiu.ac.ug';
SQL

login(){ curl -sk -X POST "$BASE/api/v1/auth/app-login" -H 'Content-Type: application/json' -m 30 \
  -d "{\"identifier\":\"$1\",\"password\":\"$2\",\"org\":\"\"}" \
  | python3 -c "import json,sys;print(json.load(sys.stdin).get('access_token',''))"; }
body(){ curl -sk -X "$1" "$BASE$2" -H "Authorization: Bearer $3" \
  ${4:+-H 'Content-Type: application/json' -d "$4"} -m 60; }
code(){ curl -sk -o /dev/null -w '%{http_code}' -X "$1" "$BASE$2" -H "Authorization: Bearer $3" \
  ${4:+-H 'Content-Type: application/json' -d "$4"} -m 60; }
room(){ python3 -c "
import json,sys
d=json.load(sys.stdin)
r=[x for x in d['rooms'] if x['venue_id']=='$1']
print(json.dumps(r[0], separators=(',',':')) if r else '{}')"; }

CRD=$(login FRT-CRD CooPass12345); MON=$(login FRT-MON MonPass12345)
QA=$(login FRT-QA QaPass12345); LEC=$(login FRT-LEC LecPass12345)
echo "tokens: coord=${#CRD} monitor=${#MON} qa=${#QA} lecturer=${#LEC}"

echo; echo "── 1. which rooms are free, right now ──"
FREE=$(body GET "/api/v1/rooms/free" "$CRD")
check "the coordinator can ask"        "$(code GET /api/v1/rooms/free "$CRD")" "200"
TT=$(room FRT-BUSY-TT <<<"$FREE"); LIVE=$(room FRT-BUSY-LIVE <<<"$FREE"); FR=$(room FRT-FREE-N <<<"$FREE")
echo "   timetabled room: $TT"
echo "   live-session room: $LIVE"
echo "   free room: $FR"
has  "a TIMETABLED lecture makes the room busy"    '"free":false'            "$TT"
has  "…and says what has it"                       '"occupied_kind":"TIMETABLE"' "$TT"
has  "…naming the lecture"                         'FRTEST Displaced Unit'   "$TT"
has  "…and when it frees up"                       '"occupied_until"'        "$TT"
# The timetable and reality can disagree in both directions; a running session counts on its own.
has  "a LIVE session makes the room busy too"      '"free":false'            "$LIVE"
has  "…marked as actually in use, not merely planned" '"occupied_kind":"LIVE_SESSION"' "$LIVE"
has  "an unclaimed room is offered as free"        '"free":true'             "$FR"

echo; echo "── 2. the search crosses every school, department and block ──"
# The only free room belongs to ANOTHER college in ANOTHER block. A department-scoped search
# would return nothing, which is the failure this feature exists to prevent.
has  "the free room is in a different college"     'FRTEST College of Nursing' "$FR"
has  "…and a different block"                      '"building":"Block N"'      "$FR"
has  "the room carries its capacity, so a class can be sized to it" '"capacity":120' "$FR"
check "every role that sees rooms can ask — QA"      "$(code GET /api/v1/rooms/free "$QA")"  "200"
check "…the monitor"                                 "$(code GET /api/v1/rooms/free "$MON")" "200"
# Looking AHEAD, so a coordinator can plan the next hour rather than discover it at the door.
check "a future time can be asked about"             "$(code GET '/api/v1/rooms/free?at=23:30&minutes=60' "$CRD")" "200"
FREE_LATER=$(sql "SELECT to_char((now() AT TIME ZONE 'Africa/Kampala') + interval '4 hours','HH24:MI')")
has  "…and four hours on, the timetabled room is free again" '"free":true' \
     "$(body GET "/api/v1/rooms/free?at=$FREE_LATER&minutes=30" "$CRD" | room FRT-BUSY-TT)"

echo; echo "── 3. the coordinator runs the lecture in the free room ──"
R=$(body POST /api/v1/sessions/open "$CRD" \
  '{"unit_id":"FRT-UNIT","venue_id":"FRT-FREE-N","room_is_provision":true,
    "provision_note":"Block A projector dead; moved with the students"}')
echo "   $R"
SID=$(python3 -c "
import json,sys
d=json.loads(sys.argv[1])
print(d['session_id'] if d.get('checkin_code') else '')" "$R")
hasnt "the session opens"  'error' "$R"
check "…recorded as a provision"      "$(sql "SELECT room_is_provision FROM sessions WHERE session_id='$SID'")" "t"
check "…in the room that was chosen"  "$(sql "SELECT venue_id FROM sessions WHERE session_id='$SID'")" "FRT-FREE-N"
check "…with the coordinator's reason kept" \
      "$(sql "SELECT provision_note FROM sessions WHERE session_id='$SID'")" "Block A projector dead; moved with the students"
# Taking a room makes it busy for the NEXT coordinator to ask — otherwise two classes are sent to it.
has  "the room is now busy for everyone else"  '"free":false' "$(body GET /api/v1/rooms/free "$CRD" | room FRT-FREE-N)"
has  "…and says it is being used as a provision" 'provision' "$(body GET /api/v1/rooms/free "$CRD" | room FRT-FREE-N)"

echo; echo "── 4. it reaches the ATTENDANCE LOG, where the dispute gets settled ──"
LOG=$(body GET /api/v1/dashboard/lecturer-attendance "$QA")
ROW=$(python3 -c "
import json,sys
rows=[r for r in json.load(sys.stdin) if r['unit_id']=='FRT-UNIT']
print(json.dumps(rows[0], separators=(',',':')) if rows else '{}')" <<<"$LOG")
echo "   $ROW"
has "the log names the room it was taught in" 'FRTEST Room Free Nursing' "$ROW"
has "…and marks it a provision"               '"room_is_provision":true' "$ROW"
has "…carrying the reason"                    'projector dead'           "$ROW"

echo; echo "── 5. the monitors are told BEFORE they walk to the wrong room ──"
INBOX=$(body GET /api/v1/app-notifications "$MON")
has "the monitor is notified"                 'Room change'                "$INBOX"
has "…told which room it is actually in"      'FRTEST Room Free Nursing'   "$INBOX"
has "…and which room the timetable says"      'FRTEST Room Timetabled'     "$INBOX"
has "…with the reason"                        'projector dead'             "$INBOX"
has "…and why it matters"                     'will find it empty'         "$INBOX"
has "the QA office is told as well"           'Room change'                "$(body GET /api/v1/app-notifications "$QA")"
has "so is the lecturer being observed"       'Room change'                "$(body GET /api/v1/app-notifications "$LEC")"

echo; echo "── 6. a NORMAL session is not announced to anyone ──"
body POST "/api/v1/sessions/$SID/close" "$CRD" '{}' >/dev/null
R=$(body POST /api/v1/sessions/open "$CRD" '{"unit_id":"FRT-UNIT","venue_id":"FRT-BUSY-TT"}')
SID2=$(python3 -c "import json,sys;print(json.loads(sys.argv[1]).get('session_id',''))" "$R")
check "…and is not marked a provision" "$(sql "SELECT room_is_provision FROM sessions WHERE session_id='$SID2'")" "f"
check "no second notification was sent" \
      "$(sql "SELECT count(*) FROM app_notifications WHERE unit_id='FRT-UNIT'")" "1"

echo; echo "── 7. every room list now shows what is in use ──"
has "the shared rooms list carries occupancy" 'in_use_by' "$(body GET /api/v1/dashboard/rooms "$QA")"

echo; echo "=== $pass passed, $fail failed ==="
[ "$fail" -eq 0 ]
