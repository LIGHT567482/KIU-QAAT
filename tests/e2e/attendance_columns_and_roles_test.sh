#!/usr/bin/env bash
#
# Seven things the institution asked for, end to end against a running stack.
#
#   1. how many STUDENTS attended, on the attendance log the admin reads
#   2. a CLASS/GROUP column saying which cohort it was — "2:1" for year 2 semester 1
#   3. COMPENSATION lectures, registered by the QA monitor and reflected in the lecturer's log
#   4. the QA MONITOR named on every attendance record and in the download
#   5. a TLC in each department, designing that department's timetable and no one else's
#   6. a STAFF ID on staff accounts, usable to sign in
#   7. "patroller" is called MONITOR everywhere a person reads it
#
# What these assertions are really for: each column is a fact the records could not previously
# state, and a report that cannot say "the lecturer was there and nobody came" or "that was a
# compensation" produces confident, wrong conclusions rather than obviously missing ones.
#
# Usage:  ./attendance_columns_and_roles_test.sh        (against https://localhost:8443)
# Seeds and removes its own COLT- fixtures via the postgres container.
set -uo pipefail
BASE="${1:-https://localhost:8443}"; PG="${PG_CONTAINER:-infra-postgres-1}"; pass=0; fail=0
check(){ if [ "$2" = "$3" ]; then echo "   ✓ $1"; pass=$((pass+1)); else echo "   ✗ $1 (got '$2' want '$3')"; fail=$((fail+1)); fi; }
has(){ if grep -qF -- "$2" <<<"$3"; then echo "   ✓ $1"; pass=$((pass+1)); else echo "   ✗ $1 — '$2' not in: ${3:0:400}"; fail=$((fail+1)); fi; }
hasnt(){ if grep -qF -- "$2" <<<"$3"; then echo "   ✗ $1 — found '$2'"; fail=$((fail+1)); else echo "   ✓ $1"; pass=$((pass+1)); fi; }
sql(){ docker exec -i "$PG" psql -U qaat -d qaat -tAc "$1"; }
jq_(){ python3 -c "import json,sys$1"; }

cleanup(){ docker exec -i "$PG" psql -U qaat -d qaat -q >/dev/null 2>&1 <<'SQL'
DELETE FROM attendance_logs        WHERE session_id IN (SELECT session_id FROM sessions WHERE unit_id LIKE 'COLT-%');
DELETE FROM lecturer_attendance_logs WHERE unit_id LIKE 'COLT-%';
DELETE FROM lecturer_patrol_logs   WHERE unit_id LIKE 'COLT-%';
DELETE FROM sessions               WHERE unit_id LIKE 'COLT-%';
DELETE FROM timetable_slots        WHERE unit_id LIKE 'COLT-%';
DELETE FROM lecturer_assignments   WHERE unit_id LIKE 'COLT-%';
DELETE FROM offering_unit_schedules WHERE unit_id LIKE 'COLT-%';
DELETE FROM students_extended      WHERE student_id LIKE 'COLT-%';
DELETE FROM course_units           WHERE unit_id LIKE 'COLT-%';
DELETE FROM course_offerings       WHERE course_id = 'COLT-COURSE';
DELETE FROM courses                WHERE course_id IN ('COLT-COURSE','COLT-OTHER');
DELETE FROM lecturers              WHERE staff_id LIKE 'COLT-%';
DELETE FROM users                  WHERE email LIKE 'colt.%';
SQL
}
trap cleanup EXIT; cleanup

TEN=$(sql "SELECT tenant_id FROM tenants WHERE tenant_id <> '00000000-0000-0000-0000-000000000000' ORDER BY created_at LIMIT 1")
OFF='cc111111-0000-4000-8000-0000000c01a1'
TODAY=$(date +%F)

