#!/usr/bin/env bash
#
# WHAT THIS COVERS
#
#   1. THE RENAME. "Patrol" and "patroller" are gone from everything a person reads. The stored role
#      value QA_PATROLLER deliberately survives — renaming a database enum means rewriting every row
#      that holds it and invalidating every signed token that carries it, which is three ways to
#      lock a live institution out of its own system in exchange for a word. The word is what people
#      see, so the word is what changed; the tests below check the words, and check that the old
#      audience code still works so a browser tab nobody has reloaded does not silently fail.
#
#   2. ONE HOUR, SEVERAL UNIT CODES. The same content is required by several programmes and each
#      codes it differently, so one lecture in one room satisfies two or three unit codes at once.
#      The monitor's search returned one of them, so the students on the other codes had a lecture
#      QA never saw. Both the search and the manual form now carry them all.
#
#   3. A COMPENSATION THAT NAMES ITS LECTURE, to the hour. Covered in the manual-attendance suite.
#
#   4. THE LECTURER IS ASKED FIRST. A lecture whose time has elapsed with nothing saying the
#      lecturer was there is offered to them to account for — before any monitor's tick has synced.
#
#   5. PERCENTAGES. Per unit and overall, for students and for lecturers' rosters.
#
#   6. THE ROSTER SIEVE. A lecturer saw every cohort of the course, not the cohorts they teach.
#
# Usage:  ./monitor_rename_and_concurrent_units_test.sh   (against https://localhost:8443)
set -uo pipefail
BASE="${1:-https://localhost:8443}"; PG="${PG_CONTAINER:-infra-postgres-1}"; pass=0; fail=0
check(){ if [ "$2" = "$3" ]; then echo "   ✓ $1"; pass=$((pass+1)); else echo "   ✗ $1 (got '$2' want '$3')"; fail=$((fail+1)); fi; }
has(){ if grep -qF -- "$2" <<<"$3"; then echo "   ✓ $1"; pass=$((pass+1)); else echo "   ✗ $1 — '$2' not in: ${3:0:400}"; fail=$((fail+1)); fi; }
hasnt(){ if grep -qF -- "$2" <<<"$3"; then echo "   ✗ $1 — '$2' WAS present"; fail=$((fail+1)); else echo "   ✓ $1"; pass=$((pass+1)); fi; }
sql(){ docker exec -i "$PG" psql -U qaat -d qaat -tAc "$1"; }

cleanup(){ docker exec -i "$PG" psql -U qaat -d qaat -q >/dev/null 2>&1 <<'SQL'
DELETE FROM monitor_log_units WHERE unit_id LIKE 'MR-%';
DELETE FROM lecturer_patrol_logs WHERE unit_id LIKE 'MR-%' OR unit_id LIKE 'Ghost MR%';
DELETE FROM attendance_logs WHERE session_id IN (SELECT session_id FROM sessions WHERE unit_id LIKE 'MR-%');
DELETE FROM lecturer_attendance_logs WHERE unit_id LIKE 'MR-%';
DELETE FROM sessions          WHERE unit_id LIKE 'MR-%';
DELETE FROM timetable_slots   WHERE unit_id LIKE 'MR-%';
DELETE FROM lecturer_assignments WHERE unit_id LIKE 'MR-%';
DELETE FROM students_extended WHERE student_id LIKE 'MR-%';
DELETE FROM course_units      WHERE unit_id LIKE 'MR-%';
DELETE FROM course_offerings  WHERE course_id LIKE 'MR-%';
DELETE FROM courses           WHERE course_id LIKE 'MR-%';
DELETE FROM lecturers         WHERE staff_id LIKE 'MR-%';
DELETE FROM notification_recipients WHERE notification_id IN
  (SELECT notification_id FROM app_notifications WHERE subject LIKE '%MRtest%');
DELETE FROM app_notifications WHERE subject LIKE '%MRtest%';
DELETE FROM course_offerings  WHERE coordinator_id IN (SELECT user_id::text FROM users WHERE email LIKE 'mr.%');
DELETE FROM users             WHERE email LIKE 'mr.%';
SQL
}
trap cleanup EXIT; cleanup

