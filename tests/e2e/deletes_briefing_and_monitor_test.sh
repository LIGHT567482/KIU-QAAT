#!/usr/bin/env bash
#
# Three things that were reported as broken from the dashboards, end to end against a running
# stack. All three share a failure mode that looks like nothing at all from the browser:
#
#   · the DELETE buttons on the lecturer pages — a stale row id reached `::uuid` unvalidated and
#     came back as a 500 carrying the raw Postgres error, so "Delete" reported an internal fault
#     for what is simply a page that has not been reloaded
#
#   · QA/DQA messaging the patrollers — the whole path, send through to dismiss, asserted NOT to
#     answer 404. A 404 here never means "not found": every one of these routes is unconditional,
#     so a 404 means the gateway being served is OLDER than the source and does not carry the
#     feature at all. That is worth failing loudly for, because the UI can only report "HTTP 404"
#
#   · the timetable reaching the patrol round — a lecture set on the Timetable page wrote only
#     offering_unit_schedules, while /patrol/manifest and /patrol/search read only
#     timetable_slots. The lecture was accepted, displayed, locked… and unpatrollable
#
# Usage:  ./deletes_briefing_and_patrol_test.sh        (against https://localhost:8443)
# Seeds and removes its own DELT-/BRIEF- fixtures via the postgres container.
set -uo pipefail
BASE="${1:-https://localhost:8443}"; PG="${PG_CONTAINER:-infra-postgres-1}"; pass=0; fail=0
check(){ if [ "$2" = "$3" ]; then echo "   ✓ $1"; pass=$((pass+1)); else echo "   ✗ $1 (got '$2' want '$3')"; fail=$((fail+1)); fi; }
has(){ if grep -qF -- "$2" <<<"$3"; then echo "   ✓ $1"; pass=$((pass+1)); else echo "   ✗ $1 — '$2' not in: ${3:0:300}"; fail=$((fail+1)); fi; }
hasnt(){ if grep -qF -- "$2" <<<"$3"; then echo "   ✗ $1 — found '$2'"; fail=$((fail+1)); else echo "   ✓ $1"; pass=$((pass+1)); fi; }
sql(){ docker exec -i "$PG" psql -U qaat -d qaat -tAc "$1"; }

cleanup(){ docker exec -i "$PG" psql -U qaat -d qaat -q >/dev/null 2>&1 <<'SQL'
DELETE FROM notification_recipients WHERE notification_id IN (SELECT notification_id FROM app_notifications WHERE subject LIKE 'BRIEFTEST%');
DELETE FROM app_notifications WHERE subject LIKE 'BRIEFTEST%';
DELETE FROM lecturer_attendance_logs WHERE unit_id LIKE 'DELTEST%';
DELETE FROM sessions              WHERE unit_id LIKE 'DELTEST%';
DELETE FROM timetable_slots       WHERE unit_id LIKE 'DELTEST%';
DELETE FROM lecturer_assignments  WHERE unit_id LIKE 'DELTEST%';
DELETE FROM offering_unit_schedules WHERE unit_id LIKE 'DELTEST%';
DELETE FROM course_units          WHERE unit_id LIKE 'DELTEST%';
DELETE FROM course_offerings      WHERE course_id = 'DELTEST-COURSE';
DELETE FROM courses               WHERE course_id = 'DELTEST-COURSE';
DELETE FROM lecturers             WHERE staff_id LIKE 'DELT-%';
DELETE FROM users                 WHERE email LIKE 'delt.%' OR email LIKE 'brief.%';
SQL
}
trap cleanup EXIT; cleanup

TEN=$(sql "SELECT tenant_id FROM tenants WHERE tenant_id <> '00000000-0000-0000-0000-000000000000' ORDER BY created_at LIMIT 1")
OFF='dddddddd-0000-4000-8000-0000000de111'