docker exec -i "$PG" psql -U qaat -d qaat -q -v ON_ERROR_STOP=1 >/dev/null <<SQL
CREATE EXTENSION IF NOT EXISTS pgcrypto;
-- Two departments, so a TLC in one can be shown NOT to reach the other.
INSERT INTO courses (course_id, tenant_id, name, department, school) VALUES
 ('COLT-COURSE','$TEN','Column Test Course','COLTEST Computing','COLTEST School'),
 ('COLT-OTHER', '$TEN','Column Other Course','COLTEST Nursing','COLTEST School');
INSERT INTO course_units (unit_id, tenant_id, course_id, name, year, semester) VALUES
 ('COLT-UNIT','$TEN','COLT-COURSE','Column Test Unit',2,1),
 ('COLT-OTHERUNIT','$TEN','COLT-OTHER','Other Department Unit',1,2);
-- Year 2, semester 1 → the class/group must read "2:1".
INSERT INTO course_offerings (offering_id, tenant_id, course_id, session_type, study_year, semester, intake)
 VALUES ('$OFF','$TEN','COLT-COURSE','Day',2,1,'August Intake');
INSERT INTO users (tenant_id, email, password_hash, role, full_name, is_active, staff_id, department, school, force_password_change) VALUES
 ('$TEN','colt.admin@kiu.ac.ug',   crypt('AdmPass12345', gen_salt('bf',10)),'ADMIN',       'COLT Admin',   true,'COLT-ADM',NULL,NULL,false),
 ('$TEN','colt.qa@kiu.ac.ug',      crypt('QaPass12345',  gen_salt('bf',10)),'QA_OFFICER',  'COLT QA',      true,'COLT-QA','COLTEST Computing','COLTEST School',false),
 ('$TEN','colt.monitor@kiu.ac.ug', crypt('MonPass12345', gen_salt('bf',10)),'QA_PATROLLER','COLT Monitor', true,'COLT-MON',NULL,NULL,false),
 ('$TEN','colt.tlc@kiu.ac.ug',     crypt('TlcPass12345', gen_salt('bf',10)),'TLC',         'COLT TLC',     true,'COLT-TLC','COLTEST Computing','COLTEST School',false),
 ('$TEN','colt.coord@kiu.ac.ug',   crypt('CooPass12345', gen_salt('bf',10)),'COORDINATOR', 'COLT Coord',   true,'COLT-CRD',NULL,NULL,false),
 ('$TEN','colt.lect@kiu.ac.ug',    crypt('LecPass12345', gen_salt('bf',10)),'LECTURER',    'COLT Lecturer',true,'COLT-LEC',NULL,NULL,false);
INSERT INTO lecturers (tenant_id, full_name, email, staff_id, department, user_id)
  SELECT '$TEN','COLT Lecturer','colt.lect@kiu.ac.ug','COLT-LEC','COLTEST Computing', user_id
    FROM users WHERE email='colt.lect@kiu.ac.ug';
SQL

LID=$(sql "SELECT lecturer_id FROM lecturers WHERE staff_id='COLT-LEC' AND tenant_id='$TEN'")
CRD=$(sql "SELECT user_id FROM users WHERE email='colt.coord@kiu.ac.ug'")
docker exec -i "$PG" psql -U qaat -d qaat -q -v ON_ERROR_STOP=1 >/dev/null <<SQL
INSERT INTO lecturer_assignments (tenant_id, lecturer_id, unit_id, course_id, academic_year, year, semester, intake_session)
  VALUES ('$TEN','$LID','COLT-UNIT','COLT-COURSE','2025/2026',2,1,'Day');
-- Three students on the cohort; TWO of them will be marked present.
INSERT INTO students_extended (student_id, tenant_id, full_name, email, course_id, academic_year, offering_id, current_year, semester)
 VALUES ('COLT-S1','$TEN','COLT One','colt.s1@studmc.kiu.ac.ug','COLT-COURSE','2025/2026','$OFF',2,1),
        ('COLT-S2','$TEN','COLT Two','colt.s2@studmc.kiu.ac.ug','COLT-COURSE','2025/2026','$OFF',2,1),
        ('COLT-S3','$TEN','COLT Three','colt.s3@studmc.kiu.ac.ug','COLT-COURSE','2025/2026','$OFF',2,1);