TEN=$(sql "SELECT tenant_id FROM tenants WHERE tenant_id <> '00000000-0000-0000-0000-000000000000' ORDER BY created_at LIMIT 1")
DOM=$(sql "SELECT domain FROM tenants WHERE tenant_id='$TEN'")
TODAY=$(sql "SELECT (now() AT TIME ZONE 'Africa/Kampala')::date")
DOW=$(sql "SELECT EXTRACT(ISODOW FROM (now() AT TIME ZONE 'Africa/Kampala'))::int")
# An hour that has already gone by today, so the "unrecorded" list has something to find. Two hours
# back, one hour long — comfortably elapsed whatever minute this runs at.
GONE=$(sql "SELECT to_char((now() AT TIME ZONE 'Africa/Kampala') - interval '2 hours','HH24:00')")
O_CS='ee111111-0000-4000-8000-00000000e001'   # the cohort the lecturer teaches
O_OTHER='ee111111-0000-4000-8000-00000000e002' # a cohort of the SAME course they do not

docker exec -i "$PG" psql -U qaat -d qaat -q -v ON_ERROR_STOP=1 >/dev/null <<SQL
CREATE EXTENSION IF NOT EXISTS pgcrypto;
-- Two courses in ONE college but DIFFERENT departments — the shape that makes the department field
-- matter and the college de-duplication visible.
INSERT INTO courses (course_id, tenant_id, name, department, school) VALUES
 ('MR-CS','$TEN','MRtest Computing','MRtest Computer Science','MRtest College'),
 ('MR-IT','$TEN','MRtest Info Tech','MRtest Information Technology','MRtest College');
INSERT INTO course_units (unit_id, tenant_id, course_id, name, year, semester) VALUES
 ('MR-CSC3103','$TEN','MR-CS','MRtest Research Methods',3,1),
 ('MR-BIT3110','$TEN','MR-IT','MRtest Research Methodology',3,1);
INSERT INTO course_offerings (offering_id, tenant_id, course_id, session_type, study_year, semester, level, intake) VALUES
 ('$O_CS','$TEN','MR-CS','Day',3,1,'Bachelors','August Intake'),
 ('$O_OTHER','$TEN','MR-CS','Evening',3,1,'Bachelors','January Intake');
INSERT INTO users (tenant_id, email, password_hash, role, full_name, is_active, staff_id, force_password_change) VALUES
 ('$TEN','mr.mon@$DOM',  crypt('MonPass12345', gen_salt('bf',10)),'QA_PATROLLER','MR Monitor',   true,'MR-MON',false),
 ('$TEN','mr.lect@$DOM', crypt('LecPass12345', gen_salt('bf',10)),'LECTURER',    'MR Lecturer',  true,'MR-LEC',false),
 ('$TEN','mr.qa@$DOM',   crypt('QaPass12345',  gen_salt('bf',10)),'QA_OFFICER',  'MR QA',        true,'MR-QA',false);
INSERT INTO lecturers (tenant_id, full_name, email, staff_id, user_id, department)
 SELECT '$TEN','MR Lecturer','mr.lect@$DOM','MR-LEC', user_id, 'MRtest Computer Science'
   FROM users WHERE email='mr.lect@$DOM';
INSERT INTO lecturer_assignments (tenant_id, lecturer_id, unit_id, course_id, academic_year, year, semester, intake_session)
 SELECT '$TEN', lecturer_id, u, c, '2025/2026',3,1,'Morning'
   FROM lecturers, (VALUES ('MR-CSC3103','MR-CS'),('MR-BIT3110','MR-IT')) AS x(u,c)
  WHERE staff_id='MR-LEC';
-- ONE hour, ONE room, ONE lecturer — two unit codes. This is the whole point of section 2.
INSERT INTO timetable_slots (tenant_id, offering_id, unit_id, day_of_week, start_time, duration_minutes, room, lecturer_id)
 SELECT '$TEN','$O_CS','MR-CSC3103',$DOW,'$GONE',60,'MR Hall 1', lecturer_id FROM lecturers WHERE staff_id='MR-LEC';