docker exec -i "$PG" psql -U qaat -d qaat -q -v ON_ERROR_STOP=1 >/dev/null <<SQL
CREATE EXTENSION IF NOT EXISTS pgcrypto;
INSERT INTO users (tenant_id, email, password_hash, role, full_name, is_active, staff_id, force_password_change) VALUES
 ('$TEN','delt.admin@kiu.ac.ug', crypt('AdmPass12345', gen_salt('bf',10)),'ADMIN',       'DelTest Admin',    true,'DELT-ADM',false),
 ('$TEN','brief.qa@kiu.ac.ug',   crypt('QaPass12345',  gen_salt('bf',10)),'QA_OFFICER',  'BriefTest QA',     true,'BRIEF-QA',false),
 ('$TEN','brief.dqa@kiu.ac.ug',  crypt('DqaPass12345', gen_salt('bf',10)),'DQA_DIRECTOR','BriefTest DQA',    true,'BRIEF-DQA',false),
 ('$TEN','brief.p1@kiu.ac.ug',   crypt('PatPass12345', gen_salt('bf',10)),'QA_PATROLLER','BriefTest Patrol', true,'BRIEF-P1',false),
 ('$TEN','delt.lect@kiu.ac.ug',  crypt('LecPass12345', gen_salt('bf',10)),'LECTURER',    'DelTest Lecturer', true,'DELT-001',false);
INSERT INTO courses (course_id, tenant_id, name) VALUES ('DELTEST-COURSE','$TEN','Delete Test Course');
INSERT INTO course_units (unit_id, tenant_id, course_id, name)
  VALUES ('DELTEST-UNIT','$TEN','DELTEST-COURSE','Delete Test Unit');
INSERT INTO course_offerings (offering_id, tenant_id, course_id, session_type, study_year, semester)
  VALUES ('$OFF','$TEN','DELTEST-COURSE','Day',1,1);
INSERT INTO lecturers (tenant_id, full_name, email, staff_id, department, user_id)
  SELECT '$TEN','DelTest Lecturer','delt.lect@kiu.ac.ug','DELT-001','Computer Science', user_id
    FROM users WHERE email='delt.lect@kiu.ac.ug';
INSERT INTO lecturers (tenant_id, full_name, email, staff_id, department) VALUES
 ('$TEN','DelTest Bulk A','delt.a@kiu.ac.ug','DELT-002','Computer Science'),
 ('$TEN','DelTest Bulk B','delt.b@kiu.ac.ug','DELT-003','Computer Science');
SQL

LID=$(sql "SELECT lecturer_id FROM lecturers WHERE staff_id='DELT-001' AND tenant_id='$TEN'")
UID_=$(sql "SELECT user_id FROM users WHERE email='delt.lect@kiu.ac.ug'")
docker exec -i "$PG" psql -U qaat -d qaat -q -v ON_ERROR_STOP=1 >/dev/null <<SQL
INSERT INTO lecturer_assignments (tenant_id, lecturer_id, unit_id, course_id, academic_year, year, semester, intake_session)
  VALUES ('$TEN','$LID','DELTEST-UNIT','DELTEST-COURSE','2025/2026',1,1,'Day');
INSERT INTO timetable_slots (tenant_id, offering_id, unit_id, lecturer_id, day_of_week, start_time, duration_minutes, room)
  VALUES ('$TEN','$OFF','DELTEST-UNIT','$LID',3,'10:00',60,'LR-DEL');
INSERT INTO sessions (tenant_id, coordinator_id, unit_id, lecturer_id, session_date, session_status)
  SELECT '$TEN', user_id::text, 'DELTEST-UNIT', '$LID', CURRENT_DATE, 'CLOSED'
    FROM users WHERE role='COORDINATOR' AND tenant_id='$TEN' LIMIT 1;
INSERT INTO lecturer_attendance_logs (tenant_id, session_id, lecturer_id, unit_id, session_date, gate_open_time)
  SELECT '$TEN', session_id, '$LID', 'DELTEST-UNIT', CURRENT_DATE, now()
    FROM sessions WHERE unit_id='DELTEST-UNIT' LIMIT 1;
SQL

login(){ curl -sk -X POST "$BASE/api/v1/auth/login" -H 'Content-Type: application/json' -m 30 \
  -d "{\"email\":\"$1\",\"password\":\"$2\",\"tenant_id\":\"$TEN\"}" \
  | python3 -c "import json,sys;print(json.load(sys.stdin).get('access_token',''))"; }
ADM=$(login delt.admin@kiu.ac.ug AdmPass12345)
QA=$(login brief.qa@kiu.ac.ug QaPass12345)
DQA=$(login brief.dqa@kiu.ac.ug DqaPass12345)
P1=$(login brief.p1@kiu.ac.ug PatPass12345)
echo "tokens: adm=${#ADM} qa=${#QA} dqa=${#DQA} p1=${#P1}"
code(){ curl -sk -o /dev/null -w '%{http_code}' -X "$1" "$BASE$2" -H "Authorization: Bearer $3" \
  ${4:+-H 'Content-Type: application/json' -d "$4"} -m 30; }
