#!/usr/bin/env bash
#
# THE LECTURER'S WEEK, AND ATTENDANCE FOR A CLASS WITH NO ROOM.
#
# Two things are proved here, and the second is the one that matters most.
#
# A. THE TIMETABLE. The lecturer's dashboard showed a month of counters; it now shows their week.
#    That week must contain every cohort they teach — Day, Evening, Weekend and e-learning — must
#    run Monday to SUNDAY, and must not contain a colleague's classes. That last point is a real
#    bug being fixed, not a hypothetical: the old query keyed only on lecturer_assignments, so on a
#    unit taught by two people to different cohorts each of them saw the other's lectures.
#
# B. DISTANCE LEARNING. Every proof of presence in this system is physical: the student is on the
#    coordinator's hotspot, the lecturer scanned at the door. A distance cohort has no room, so
#    every one of those students was rejected with NOT_SAME_NETWORK and the institution's
#    e-learning attendance did not exist — in a system where attendance decides exam eligibility.
#
#    The dangerous way to fix that is a switch that lets anyone check in from anywhere. So the
#    tests below spend most of their effort proving the OPPOSITE of a new feature working: that
#    the campus gate is exactly as strict as it was, that only a cohort the institution marked as
#    e-learning can have an online class at all, that a code from thirty seconds ago is refused,
#    and that a student from another cohort holding a valid code is still turned away.
#
# Usage:  ./lecturer_timetable_and_distance_test.sh        (against https://localhost:8443)
# Seeds and removes its own DL- fixtures via the postgres container.
set -uo pipefail
BASE="${1:-https://localhost:8443}"; PG="${PG_CONTAINER:-infra-postgres-1}"; pass=0; fail=0
check(){ if [ "$2" = "$3" ]; then echo "   ✓ $1"; pass=$((pass+1)); else echo "   ✗ $1 (got '$2' want '$3')"; fail=$((fail+1)); fi; }
has(){ if grep -qF -- "$2" <<<"$3"; then echo "   ✓ $1"; pass=$((pass+1)); else echo "   ✗ $1 — '$2' not in: ${3:0:300}"; fail=$((fail+1)); fi; }
hasnt(){ if grep -qF -- "$2" <<<"$3"; then echo "   ✗ $1 — '$2' WAS present in: ${3:0:300}"; fail=$((fail+1)); else echo "   ✓ $1"; pass=$((pass+1)); fi; }
sql(){ docker exec -i "$PG" psql -U qaat -d qaat -tAc "$1"; }