INSERT INTO timetable_slots (tenant_id, offering_id, unit_id, day_of_week, start_time, duration_minutes, room, lecturer_id)
 SELECT '$TEN','$O_CS','MR-BIT3110',$DOW,'$GONE',60,'MR Hall 1', lecturer_id FROM lecturers WHERE staff_id='MR-LEC';
-- Students: two in the cohort the lecturer teaches, one in the cohort they do NOT.
-- TWO coordinators, one per cohort — the shape that makes "the coordinator" meaningless. A third
-- runs a cohort of the same course the lecturer does NOT teach, and must be unreachable.
INSERT INTO users (tenant_id, email, password_hash, role, full_name, is_active, staff_id, coordinator_code, force_password_change) VALUES
 ('$TEN','mr.c1@$DOM', crypt('CooPass12345', gen_salt('bf',10)),'COORDINATOR','MR Day Coordinator',     true,'MR-C1','MRC1',false),
 ('$TEN','mr.c2@$DOM', crypt('CooPass12345', gen_salt('bf',10)),'COORDINATOR','MR Evening Coordinator', true,'MR-C2','MRC2',false);
UPDATE course_offerings SET coordinator_id=(SELECT user_id::text FROM users WHERE email='mr.c1@$DOM') WHERE offering_id='$O_CS';
UPDATE course_offerings SET coordinator_id=(SELECT user_id::text FROM users WHERE email='mr.c2@$DOM') WHERE offering_id='$O_OTHER';
INSERT INTO students_extended (student_id, tenant_id, full_name, email, course_id, academic_year, offering_id, current_year, semester) VALUES
 ('MR-S1','$TEN','MR Student One','mr.s1@studmc.$DOM','MR-CS','2025/2026','$O_CS',3,1),
 ('MR-S2','$TEN','MR Student Two','mr.s2@studmc.$DOM','MR-CS','2025/2026','$O_CS',3,1),
 ('MR-S3','$TEN','MR Evening Student','mr.s3@studmc.$DOM','MR-CS','2025/2026','$O_OTHER',3,1);
-- Students are reached through their sign-in accounts, so a roster row alone is not a recipient.
INSERT INTO users (tenant_id, email, password_hash, role, full_name, is_active, registration_number, force_password_change) VALUES
 ('$TEN','mr.s1@studmc.$DOM', crypt('StuPass12345', gen_salt('bf',10)),'STUDENT','MR Student One',     true,'MR-S1',false),
 ('$TEN','mr.s2@studmc.$DOM', crypt('StuPass12345', gen_salt('bf',10)),'STUDENT','MR Student Two',     true,'MR-S2',false),
 ('$TEN','mr.s3@studmc.$DOM', crypt('StuPass12345', gen_salt('bf',10)),'STUDENT','MR Evening Student', true,'MR-S3',false);
SQL

login(){ curl -sk -X POST "$BASE/api/v1/auth/app-login" -H 'Content-Type: application/json' -m 30 \
  -d "{\"identifier\":\"$1\",\"password\":\"$2\",\"org\":\"\"}" \
  | python3 -c "import json,sys;print(json.load(sys.stdin).get('access_token',''))"; }
body(){ curl -sk -X "$1" "$BASE$2" -H "Authorization: Bearer $3" \
  ${4:+-H 'Content-Type: application/json' -d "$4"} -m 60; }
jget(){ python3 -c "import json,sys;print(json.load(sys.stdin).get('$1',''))"; }

MON=$(login MR-MON MonPass12345); LEC=$(login MR-LEC LecPass12345); QA=$(login MR-QA QaPass12345)
echo "tokens: mon=${#MON} lec=${#LEC} qa=${#QA}"
curl -sk -X POST "$BASE/api/v1/patrol/bind-device" -H "Authorization: Bearer $MON" \
  -H 'Content-Type: application/json' -d '{"device_fingerprint":"mr-handset"}' -m 30 >/dev/null