body(){ curl -sk -X "$1" "$BASE$2" -H "Authorization: Bearer $3" \
  ${4:+-H 'Content-Type: application/json' -d "$4"} -m 30; }

echo; echo "── 1. the DELETE buttons on the lecturer pages ──"
AID=$(sql "SELECT assignment_id FROM lecturer_assignments WHERE unit_id='DELTEST-UNIT'")
check "Remove on an assignment succeeds"   "$(code DELETE "/api/v1/admin/lecturer-assignments/$AID" "$ADM")" "204"
check "the assignment is gone"             "$(sql "SELECT count(*) FROM lecturer_assignments WHERE assignment_id='$AID'")" "0"
check "an unknown assignment is 404"       "$(code DELETE "/api/v1/admin/lecturer-assignments/99999999-9999-4999-8999-999999999999" "$ADM")" "404"
# A stale row id is a bad request. It used to reach ::uuid and return 500 with the raw SQLSTATE.
check "a malformed assignment id is 400"   "$(code DELETE "/api/v1/admin/lecturer-assignments/not-a-uuid" "$ADM")" "400"

docker exec -i "$PG" psql -U qaat -d qaat -q >/dev/null <<SQL
INSERT INTO lecturer_assignments (tenant_id, lecturer_id, unit_id, course_id, academic_year, year, semester, intake_session)
  VALUES ('$TEN','$LID','DELTEST-UNIT','DELTEST-COURSE','2025/2026',1,1,'Day');
SQL
check "a malformed lecturer id is 400"     "$(code DELETE "/api/v1/admin/tenants/$TEN/lecturers/not-a-uuid" "$ADM")" "400"
check "…and so is one inside a bulk list"  "$(code POST "/api/v1/admin/tenants/$TEN/lecturers/bulk-delete" "$ADM" '{"lecturer_ids":["not-a-uuid"]}')" "400"
check "an empty bulk selection is refused" "$(code POST "/api/v1/admin/tenants/$TEN/lecturers/bulk-delete" "$ADM" '{"lecturer_ids":[]}')" "400"

R=$(body DELETE "/api/v1/admin/tenants/$TEN/lecturers/$LID" "$ADM")
echo "   $R"
has  "Delete reports one deleted"          '"deleted":1' "$R"
has  "…and the assignment it took"         '"assignments_removed":1' "$R"
has  "…and the sign-in it stopped"         '"logins_deactivated":1' "$R"
check "the lecturer is gone"               "$(sql "SELECT count(*) FROM lecturers WHERE lecturer_id='$LID'")" "0"
# The point of the whole design: removing a person must not remove what they taught.
check "the TEACHING RECORD survives"       "$(sql "SELECT count(*) FROM lecturer_attendance_logs WHERE unit_id='DELTEST-UNIT'")" "1"
check "the LECTURE survives"               "$(sql "SELECT count(*) FROM timetable_slots WHERE unit_id='DELTEST-UNIT'")" "1"
check "…now unattributed, not dangling"    "$(sql "SELECT lecturer_id IS NULL FROM timetable_slots WHERE unit_id='DELTEST-UNIT'")" "t"
check "the login is deactivated not erased" "$(sql "SELECT is_active::text FROM users WHERE user_id='$UID_'")" "false"
check "deleting them again is 404"         "$(code DELETE "/api/v1/admin/tenants/$TEN/lecturers/$LID" "$ADM")" "404"

BULK=$(sql "SELECT '[\"'||string_agg(lecturer_id::text,'\",\"')||'\"]' FROM lecturers WHERE staff_id IN ('DELT-002','DELT-003') AND tenant_id='$TEN'")
R=$(body POST "/api/v1/admin/tenants/$TEN/lecturers/bulk-delete" "$ADM" "{\"lecturer_ids\":$BULK}")
has  "bulk delete removes both"            '"deleted":2' "$R"
check "neither is left"                    "$(sql "SELECT count(*) FROM lecturers WHERE staff_id IN ('DELT-002','DELT-003')")" "0"

echo; echo "── 2. QA/DQA messaging the patrollers — no route may answer 404 ──"
for pair in "GET|/api/v1/dashboard/qa/patrollers|$QA" "GET|/api/v1/dashboard/qa/patrollers|$DQA" \
            "GET|/api/v1/app-notifications|$P1" "GET|/api/v1/app-notifications/unread-count|$P1"; do
  IFS='|' read -r M P T <<<"$pair"
  C=$(code "$M" "$P" "$T")
  if [ "$C" = "404" ]; then echo "   ✗ $M $P answered 404 — this gateway does not carry the feature"; fail=$((fail+1));
  else echo "   ✓ $M $P → $C"; pass=$((pass+1)); fi