INSERT INTO sessions (session_id, tenant_id, coordinator_id, unit_id, lecturer_id, offering_id, session_date, session_status, gate_open_time)
 VALUES ('cc222222-0000-4000-8000-0000000c01a2','$TEN','$CRD','COLT-UNIT','$LID','$OFF','$TODAY','CLOSED', now());
INSERT INTO attendance_logs (tenant_id, session_id, student_id, checkin_timestamp, sequence_number)
 VALUES ('$TEN','cc222222-0000-4000-8000-0000000c01a2','COLT-S1', now(), 1),
        ('$TEN','cc222222-0000-4000-8000-0000000c01a2','COLT-S2', now(), 2);
INSERT INTO lecturer_attendance_logs (tenant_id, session_id, lecturer_id, unit_id, session_date, gate_open_time, contact_hours)
 VALUES ('$TEN','cc222222-0000-4000-8000-0000000c01a2','$LID','COLT-UNIT','$TODAY', now(), 2);
SQL

login(){ curl -sk -X POST "$BASE/api/v1/auth/app-login" -H 'Content-Type: application/json' -m 30 \
  -d "{\"identifier\":\"$1\",\"password\":\"$2\",\"org\":\"\"}" \
  | python3 -c "import json,sys;print(json.load(sys.stdin).get('access_token',''))"; }
body(){ curl -sk -X "$1" "$BASE$2" -H "Authorization: Bearer $3" \
  ${4:+-H 'Content-Type: application/json' -d "$4"} -m 60; }
code(){ curl -sk -o /dev/null -w '%{http_code}' -X "$1" "$BASE$2" -H "Authorization: Bearer $3" \
  ${4:+-H 'Content-Type: application/json' -d "$4"} -m 60; }

echo; echo "── 6. STAFF ID signs in (every role, no email typed) ──"
ADM=$(login COLT-ADM AdmPass12345); QA=$(login COLT-QA QaPass12345)
MON=$(login COLT-MON MonPass12345); TLC=$(login COLT-TLC TlcPass12345)
for pair in "admin:$ADM" "qa:$QA" "monitor:$MON" "tlc:$TLC"; do
  n="${pair%%:*}"; t="${pair#*:}"
  if [ -n "$t" ]; then echo "   ✓ $n signs in with a staff ID"; pass=$((pass+1));
  else echo "   ✗ $n could NOT sign in with a staff ID"; fail=$((fail+1)); fi
done
has "the admin's user list carries the staff ID" '"staff_id":"COLT-QA"' "$(body GET "/api/v1/admin/tenants/$TEN/users" "$ADM")"
R=$(body POST "/api/v1/admin/tenants/$TEN/users" "$ADM" \
   '{"email":"colt.new@kiu.ac.ug","password":"NewPass12345","role":"DVC","full_name":"COLT New","staff_id":"COLT-NEW"}')
has "an account can be CREATED with a staff ID" '"status":"CREATED"' "$R"
check "…and it is stored" "$(sql "SELECT staff_id FROM users WHERE email='colt.new@kiu.ac.ug'")" "COLT-NEW"