echo; echo "── 1. the word is gone from everything a person reads ──"
# The source is the artefact here: strings on screens, not JSON field names or the stored role.
VISIBLE=$(grep -rniE "\bpatrol(ler|led|ling|s)?\b" \
  frontend/admin-dashboards/src frontend/coordinator-android/app/src/main \
  --include=*.tsx --include=*.ts --include=*.kt 2>/dev/null \
  | grep -viE "api/v1|QA_PATROLLER|patroller_name|patroller_staff_id|patrol_id|patrol_taught|patrol_room|patrol_taken_at|patrolled|patrols_week|last_patrol_date|patrollers_unbound|PATROL_DEVICE|PATROL_PIN|patrol_logs|patrol_slots" | wc -l)
check "no readable 'patrol' left in the dashboards or the app" "$VISIBLE" "0"
check "the PIN screen says monitor" \
  "$(grep -c 'Change monitor PIN' frontend/coordinator-android/app/src/main/java/ug/qaat/coordinator/ui/PatrolPinGate.kt)" "1"
check "the role is LABELLED QA Monitor" \
  "$(grep -c "QA_PATROLLER:      .QA Monitor" frontend/admin-dashboards/src/lib/roleLabel.ts)" "1"
check "…while the STORED role value is untouched" \
  "$(sql "SELECT role FROM users WHERE email='mr.mon@$DOM'")" "QA_PATROLLER"
# The dashboards now send MONITORS; a tab nobody has reloaded still sends PATROLLERS. Both must work
# or the rename silently breaks messaging for everyone mid-deploy.
R=$(body POST /api/v1/app-notifications "$QA" '{"audience":"MONITORS","subject":"MRtest brief","body":"new spelling"}')
has "a briefing sent to MONITORS is delivered"  '"status":"SENT"' "$R"
R=$(body POST /api/v1/app-notifications "$QA" '{"audience":"PATROLLERS","subject":"MRtest brief old","body":"old spelling"}')
has "…and a stale client's PATROLLERS still works" '"status":"SENT"' "$R"

echo; echo "── 2. one hour, several unit codes ──"
SR=$(body GET "/api/v1/patrol/search?by=lecturer&q=MR-LEC" "$MON")
has "the search finds the lecture"              'MR-CSC3103' "$SR"
has "…and names the OTHER code in the same room" '"also_here"' "$SR"
has "…with its code"                            'MR-BIT3110' "$SR"
check "each result carries exactly one companion" \
  "$(python3 -c "import json,sys;d=json.load(sys.stdin);print(sorted(len(r['also_here']) for r in d['results']))" <<<"$SR")" "[1, 1]"
# The companion must be the OTHER unit, never the row's own.
check "a result never lists itself as a companion" \
  "$(python3 -c "
import json,sys
d=json.load(sys.stdin)
print(sum(1 for r in d['results'] for a in r['also_here'] if a['unit_id']==r['unit_id']))" <<<"$SR")" "0"
has "the companion carries its cohort"          'August Intake' "$SR"

echo; echo "── 3. the manual form records every code the hour covered ──"
R=$(body POST /api/v1/patrol/manual "$MON" \
  '{"unit_id":"MR-CSC3103","lecturer_staff_id":"MR-LEC","students_counted":30,"taught":true,
    "time_of_day":"11:00","end_time":"13:00",
    "also_units":[{"unit_id":"MR-BIT3110"},{"unit_name":"MRtest Typed Code"}]}')
PID=$(jget patrol_id <<<"$R")
has   "the lecture is recorded"                 '"status":"RECORDED"' "$R"
check "both extra units are stored"             "$(sql "SELECT count(*) FROM monitor_log_units WHERE patrol_id='$PID'")" "2"
check "…the picked one resolved to its department" \
  "$(sql "SELECT department FROM monitor_log_units WHERE patrol_id='$PID' AND unit_id='MR-BIT3110'")" "MRtest Information Technology"