cleanup(){ docker exec -i "$PG" psql -U qaat -d qaat -q >/dev/null 2>&1 <<'SQL'
DELETE FROM attendance_logs WHERE session_id IN (SELECT session_id FROM sessions WHERE unit_id LIKE 'DL-%');
DELETE FROM lecturer_attendance_logs WHERE unit_id LIKE 'DL-%';
DELETE FROM lecturer_patrol_logs WHERE unit_id LIKE 'DL-%';
DELETE FROM sessions          WHERE unit_id LIKE 'DL-%';
DELETE FROM timetable_slots   WHERE unit_id LIKE 'DL-%';
DELETE FROM lecturer_assignments WHERE unit_id LIKE 'DL-%';
DELETE FROM students_extended WHERE student_id LIKE 'DL-%';
DELETE FROM course_units      WHERE unit_id LIKE 'DL-%';
DELETE FROM course_offerings  WHERE course_id = 'DL-C';
DELETE FROM courses           WHERE course_id = 'DL-C';
DELETE FROM lecturers         WHERE staff_id LIKE 'DL-%';
DELETE FROM users             WHERE email LIKE 'dl.%';
SQL
}
SAVED_WINDOW=$(docker exec -i "$PG" psql -U qaat -d qaat -tAc \
  "SELECT to_char(session_window_start,'HH24:MI')||'|'||to_char(session_window_end,'HH24:MI')||'|'||session_active_days::text
     FROM tenants WHERE tenant_id <> '00000000-0000-0000-0000-000000000000' ORDER BY created_at LIMIT 1")
restore_window(){
  [ -n "${SAVED_WINDOW:-}" ] || return 0
  local st=${SAVED_WINDOW%%|*}; local rest=${SAVED_WINDOW#*|}; local en=${rest%%|*}; local days=${rest#*|}
  docker exec -i "$PG" psql -U qaat -d qaat -q -c \
    "UPDATE tenants SET session_window_start='$st'::time, session_window_end='$en'::time, session_active_days='$days'
      WHERE tenant_id=(SELECT tenant_id FROM tenants WHERE tenant_id <> '00000000-0000-0000-0000-000000000000' ORDER BY created_at LIMIT 1)" >/dev/null 2>&1
}
trap 'restore_window; cleanup' EXIT
cleanup
docker exec -i "$PG" psql -U qaat -d qaat -q -c \
  "UPDATE tenants SET session_window_start='00:00'::time, session_window_end='23:59'::time,
                      session_active_days=ARRAY[1,2,3,4,5,6,7]::smallint[]
    WHERE tenant_id=(SELECT tenant_id FROM tenants WHERE tenant_id <> '00000000-0000-0000-0000-000000000000' ORDER BY created_at LIMIT 1)" >/dev/null 2>&1

TEN=$(sql "SELECT tenant_id FROM tenants WHERE tenant_id <> '00000000-0000-0000-0000-000000000000' ORDER BY created_at LIMIT 1")
DOM=$(sql "SELECT domain FROM tenants WHERE tenant_id='$TEN'")
O_DAY='dd111111-0000-4000-8000-00000000d001'   # Day cohort, in person
O_WKD='dd111111-0000-4000-8000-00000000d002'   # Weekend cohort, in person
O_EVE='dd111111-0000-4000-8000-00000000d003'   # Evening cohort, in person
O_DIS='dd111111-0000-4000-8000-00000000d004'   # Distance Learning cohort → backfilled to ONLINE
TODAY_DOW=$(sql "SELECT EXTRACT(ISODOW FROM (now() AT TIME ZONE 'Africa/Kampala'))::int")

docker exec -i "$PG" psql -U qaat -d qaat -q -v ON_ERROR_STOP=1 >/dev/null <<SQL
CREATE EXTENSION IF NOT EXISTS pgcrypto;
INSERT INTO courses (course_id, tenant_id, name, department) VALUES ('DL-C','$TEN','DLTest Course','DLTEST Dept');
INSERT INTO course_units (unit_id, tenant_id, course_id, name, year, semester) VALUES
 ('DL-U1','$TEN','DL-C','DLTest Shared Unit',2,1),
 ('DL-U2','$TEN','DL-C','DLTest Evening Unit',2,1),
 ('DL-U3','$TEN','DL-C','DLTest Distance Unit',2,1);
-- Four cohorts of one course. "Distance Learning" is typed the way the institution types it — the
-- migration's backfill has to recognise it, because nobody is going to revisit every cohort.
INSERT INTO course_offerings (offering_id, tenant_id, course_id, session_type, study_year, semester, level, intake) VALUES
 ('$O_DAY','$TEN','DL-C','Day',2,1,'Bachelors','August Intake'),
 ('$O_WKD','$TEN','DL-C','Weekend',2,1,'Bachelors','August Intake'),
 ('$O_EVE','$TEN','DL-C','Evening',2,1,'Bachelors','January Intake'),
 ('$O_DIS','$TEN','DL-C','Distance Learning',2,1,'Bachelors','August Intake');
INSERT INTO users (tenant_id, email, password_hash, role, full_name, is_active, staff_id, force_password_change) VALUES
 ('$TEN','dl.lect@$DOM',  crypt('LecPass12345', gen_salt('bf',10)),'LECTURER','DL Lecturer',       true,'DL-LEC',false),
 ('$TEN','dl.other@$DOM', crypt('LecPass12345', gen_salt('bf',10)),'LECTURER','DL Other Lecturer', true,'DL-OTH',false),
 ('$TEN','dl.coord@$DOM', crypt('CooPass12345', gen_salt('bf',10)),'COORDINATOR','DL Coordinator', true,'DL-CRD',false),
 ('$TEN','dl.mon@$DOM',   crypt('MonPass12345', gen_salt('bf',10)),'QA_PATROLLER','DL Monitor',    true,'DL-MON',false),
 ('$TEN','dl.qao@$DOM',   crypt('QaoPass12345', gen_salt('bf',10)),'QA_OFFICER','DL QA Officer',    true,'DL-QAO',false);
INSERT INTO lecturers (tenant_id, full_name, email, staff_id, user_id)
 SELECT '$TEN','DL Lecturer','dl.lect@$DOM','DL-LEC', user_id FROM users WHERE email='dl.lect@$DOM';
INSERT INTO lecturers (tenant_id, full_name, email, staff_id, user_id)
 SELECT '$TEN','DL Other Lecturer','dl.other@$DOM','DL-OTH', user_id FROM users WHERE email='dl.other@$DOM';
-- BOTH lecturers are assigned to DL-U1. That is the shape that used to make each of them see the
-- other's cohorts, and it is why the slot's own lecturer must win over the assignment.
INSERT INTO lecturer_assignments (tenant_id, lecturer_id, unit_id, course_id, academic_year, year, semester, intake_session)
 SELECT '$TEN', lecturer_id, 'DL-U1','DL-C','2025/2026',2,1,'Morning' FROM lecturers WHERE staff_id IN ('DL-LEC','DL-OTH');
INSERT INTO lecturer_assignments (tenant_id, lecturer_id, unit_id, course_id, academic_year, year, semester, intake_session)
 SELECT '$TEN', lecturer_id, u, 'DL-C','2025/2026',2,1,'Morning'
   FROM lecturers, (VALUES ('DL-U2'),('DL-U3')) AS x(u) WHERE staff_id='DL-LEC';
UPDATE course_offerings SET coordinator_id=(SELECT user_id::text FROM users WHERE email='dl.coord@$DOM') WHERE offering_id='$O_DAY';

-- The week. Tuesday morning (Day), SUNDAY evening (Distance), SATURDAY (Weekend), Wed night
-- (Evening). The Sunday and Saturday rows are the ones a Mon–Fri grid would silently swallow.
INSERT INTO timetable_slots (tenant_id, offering_id, unit_id, day_of_week, start_time, duration_minutes, room, lecturer_id)
 SELECT '$TEN','$O_DAY','DL-U1',2,'08:00',120,'DL Hall A', lecturer_id FROM lecturers WHERE staff_id='DL-LEC';
INSERT INTO timetable_slots (tenant_id, offering_id, unit_id, day_of_week, start_time, duration_minutes, room, lecturer_id)
 SELECT '$TEN','$O_WKD','DL-U1',6,'09:00',180,'DL Hall B', lecturer_id FROM lecturers WHERE staff_id='DL-LEC';
INSERT INTO timetable_slots (tenant_id, offering_id, unit_id, day_of_week, start_time, duration_minutes, room, lecturer_id)
 SELECT '$TEN','$O_EVE','DL-U2',3,'18:00',120,'DL Hall C', lecturer_id FROM lecturers WHERE staff_id='DL-LEC';
INSERT INTO timetable_slots (tenant_id, offering_id, unit_id, day_of_week, start_time, duration_minutes, lecturer_id)
 SELECT '$TEN','$O_DIS','DL-U3',7,'20:00',120, lecturer_id FROM lecturers WHERE staff_id='DL-LEC';
-- The other lecturer's Day cohort of the SAME unit. Named on the slot, so it is unambiguously
-- theirs — and must not appear in DL-LEC's week even though DL-LEC is assigned to the unit.
INSERT INTO timetable_slots (tenant_id, offering_id, unit_id, day_of_week, start_time, duration_minutes, room, lecturer_id)
 SELECT '$TEN','$O_EVE','DL-U1',4,'14:00',60,'DL Hall D', lecturer_id FROM lecturers WHERE staff_id='DL-OTH';
-- And a slot for the distance cohort TODAY, so the monitor-round assertions have something to
-- find (or, as it should turn out, not find).
INSERT INTO timetable_slots (tenant_id, offering_id, unit_id, day_of_week, start_time, duration_minutes, lecturer_id)
 SELECT '$TEN','$O_DIS','DL-U3',$TODAY_DOW,'21:00',60, lecturer_id FROM lecturers WHERE staff_id='DL-LEC';

-- Students: two on the distance cohort, one on the Day cohort (who must NOT be able to use a
-- distance code).
INSERT INTO students_extended (student_id, tenant_id, full_name, email, course_id, academic_year, offering_id, current_year, semester, intake_session) VALUES
 ('DL-S1','$TEN','DL Distance One','dl.s1@studmc.$DOM','DL-C','2025/2026','$O_DIS',2,1,'Morning'),
 ('DL-S2','$TEN','DL Distance Two','dl.s2@studmc.$DOM','DL-C','2025/2026','$O_DIS',2,1,'Morning'),
 ('DL-S3','$TEN','DL Day Student', 'dl.s3@studmc.$DOM','DL-C','2025/2026','$O_DAY',2,1,'Morning');
INSERT INTO users (tenant_id, email, password_hash, role, full_name, is_active, registration_number, force_password_change) VALUES
 ('$TEN','dl.s1@studmc.$DOM', crypt('StuPass12345', gen_salt('bf',10)),'STUDENT','DL Distance One', true,'DL-S1',false),
 ('$TEN','dl.s2@studmc.$DOM', crypt('StuPass12345', gen_salt('bf',10)),'STUDENT','DL Distance Two', true,'DL-S2',false),
 ('$TEN','dl.s3@studmc.$DOM', crypt('StuPass12345', gen_salt('bf',10)),'STUDENT','DL Day Student',  true,'DL-S3',false);
SQL

login(){ curl -sk -X POST "$BASE/api/v1/auth/app-login" -H 'Content-Type: application/json' -m 30 \
  -d "{\"identifier\":\"$1\",\"password\":\"$2\",\"org\":\"\"}" \
  | python3 -c "import json,sys;print(json.load(sys.stdin).get('access_token',''))"; }
body(){ curl -sk -X "$1" "$BASE$2" -H "Authorization: Bearer $3" \
  ${4:+-H 'Content-Type: application/json' -d "$4"} -m 60; }
jget(){ python3 -c "import json,sys;print(json.load(sys.stdin).get('$1',''))"; }

LEC=$(login DL-LEC LecPass12345); OTH=$(login DL-OTH LecPass12345)
CRD=$(login DL-CRD CooPass12345); MON=$(login DL-MON MonPass12345); QAO=$(login DL-QAO QaoPass12345)
S1=$(login DL-S1 StuPass12345); S2=$(login DL-S2 StuPass12345); S3=$(login DL-S3 StuPass12345)
echo "tokens: lec=${#LEC} other=${#OTH} coord=${#CRD} mon=${#MON} s1=${#S1} s2=${#S2} s3=${#S3}"

echo; echo "── 0. the migration recognised the institution's own spelling ──"
# This cohort was created AFTER the migration ran, which is the case the backfill alone could
# never cover — every intake creates new cohorts. Migration 088's trigger is what makes it true.
check "a NEW \"Distance Learning\" cohort becomes ONLINE by itself" \
  "$(sql "SELECT delivery_mode FROM course_offerings WHERE offering_id='$O_DIS'")" "ONLINE"
check "…and the Day cohort was left alone" \
  "$(sql "SELECT delivery_mode FROM course_offerings WHERE offering_id='$O_DAY'")" "IN_PERSON"
check "…as were Evening and Weekend" \
  "$(sql "SELECT count(*) FROM course_offerings WHERE offering_id IN ('$O_EVE','$O_WKD') AND delivery_mode='IN_PERSON'")" "2"

echo; echo "── 1. the lecturer's week: every cohort, Monday to Sunday ──"
TT=$(body GET /api/v1/lecturer/timetable "$LEC")
N=$(python3 -c "import json,sys;print(len(json.load(sys.stdin)['slots']))" <<<"$TT")
check "five slots come back (4 cohorts + today's distance slot)" "$N" "5"
has "the Day cohort is there"      '"session_type":"Day"' "$TT"
has "the Weekend cohort is there"  '"session_type":"Weekend"' "$TT"
has "the Evening cohort is there"  '"session_type":"Evening"' "$TT"
has "the e-learning cohort is there" '"session_type":"Distance Learning"' "$TT"
# The two days a Mon–Fri grid cannot show.
check "Saturday is on the week" \
  "$(python3 -c "import json,sys;print(sum(1 for s in json.load(sys.stdin)['slots'] if s['day_of_week']==6))" <<<"$TT")" "1"
check "Sunday is on the week" \
  "$(python3 -c "import json,sys;print(sum(1 for s in json.load(sys.stdin)['slots'] if s['day_of_week']==7))" <<<"$TT")" "1"
has "the course is named, so a filter can use it"  '"course_name":"DLTest Course"' "$TT"
has "the intake is carried"        '"intake":"August Intake"' "$TT"
has "the other intake is carried"  '"intake":"January Intake"' "$TT"
has "the distance slot is marked ONLINE" '"delivery_mode":"ONLINE"' "$TT"
has "enrolment is on the slot"     '"enrolled":2' "$TT"
# THE BUG THIS FIXES: DL-LEC is assigned to DL-U1, and DL-OTH's Thursday slot is on that unit.
hasnt "the colleague's slot is NOT in this lecturer's week" '"room":"DL Hall D"' "$TT"
OT=$(body GET /api/v1/lecturer/timetable "$OTH")
has  "…and it IS in the colleague's own week" '"room":"DL Hall D"' "$OT"
check "the colleague sees only their own slot" \
  "$(python3 -c "import json,sys;print(len(json.load(sys.stdin)['slots']))" <<<"$OT")" "1"

echo; echo "── 2. an online class can only be started for an e-learning cohort ──"
# The whole safety boundary. If this refusal fails, a lecturer could open a room-less, LAN-less
# session for a campus cohort and every one of those students could mark themselves present from
# home — which is precisely what the proximity gate exists to stop.
R=$(body POST /api/v1/lecturer/online-class "$LEC" '{"unit_id":"DL-U1"}')
has "an in-person unit is refused" 'NOT_AN_ONLINE_COHORT' "$R"
R=$(body POST /api/v1/lecturer/online-class "$LEC" '{"unit_id":"DL-NOPE"}')
has "a unit they do not teach is refused" 'NOT_YOUR_UNIT' "$R"
R=$(body POST /api/v1/lecturer/online-class "$OTH" '{"unit_id":"DL-U3"}')
has "a lecturer who does not teach the distance unit is refused" 'NOT_YOUR_UNIT' "$R"

echo; echo "── 3. the lecturer starts the distance class and is recorded present ──"
R=$(body POST /api/v1/lecturer/online-class "$LEC" '{"unit_id":"DL-U3","fingerprint":"dl-phone"}')
SID=$(jget session_id <<<"$R"); CODE=$(jget student_code <<<"$R")
has   "the class starts"                    '"session_id"' "$R"
check "the session is marked ONLINE"        "$(sql "SELECT delivery_mode FROM sessions WHERE session_id='$SID'")" "ONLINE"
check "…and has no room"                    "$(sql "SELECT COALESCE(venue_id,'-') FROM sessions WHERE session_id='$SID'")" "-"
# The lecturer's presence is the same column a gate scan writes, because it means the same thing.
check "the lecturer is recorded present"    "$(sql "SELECT lecturer_scanned_at IS NOT NULL FROM lecturer_attendance_logs WHERE session_id='$SID'")" "t"
check "the code is six digits"              "${#CODE}" "6"
# Starting twice returns the SAME class rather than splitting the roster across two.
R2=$(body POST /api/v1/lecturer/online-class "$LEC" '{"unit_id":"DL-U3"}')
check "starting again returns the same class" "$(jget session_id <<<"$R2")" "$SID"
check "…and did not open a second session"  "$(sql "SELECT count(*) FROM sessions WHERE unit_id='DL-U3'")" "1"

echo; echo "── 4. the distance students check in — and the gates that remain ──"
# The rotating code is the proof, so yesterday's digits are worthless.
R=$(body POST /api/v1/student/checkin "$S1" "{\"session_id\":\"$SID\",\"room_code\":\"000000\"}")
has "a stale/wrong code is refused"         'CODE_NOT_CURRENT' "$R"
LIVE=$(body GET /api/v1/lecturer/online-class "$LEC" | jget student_code)
R=$(body POST /api/v1/student/checkin "$S1" "{\"session_id\":\"$SID\",\"room_code\":\"$LIVE\",\"device_fingerprint\":\"dl-1\"}")
has "the distance student checks in"        '"status":"PRESENT"' "$R"
LIVE=$(body GET /api/v1/lecturer/online-class "$LEC" | jget student_code)
R=$(body POST /api/v1/student/checkin "$S2" "{\"session_id\":\"$SID\",\"room_code\":\"$LIVE\",\"device_fingerprint\":\"dl-2\"}")
has "and so does the second"                '"status":"PRESENT"' "$R"
# Cohort membership replaces the room. A student holding a perfectly valid live code, who is not
# enrolled in this class, is not present at it.
LIVE=$(body GET /api/v1/lecturer/online-class "$LEC" | jget student_code)
R=$(body POST /api/v1/student/checkin "$S3" "{\"session_id\":\"$SID\",\"room_code\":\"$LIVE\",\"device_fingerprint\":\"dl-3\"}")
has "a student from another cohort is refused" 'NOT_IN_THIS_COHORT' "$R"
# One device, one person — unchanged from the campus flow.
LIVE=$(body GET /api/v1/lecturer/online-class "$LEC" | jget student_code)
R=$(body POST /api/v1/student/checkin "$S1" "{\"session_id\":\"$SID\",\"room_code\":\"$LIVE\",\"device_fingerprint\":\"dl-2\"}")
has "one device still cannot mark two students" 'DEVICE_ALREADY_USED' "$R"
check "exactly two on the roll, no duplicates" \
  "$(sql "SELECT count(*)||'/'||count(DISTINCT student_id) FROM attendance_logs al JOIN sessions s ON s.session_id=al.session_id WHERE s.unit_id='DL-U3'")" "2/2"

echo; echo "── 5. THE CAMPUS GATE IS UNCHANGED ──"
# The point of the whole exercise. An in-person session must still refuse a check-in that cannot
# prove it is on the coordinator's network — this request comes from outside it.
R=$(body POST /api/v1/sessions/open "$CRD" '{"unit_id":"DL-U1","unscheduled":true}')
PSID=$(jget session_id <<<"$R"); PCODE=$(jget student_code <<<"$R")
check "the in-person session is IN_PERSON"  "$(sql "SELECT delivery_mode FROM sessions WHERE session_id='$PSID'")" "IN_PERSON"
# Strip the coordinator's network anchor: with no anchor, presence cannot be proven, and the
# answer must be a refusal rather than a fallback to the online path.
docker exec -i "$PG" psql -U qaat -d qaat -q -c "UPDATE sessions SET coordinator_ip='' WHERE session_id='$PSID'" >/dev/null
R=$(body POST /api/v1/student/checkin "$S3" "{\"session_id\":\"$PSID\",\"room_code\":\"$PCODE\",\"device_fingerprint\":\"dl-3\"}")
has "a campus check-in with no network proof is still refused" 'NOT_SAME_NETWORK' "$R"
# And the campus session still wants the STATIC code, not a rotating one — the two paths did not
# get crossed.
R=$(body POST /api/v1/student/checkin "$S3" "{\"session_id\":\"$PSID\",\"room_code\":\"$LIVE\",\"device_fingerprint\":\"dl-3\"}")
has "a rotating code does not open a campus session" 'REJECTED' "$R"

echo; echo "── 6. the QA monitor is not sent to a room that does not exist ──"
curl -sk -X POST "$BASE/api/v1/patrol/bind-device" -H "Authorization: Bearer $MON" \
  -H 'Content-Type: application/json' -d '{"device_fingerprint":"dl-handset"}' -m 30 >/dev/null
MF=$(body GET /api/v1/patrol/manifest "$MON")
hasnt "the distance lecture is off the monitor's round" '"unit_id":"DL-U3"' "$MF"
has   "…while the in-person ones are still on it"       '"unit_id":"DL-U1"' "$MF"
SR=$(body GET "/api/v1/patrol/search?by=lecturer&q=DL-LEC" "$MON")
hasnt "and searching the lecturer does not surface it either" '"unit_id":"DL-U3"' "$SR"

echo; echo "── 6b. the online class is reachable from the PHONE, not a web console ──"
# The web lecturer dashboard is gone. It was the only place this control lived, so if these
# endpoints were not on the phone's path, removing it would have silently taken distance-learning
# attendance with it: no coordinator opens an e-learning room, so a lecturer who cannot start the
# class means no student on that cohort can check in to anything.
check "the passwordless staff-ID login is gone" \
  "$(curl -sk -o /dev/null -w '%{http_code}' -X POST "$BASE/api/v1/auth/lecturer-login" \
      -H 'Content-Type: application/json' -d '{"staff_id":"DL-LEC","org":""}' -m 20)" "404"
# …and the token the phone actually holds still opens everything the lecturer needs.
has "the phone's timetable call works"      '"slots"' "$(body GET /api/v1/lecturer/timetable "$LEC")"
has "the phone's recipient list works"      '"coordinators"' "$(body GET /api/v1/lecturer/recipients "$LEC")"
has "the phone's unrecorded list works"     '"lectures"' "$(body GET /api/v1/lecturer/unrecorded "$LEC")"
has "…and the running class is readable"    '"student_code"' "$(body GET /api/v1/lecturer/online-class "$LEC")"

echo; echo "── 7. ending the class books the contact hours ──"
R=$(body POST /api/v1/lecturer/online-class/end "$LEC" '{}')
has   "the class ends"                      '"status":"ENDED"' "$R"
check "contact hours are recorded"          "$(sql "SELECT contact_hours IS NOT NULL FROM lecturer_attendance_logs WHERE session_id='$SID'")" "t"
check "the session is closed"               "$(sql "SELECT session_status FROM sessions WHERE session_id='$SID'")" "CLOSED"
R=$(body POST /api/v1/lecturer/online-class/end "$LEC" '{}')
has   "ending again says there is nothing running" 'NO_OPEN_CLASS' "$R"

echo; echo "── 8. the record says ONLINE wherever it is read ──"
# An online tick must never be read as a physical one, so delivery_mode travels with the row.
AD=$(body GET "/api/v1/dashboard/lecturer-attendance?unit_id=DL-U3" "$QAO")
has "the attendance log carries the delivery mode" '"delivery_mode":"ONLINE"' "$AD"
LD=$(body GET "/api/v1/lecturer/attendance?unit_id=DL-U3" "$LEC")
has "the lecturer's own dashboard says so too"     '"delivery_mode":"ONLINE"' "$LD"

echo; echo "── 9. the duplicate-attendance hole is closed ──"
# Both of attendance_logs' unique indexes were partial on entry_method='QR_SCAN', so the
# authenticated cloud check-in — which writes AUTHENTICATED — was covered by neither, and the
# INSERT's error branch was relying on an index that did not apply to it.
check "AUTHENTICATED rows now have a unique index" \
  "$(sql "SELECT count(*) FROM pg_indexes WHERE indexname='uq_attendance_session_student_auth'")" "1"
DUP=$(sql "INSERT INTO attendance_logs (tenant_id, session_id, student_id, checkin_timestamp, sequence_number, entry_method)
           VALUES ('$TEN','$SID','DL-S1', now(), 99, 'AUTHENTICATED') RETURNING 1" 2>&1)
has "a second row for the same student is rejected by the database" 'duplicate key' "$DUP"

echo
echo "── done ── passed: $pass, failed: $fail"
[ "$fail" -eq 0 ]