echo; echo "── 1 & 2 & 4. students attended, class/group and the QA monitor, on the log ──"
LOG=$(body GET "/api/v1/admin/tenants/$TEN/lecturer-attendance" "$ADM")
ROW=$(python3 -c "
import json,sys
rows=[r for r in json.load(sys.stdin) if r['unit_id']=='COLT-UNIT']
print(json.dumps(rows[0], separators=(',',':')) if rows else '{}')" <<<"$LOG")
echo "   $ROW"
has "students attended is on the admin's log"   '"students_attended":2' "$ROW"
has "class/group reads YEAR:SEMESTER"           '"class_group":"2:1"'   "$ROW"
has "a QA monitor column exists"                '"qa_monitor"'          "$ROW"
has "…and a compensation flag"                  '"is_compensation"'     "$ROW"

echo; echo "── 3. the QA MONITOR registers a compensation ──"
curl -sk -X POST "$BASE/api/v1/patrol/bind-device" -H "Authorization: Bearer $MON" \
  -H 'Content-Type: application/json' -d '{"device_fingerprint":"colt-handset"}' -m 30 >/dev/null
R=$(body POST /api/v1/patrol/sync "$MON" \
  "{\"logs\":[{\"unit_id\":\"COLT-UNIT\",\"unit_name\":\"Column Test Unit\",\"course_code\":\"COLT-COURSE\",
    \"lecturer_id\":\"COLT-LEC\",\"lecturer_name\":\"COLT Lecturer\",\"room\":\"LR-COLT\",
    \"session_date\":\"$TODAY\",\"scheduled_time\":\"10:00\",\"taught\":true,
    \"offering_id\":\"$OFF\",\"is_compensation\":true,\"compensation_for\":\"2026-08-01\"}]}")
has "the monitor's round accepts a compensation" '"records_written":1' "$R"
check "…and it is stored as one" "$(sql "SELECT is_compensation||'/'||compensation_for FROM lecturer_patrol_logs WHERE unit_id='COLT-UNIT'")" "true/2026-08-01"

echo; echo "── 3 & 4. it reflects on the LECTURER ATTENDANCE LOG ──"
ROW=$(python3 -c "
import json,sys
rows=[r for r in json.load(sys.stdin) if r['unit_id']=='COLT-UNIT']
print(json.dumps(rows[0], separators=(',',':')) if rows else '{}')" <<<"$(body GET "/api/v1/admin/tenants/$TEN/lecturer-attendance" "$ADM")")
has "the log now says compensation"          '"is_compensation":true'        "$ROW"
has "…naming the lecture it makes good"      '"compensation_for":"2026-08-01"' "$ROW"
has "…and naming the QA monitor"             '"qa_monitor":"COLT Monitor"'   "$ROW"
has "…with the monitor's staff id"           '"qa_monitor_staff_id":"COLT-MON"' "$ROW"

ROW=$(python3 -c "
import json,sys
rows=[r for r in json.load(sys.stdin) if r['unit_id']=='COLT-UNIT']
print(json.dumps(rows[0], separators=(',',':')) if rows else '{}')" <<<"$(body GET /api/v1/dashboard/lecturer-attendance "$QA")")
has "the oversight dashboard carries them too" '"qa_monitor":"COLT Monitor"' "$ROW"
has "…and the student count"                   '"students_attended":2'      "$ROW"

echo; echo "── 4. the LECTURER's own dashboard sees the monitor and the compensation ──"
LEC=$(login COLT-LEC LecPass12345)
S=$(body GET "/api/v1/lecturer/sessions?unit_id=COLT-UNIT" "$LEC")
has "the lecturer's session list names the monitor" '"qa_monitor":"COLT Monitor"' "$S"
has "…marks the compensation"                       '"is_compensation":true'      "$S"
has "…carries the class/group"                      '"class_group":"2:1"'         "$S"
has "…and how many students came"                   '"present_count":2'           "$S"

echo; echo "── 4. the QA monitor column is in the DOWNLOAD ──"
CSV=$(curl -sk "$BASE/api/v1/dashboard/lecturer-attendance/patrol/export.csv" -H "Authorization: Bearer $QA" -m 60)
has "the export has a QA Monitor column"   'QA Monitor'   "$CSV"
has "…a Class/Group column"                'Class/Group'  "$CSV"
has "…a Compensation column"               'Compensation' "$CSV"
has "…a Students column"                   'Students'     "$CSV"
has "…and the monitor's name in the data"  'COLT Monitor' "$CSV"
has "…with the compensation marked"        'Yes'          "$CSV"
for f in xlsx pdf; do
  C=$(curl -sk -o /dev/null -w '%{http_code}' "$BASE/api/v1/dashboard/lecturer-attendance/patrol/export.$f" -H "Authorization: Bearer $QA" -m 60)
  check "the .$f download still works" "$C" "200"
done

echo; echo "── 5. a TLC designs their OWN department's timetable ──"
DOW=$(date +%u)
check "the TLC may set their department's lecture" \
  "$(code PUT /api/v1/dashboard/timetable "$TLC" "{\"offering_id\":\"$OFF\",\"unit_id\":\"COLT-UNIT\",\"day_of_week\":$DOW,\"session_start\":\"11:00\",\"session_duration_minutes\":60}")" "200"
check "…and it reaches the QA monitor's round" \
  "$(sql "SELECT count(*) FROM timetable_slots WHERE unit_id='COLT-UNIT'")" "1"
R=$(body PUT /api/v1/dashboard/timetable "$TLC" "{\"offering_id\":\"$OFF\",\"unit_id\":\"COLT-OTHERUNIT\",\"day_of_week\":$DOW,\"session_start\":\"11:00\",\"session_duration_minutes\":60}")
has "…but NOT another department's" 'OUT_OF_DEPARTMENT' "$R"
has "…and the refusal says whose it is" 'COLTEST Nursing' "$R"
check "…nothing was written for the other department" \
  "$(sql "SELECT count(*) FROM timetable_slots WHERE unit_id='COLT-OTHERUNIT'")" "0"
TLCGRID=$(body GET /api/v1/dashboard/timetable/slots "$TLC" | tr -d '\n')
if grep -qF '"tlc_department":"COLTEST Computing"' <<<"$TLCGRID"; then
  echo "   ✓ the grid tells the TLC which department is theirs"; pass=$((pass+1))
else
  echo "   ✗ the grid tells the TLC which department is theirs — tail: ${TLCGRID: -160}"; fail=$((fail+1))
fi
# An admin is not a departmental TLC and keeps the whole institution.
check "an admin may still timetable any department" \
  "$(code PUT /api/v1/dashboard/timetable "$ADM" "{\"offering_id\":\"$OFF\",\"unit_id\":\"COLT-OTHERUNIT\",\"day_of_week\":$DOW,\"session_start\":\"12:00\",\"session_duration_minutes\":60}")" "200"

echo; echo "── 7. the word is MONITOR, not patroller ──"
hasnt "the monitor download says nothing about patrols" 'Patrolled' "$CSV"
# The PDF's text lives in a Flate-compressed stream, so the title is only readable after
# inflating it — grepping the raw bytes would pass or fail for reasons unrelated to the wording.
PDFTEXT=$(curl -sk "$BASE/api/v1/dashboard/lecturer-attendance/patrol/export.pdf" -H "Authorization: Bearer $QA" -m 60 \
  | python3 -c "
import re,sys,zlib
raw=sys.stdin.buffer.read()
out=[]
for m in re.finditer(rb'stream\r?\n(.*?)endstream', raw, re.S):
    try: out.append(zlib.decompress(m.group(1)).decode('latin-1'))
    except Exception: pass
print(' '.join(out))")
has   "…the PDF is titled 'QA monitor record'"           'QA monitor record' "$PDFTEXT"
JS=$(docker exec infra-admin-dashboards-1 sh -c 'cat /usr/share/nginx/html/assets/*.js' 2>/dev/null)
if [ -n "$JS" ]; then
  has   "the dashboard says 'QA Monitor'"        'QA Monitor'          "$JS"
  has   "…and 'Message the QA monitors'"         'Message the QA monitors' "$JS"
  hasnt "…and no longer 'Message the patrollers'" 'Message the patrollers'  "$JS"
  hasnt "…nor 'Patroller handsets'"               'Patroller handsets'      "$JS"
else
  echo "   — dashboard bundle not readable, skipping the wording check"
fi

echo; echo "=== $pass passed, $fail failed ==="
[ "$fail" -eq 0 ]