check "…and the typed one is kept as typed"     "$(sql "SELECT count(*) FROM monitor_log_units WHERE patrol_id='$PID' AND resolved = false")" "1"
# ONE college, named ONCE — both codes are in MRtest College.
check "the college is listed once, not twice"   "$(python3 -c "import json,sys;print(len(json.load(sys.stdin)['schools']))" <<<"$R")" "1"
check "…but both departments are listed"        "$(python3 -c "import json,sys;print(len(json.load(sys.stdin)['departments']))" <<<"$R")" "2"
check "the primary unit's department is inherited" "$(sql "SELECT department FROM lecturer_patrol_logs WHERE patrol_id='$PID'")" "MRtest Computer Science"
check "the lecture's end time is on the record" "$(sql "SELECT end_time FROM lecturer_patrol_logs WHERE patrol_id='$PID'")" "13:00"
# Re-filing REPLACES the extras rather than merging, so a mis-added unit can be taken off again.
R=$(body POST /api/v1/patrol/manual "$MON" \
  '{"unit_id":"MR-CSC3103","lecturer_staff_id":"MR-LEC","students_counted":30,"taught":true,
    "time_of_day":"11:00","also_units":[{"unit_id":"MR-BIT3110"}]}')
check "re-filing removes a unit added by mistake" "$(sql "SELECT count(*) FROM monitor_log_units WHERE patrol_id='$PID'")" "1"
# The same unit twice is a mis-tap, not two lectures.
R=$(body POST /api/v1/patrol/manual "$MON" \
  '{"unit_id":"MR-CSC3103","lecturer_staff_id":"MR-LEC","students_counted":30,"taught":true,
    "time_of_day":"11:00","also_units":[{"unit_id":"MR-CSC3103"}]}')
check "the primary unit cannot be added to itself" "$(sql "SELECT count(*) FROM monitor_log_units WHERE patrol_id='$PID'")" "0"
# A lecture that "ends" before it began is a typo, and would put a negative hour into reporting.
R=$(body POST /api/v1/patrol/manual "$MON" \
  '{"unit_id":"MR-CSC3103","lecturer_staff_id":"MR-LEC","students_counted":5,"taught":true,
    "time_of_day":"15:00","end_time":"14:00"}')
check "an end before the start is dropped, not stored" \
  "$(sql "SELECT COALESCE(end_time,'-') FROM lecturer_patrol_logs WHERE unit_id='MR-CSC3103' AND scheduled_time='15:00'")" "-"

echo; echo "── 4. the lecturer is asked before the accusation arrives ──"
UR=$(body GET /api/v1/lecturer/unrecorded "$LEC")
has "the elapsed lecture is offered to account for" 'MR-CSC3103' "$UR"
has "…with the hour it was due to run"             "\"start_time\":\"$GONE\"" "$UR"
check "…and nothing said the lecturer was there"   \
  "$(python3 -c "import json,sys;print(json.load(sys.stdin)['lectures'][0]['session_opened'])" <<<"$UR")" "False"
# A lecture still to come today is NOT offered — telling someone they missed a class they are about
# to teach is both wrong and insulting.
docker exec -i "$PG" psql -U qaat -d qaat -q -c \
  "INSERT INTO timetable_slots (tenant_id, offering_id, unit_id, day_of_week, start_time, duration_minutes, room, lecturer_id)
   SELECT '$TEN','$O_CS','MR-CSC3103',$DOW,'23:30',60,'MR Hall 9', lecturer_id FROM lecturers WHERE staff_id='MR-LEC'" >/dev/null 2>&1
UR=$(body GET /api/v1/lecturer/unrecorded "$LEC")
hasnt "a lecture still ahead of them is not offered" '"start_time":"23:30"' "$UR"
# Once the gate record exists, it disappears — it is no longer unaccounted for.
docker exec -i "$PG" psql -U qaat -d qaat -q -c \
  "INSERT INTO sessions (tenant_id, coordinator_id, unit_id, session_date, session_status, offering_id)
   VALUES ('$TEN','00000000-0000-0000-0000-000000000001','MR-CSC3103','$TODAY','CLOSED','$O_CS')" >/dev/null 2>&1