done
R=$(body POST /api/v1/app-notifications "$QA" '{"audience":"PATROLLERS","target_id":"","subject":"BRIEFTEST round change","body":"Start at the Science block."}')
hasnt "QA can send to every patroller"     'HTTP 404' "$R"
has   "…and it is SENT"                    '"status":"SENT"' "$R"
has   "the patroller has it"               'BRIEFTEST round change' "$(body GET /api/v1/app-notifications "$P1")"
R=$(body POST /api/v1/app-notifications "$DQA" '{"audience":"PATROLLER","target_id":"BRIEF-P1","subject":"BRIEFTEST handset","body":"Swap it at the QA office."}')
has   "DQA can address one patroller"      '"recipients":1' "$R"
NID=$(body GET /api/v1/app-notifications "$P1" | python3 -c "import json,sys;print(next((n['notification_id'] for n in json.load(sys.stdin) if n['subject']=='BRIEFTEST handset'),''))")
check "marking it read is not a 404"       "$(code POST "/api/v1/app-notifications/$NID/read" "$P1" '{}')" "200"
check "dismissing it is not a 404"         "$(code DELETE "/api/v1/app-notifications/$NID" "$P1")" "200"
check "an empty recipient is refused"      "$(code POST /api/v1/app-notifications "$QA" '{"audience":"PATROLLER","target_id":"","subject":"x","body":"y"}')" "400"

echo; echo "── 3. a timetabled lecture reaches the patrol round ──"
curl -sk -X POST "$BASE/api/v1/patrol/bind-device" -H "Authorization: Bearer $P1" \
  -H 'Content-Type: application/json' -d '{"device_fingerprint":"brieftest-handset"}' -m 30 >/dev/null
DOW=$(body GET /api/v1/patrol/manifest "$P1" | python3 -c "import json,sys;print(json.load(sys.stdin)['day_of_week'])")
# The Timetable page's schedule editor. It used to write offering_unit_schedules ONLY, and the
# patrol round reads timetable_slots ONLY — so this lecture existed for everyone except the
# person sent to observe it.
R=$(body PUT /api/v1/dashboard/timetable "$ADM" \
   "{\"offering_id\":\"$OFF\",\"unit_id\":\"DELTEST-UNIT\",\"day_of_week\":$DOW,\"session_start\":\"13:00\",\"session_duration_minutes\":90}")
has  "the schedule is accepted"            '"schedule_locked":true' "$R"
M=$(body GET /api/v1/patrol/manifest "$P1")
has  "the lecture is on the patrol round"  '"unit_id":"DELTEST-UNIT"' "$M"
has  "…at the time it was set"             '"start_time":"13:00"' "$M"
has  "…with its length"                    '"duration_minutes":90' "$M"
has  "…and the lecturer it is assigned to" 'DELTEST-UNIT' "$(body GET "/api/v1/patrol/search?by=unit&q=DELTEST" "$P1")"
# Moving it must not leave a ghost at the old time for the patroller to mark not-taught.
body PUT /api/v1/dashboard/timetable "$ADM" \
   "{\"offering_id\":\"$OFF\",\"unit_id\":\"DELTEST-UNIT\",\"day_of_week\":$DOW,\"session_start\":\"15:30\",\"session_duration_minutes\":90}" >/dev/null
check "moving it leaves exactly one slot"  "$(sql "SELECT count(*) FROM timetable_slots WHERE unit_id='DELTEST-UNIT' AND offering_id='$OFF'")" "1"
has  "…at the new time"                    '"start_time":"15:30"' "$(body GET /api/v1/patrol/manifest "$P1")"
# "Unset" is a NULL day, not a zero one — writing 0 tripped the column's CHECK and 500'd.
check "clearing the day is accepted"       "$(code PUT /api/v1/dashboard/timetable "$ADM" "{\"offering_id\":\"$OFF\",\"unit_id\":\"DELTEST-UNIT\",\"day_of_week\":0,\"session_start\":\"\",\"session_duration_minutes\":0}")" "200"
hasnt "…and takes it off the round"        '"unit_id":"DELTEST-UNIT"' "$(body GET /api/v1/patrol/manifest "$P1")"

echo; echo "=== $pass passed, $fail failed ==="
[ "$fail" -eq 0 ]