docker exec -i "$PG" psql -U qaat -d qaat -q -c \
  "INSERT INTO lecturer_attendance_logs (tenant_id, session_id, lecturer_id, gate_open_time, unit_id, session_date, lecturer_scanned_at)
   SELECT '$TEN', s.session_id, l.lecturer_id::text, now(), 'MR-CSC3103','$TODAY', now()
     FROM sessions s, lecturers l WHERE s.unit_id='MR-CSC3103' AND l.staff_id='MR-LEC' LIMIT 1" >/dev/null 2>&1
UR=$(body GET /api/v1/lecturer/unrecorded "$LEC")
hasnt "a lecture they gated in for drops off the list" '"unit_id":"MR-CSC3103"' "$UR"

echo; echo "── 5. the not-taught notification carries its reply ──"
R=$(body POST /api/v1/patrol/sync "$MON" \
  "{\"logs\":[{\"unit_id\":\"MR-BIT3110\",\"unit_name\":\"MRtest Research Methodology\",\"lecturer_id\":\"MR-LEC\",
     \"session_date\":\"$TODAY\",\"scheduled_time\":\"$GONE\",\"taught\":false,\"taken_at\":\"$(date -u +%FT%TZ)\"}]}")
has "the monitor's not-taught tick lands" '"records_written":1' "$R"
check "the notification carries an action" \
  "$(sql "SELECT COALESCE(action,'-') FROM app_notifications WHERE subject LIKE '%MRtest Research Methodology%' ORDER BY created_at DESC LIMIT 1")" "APPEAL_NOT_TAUGHT"
check "…pointing at the exact lecture" \
  "$(sql "SELECT COALESCE(action_ref,'-') FROM app_notifications WHERE subject LIKE '%MRtest Research Methodology%' ORDER BY created_at DESC LIMIT 1")" "MR-BIT3110|$TODAY|$GONE"
has "…and the lecturer's inbox shows it" 'APPEAL_NOT_TAUGHT' "$(body GET /api/v1/app-notifications "$LEC")"

echo; echo "── 6. the roster sieves to the lecturer's own cohorts ──"
RO=$(body GET "/api/v1/lecturer/roster?with_overall=1" "$LEC")
has   "their own cohort's students are there"  'MR Student One' "$RO"
hasnt "another cohort's student is NOT"        'MR Evening Student' "$RO"
# And the filters cut it further, in SQL — a filter that only hid rows locally would still have
# counted them into the percentages.
RO=$(body GET "/api/v1/lecturer/roster?with_overall=1&session_type=Evening" "$LEC")
check "filtering to a cohort they do not teach yields nobody" \
  "$(python3 -c "import json,sys;print(len(json.load(sys.stdin)['students']))" <<<"$RO")" "0"
RO=$(body GET "/api/v1/lecturer/roster?with_overall=1&intake=August%20Intake" "$LEC")
has "filtering to their own intake keeps them" 'MR Student One' "$RO"

echo; echo "── 7. percentages, per unit and overall ──"
SID=$(sql "SELECT session_id FROM sessions WHERE unit_id='MR-CSC3103' LIMIT 1")
docker exec -i "$PG" psql -U qaat -d qaat -q -c \
  "INSERT INTO attendance_logs (tenant_id, session_id, student_id, checkin_timestamp, sequence_number, entry_method)
   VALUES ('$TEN','$SID','MR-S1', now(), 1, 'AUTHENTICATED')" >/dev/null 2>&1
RO=$(body GET "/api/v1/lecturer/roster?with_overall=1&unit_id=MR-CSC3103" "$LEC")
check "the student who came is at 100% for that unit" \
  "$(python3 -c "
import json,sys
d=json.load(sys.stdin)
print([r['pct'] for r in d['students'] if r['student_id']=='MR-S1'][0])" <<<"$RO")" "100"
check "the one who did not is at 0%" \
  "$(python3 -c "
import json,sys
d=json.load(sys.stdin)
print([r['pct'] for r in d['students'] if r['student_id']=='MR-S2'][0])" <<<"$RO")" "0"
check "the class figure is the weighted total" \
  "$(python3 -c "import json,sys;print(json.load(sys.stdin)['class_pct'])" <<<"$RO")" "50"
# The student's own view: per unit AND one overall figure they do not have to work out.
SP=$(curl -sk "$BASE/api/v1/student/progress?reg=MR-S1" -m 30)
has "the student's progress carries an overall block" '"overall"' "$SP"
has "…with a percentage"                              '"percentage"' "$SP"
has "…and the count of units that could stop them"    '"units_at_risk"' "$SP"

echo; echo "── 8. the lecturer writes to ONE coordinator, by name ──"
RC=$(body GET /api/v1/lecturer/recipients "$LEC")
has   "their own cohort's coordinator is offered"  'MR Day Coordinator' "$RC"
hasnt "a coordinator of a cohort they do not teach is NOT" 'MR Evening Coordinator' "$RC"
has   "…and each one carries the cohort they run"  '"cohort":"Day' "$RC"
has   "their own students are offered"             'MR Student One' "$RC"
hasnt "another cohort's student is NOT"            'MR Evening Student' "$RC"

C1=$(sql "SELECT user_id FROM users WHERE email='mr.c1@$DOM'")
C2=$(sql "SELECT user_id FROM users WHERE email='mr.c2@$DOM'")
R=$(body POST /api/v1/app-notifications "$LEC" \
  "{\"audience\":\"COORDINATOR\",\"target_id\":\"$C1\",\"subject\":\"MRtest to one\",\"body\":\"projector\"}")
has   "a message to one coordinator is sent"       '"status":"SENT"' "$R"
check "…to exactly one person"                     "$(python3 -c "import json,sys;print(json.load(sys.stdin)['recipients'])" <<<"$R")" "1"
check "…and it is the one they picked" \
  "$(sql "SELECT count(*) FROM notification_recipients nr JOIN app_notifications n USING (notification_id)
           WHERE n.subject='MRtest to one' AND nr.recipient_user_id='$C1'")" "1"
# The safety property: a target the lecturer has no business writing to is refused rather than
# quietly delivered, because the target arrives from a client and a client is the least trusted
# thing here.
R=$(body POST /api/v1/app-notifications "$LEC" \
  "{\"audience\":\"COORDINATOR\",\"target_id\":\"$C2\",\"subject\":\"MRtest wrong\",\"body\":\"x\"}")
has "a coordinator outside their cohorts is refused, not silently dropped" 'NO_RECIPIENTS' "$R"
# The broadcast still works, and now reaches only their OWN coordinators.
R=$(body POST /api/v1/app-notifications "$LEC" \
  '{"audience":"COORDINATOR","subject":"MRtest to all","body":"y"}')
check "the broadcast reaches only their own coordinators" \
  "$(python3 -c "import json,sys;print(json.load(sys.stdin)['recipients'])" <<<"$R")" "1"
# One student, by registration number.
R=$(body POST /api/v1/app-notifications "$LEC" \
  '{"audience":"STUDENT","target_id":"MR-S1","subject":"MRtest one student","body":"see me"}')
has   "a message to one student is sent"           '"status":"SENT"' "$R"
check "…to exactly that student"                   "$(python3 -c "import json,sys;print(json.load(sys.stdin)['recipients'])" <<<"$R")" "1"
R=$(body POST /api/v1/app-notifications "$LEC" \
  '{"audience":"STUDENT","target_id":"MR-S3","subject":"MRtest wrong student","body":"z"}')
has "a student they do not teach is refused too" 'NO_RECIPIENTS' "$R"
R=$(body POST /api/v1/app-notifications "$LEC" \
  '{"audience":"STUDENTS","subject":"MRtest whole class","body":"all"}')
check "the whole class is still reachable in one go" \
  "$(python3 -c "import json,sys;print(json.load(sys.stdin)['recipients'])" <<<"$R")" "2"

echo
echo "── done ── passed: $pass, failed: $fail"
[ "$fail" -eq 0 ]
