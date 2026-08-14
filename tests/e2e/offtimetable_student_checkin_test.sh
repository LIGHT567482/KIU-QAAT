#!/usr/bin/env bash
#
# STUDENTS CHECK IN THE SAME WAY, even for a lecture nobody timetabled.
#
# Migration 084 gave the QA monitor a way to record an untimetabled lecture. That fixed the
# OBSERVATION and left the ATTENDANCE behind: the coordinator could only start a session for a unit
# timetabled today, so for exactly that lecture there was no session — no room code, no check-in, no
# register. The students who sat through it had no attendance for it, and attendance decides exam
# eligibility, so the number that governs whether they sit the exam was wrong in the direction that
# costs them. Meanwhile the monitor's headcount — an estimate from a doorway — was the only record.
#
# What this proves, in order:
#   1. the monitor's manual entry writes NOTHING to the student ledger — the two records are
#      independent, as they are designed to be
#   2. an off-timetable lecture can now be opened, and the record says so
#   3. the student check-in flow is IDENTICAL: every gate still applies, and every one of them is
#      asserted here rather than assumed — a relaxed gate would be a far worse bug than a missing
#      session, because it would look like it was working
#
# Usage:  ./offtimetable_student_checkin_test.sh        (against https://localhost:8443)
# Seeds and removes its own OTT- fixtures via the postgres container.
set -uo pipefail
BASE="${1:-https://localhost:8443}"; PG="${PG_CONTAINER:-infra-postgres-1}"; pass=0; fail=0
check(){ if [ "$2" = "$3" ]; then echo "   ✓ $1"; pass=$((pass+1)); else echo "   ✗ $1 (got '$2' want '$3')"; fail=$((fail+1)); fi; }
has(){ if grep -qF -- "$2" <<<"$3"; then echo "   ✓ $1"; pass=$((pass+1)); else echo "   ✗ $1 — '$2' not in: ${3:0:300}"; fail=$((fail+1)); fi; }
sql(){ docker exec -i "$PG" psql -U qaat -d qaat -tAc "$1"; }

cleanup(){ docker exec -i "$PG" psql -U qaat -d qaat -q >/dev/null 2>&1 <<'SQL'
DELETE FROM attendance_logs WHERE session_id IN (SELECT session_id FROM sessions WHERE unit_id LIKE 'OTT-%');
DELETE FROM lecturer_attendance_logs WHERE unit_id LIKE 'OTT-%';
DELETE FROM lecturer_patrol_logs WHERE unit_id LIKE 'OTT-%';
DELETE FROM sessions        WHERE unit_id LIKE 'OTT-%';
DELETE FROM timetable_slots WHERE unit_id LIKE 'OTT-%';
DELETE FROM lecturer_assignments WHERE unit_id LIKE 'OTT-%';
DELETE FROM students_extended WHERE student_id LIKE 'OTT-%';
DELETE FROM course_units    WHERE unit_id LIKE 'OTT-%';
DELETE FROM course_offerings WHERE course_id = 'OTT-C';
DELETE FROM courses         WHERE course_id = 'OTT-C';
DELETE FROM lecturers       WHERE staff_id LIKE 'OTT-%';
DELETE FROM users           WHERE email LIKE 'ott.%';
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
OFF='cc999999-0000-4000-8000-0000000cc999'
DOM=$(sql "SELECT domain FROM tenants WHERE tenant_id='$TEN'")

docker exec -i "$PG" psql -U qaat -d qaat -q -v ON_ERROR_STOP=1 >/dev/null <<SQL
CREATE EXTENSION IF NOT EXISTS pgcrypto;
INSERT INTO courses (course_id, tenant_id, name, department) VALUES ('OTT-C','$TEN','OTTest Course','OTTEST Dept');
-- NO timetable_slots row for this unit anywhere: that is the whole point.
INSERT INTO course_units (unit_id, tenant_id, course_id, name, year, semester)
 VALUES ('OTT-UNIT','$TEN','OTT-C','OTTest Make-up Lecture',2,1);
INSERT INTO course_offerings (offering_id, tenant_id, course_id, session_type, study_year, semester)
 VALUES ('$OFF','$TEN','OTT-C','Day',2,1);
INSERT INTO users (tenant_id, email, password_hash, role, full_name, is_active, staff_id, force_password_change) VALUES
 ('$TEN','ott.coord@$DOM',   crypt('CooPass12345', gen_salt('bf',10)),'COORDINATOR', 'OTT Coordinator', true,'OTT-CRD',false),
 ('$TEN','ott.monitor@$DOM', crypt('MonPass12345', gen_salt('bf',10)),'QA_PATROLLER','OTT Monitor',     true,'OTT-MON',false),
 ('$TEN','ott.lect@$DOM',    crypt('LecPass12345', gen_salt('bf',10)),'LECTURER',    'OTT Lecturer',    true,'OTT-LEC',false);
INSERT INTO lecturers (tenant_id, full_name, email, staff_id, user_id)
 SELECT '$TEN','OTT Lecturer','ott.lect@$DOM','OTT-LEC', user_id FROM users WHERE email='ott.lect@$DOM';
INSERT INTO lecturer_assignments (tenant_id, lecturer_id, unit_id, course_id, academic_year, year, semester, intake_session)
 SELECT '$TEN', lecturer_id, 'OTT-UNIT','OTT-C','2025/2026',2,1,'Morning' FROM lecturers WHERE staff_id='OTT-LEC';
UPDATE course_offerings SET coordinator_id=(SELECT user_id::text FROM users WHERE email='ott.coord@$DOM') WHERE offering_id='$OFF';
INSERT INTO students_extended (student_id, tenant_id, full_name, email, course_id, academic_year, offering_id, current_year, semester)
 VALUES ('OTT-S1','$TEN','OTT Student One','ott.s1@studmc.$DOM','OTT-C','2025/2026','$OFF',2,1),
        ('OTT-S2','$TEN','OTT Student Two','ott.s2@studmc.$DOM','OTT-C','2025/2026','$OFF',2,1);
INSERT INTO users (tenant_id, email, password_hash, role, full_name, is_active, registration_number, force_password_change)
 VALUES ('$TEN','ott.s1@studmc.$DOM', crypt('StuPass12345', gen_salt('bf',10)),'STUDENT','OTT Student One', true,'OTT-S1',false),
        ('$TEN','ott.s2@studmc.$DOM', crypt('StuPass12345', gen_salt('bf',10)),'STUDENT','OTT Student Two', true,'OTT-S2',false);
SQL

login(){ curl -sk -X POST "$BASE/api/v1/auth/app-login" -H 'Content-Type: application/json' -m 30 \
  -d "{\"identifier\":\"$1\",\"password\":\"$2\",\"org\":\"\"}" \
  | python3 -c "import json,sys;print(json.load(sys.stdin).get('access_token',''))"; }
body(){ curl -sk -X "$1" "$BASE$2" -H "Authorization: Bearer $3" \
  ${4:+-H 'Content-Type: application/json' -d "$4"} -m 60; }
jget(){ python3 -c "import json,sys;print(json.load(sys.stdin).get('$1',''))"; }

CRD=$(login OTT-CRD CooPass12345); MON=$(login OTT-MON MonPass12345)
S1=$(login OTT-S1 StuPass12345);   S2=$(login OTT-S2 StuPass12345)
echo "tokens: coord=${#CRD} monitor=${#MON} s1=${#S1} s2=${#S2}"

echo; echo "── 1. the monitor's manual entry does NOT touch the student ledger ──"
curl -sk -X POST "$BASE/api/v1/patrol/bind-device" -H "Authorization: Bearer $MON" \
  -H 'Content-Type: application/json' -d '{"device_fingerprint":"ott-handset"}' -m 30 >/dev/null
R=$(body POST /api/v1/patrol/manual "$MON" \
  '{"unit_id":"OTT-UNIT","lecturer_staff_id":"OTT-LEC","students_counted":40,"taught":true,"time_of_day":"07:30"}')
has   "the monitor records the untimetabled lecture" '"status":"RECORDED"' "$R"
check "…and the student ledger is still empty"  "$(sql "SELECT count(*) FROM attendance_logs al JOIN sessions s ON s.session_id=al.session_id WHERE s.unit_id='OTT-UNIT'")" "0"
# The headcount is the monitor's estimate and lives on THEIR record — never merged into the roll.
check "…the headcount is on the monitor's record only" "$(sql "SELECT students_counted FROM lecturer_patrol_logs WHERE unit_id='OTT-UNIT'")" "40"

echo; echo "── 2. the coordinator can open the off-timetable lecture ──"
check "the unit is genuinely not timetabled" "$(sql "SELECT count(*) FROM timetable_slots WHERE unit_id='OTT-UNIT'")" "0"
R=$(body POST /api/v1/sessions/open "$CRD" \
  '{"unit_id":"OTT-UNIT","unscheduled":true}')
SID=$(jget session_id <<<"$R"); CODE=$(jget student_code <<<"$R")
has   "the session opens"                    '"checkin_code"' "$R"
check "…and is marked off-timetable"         "$(sql "SELECT unscheduled FROM sessions WHERE session_id='$SID'")" "t"

echo; echo "── 3. the students check in through the IDENTICAL flow ──"
# Gate 1: the lecturer must have started. Unchanged — an off-timetable lecture is not a licence
# to check in before anyone is teaching.
R=$(body POST /api/v1/student/checkin "$S1" "{\"session_id\":\"$SID\",\"room_code\":\"$CODE\"}")
has "refused until the lecturer starts"      'LECTURER_NOT_STARTED' "$R"
CC=$(body GET "/api/v1/sessions/$SID/checkin-code" "$CRD" | jget code)
body POST /api/v1/lecturer/gate-scan "" >/dev/null 2>&1 || true
R=$(curl -sk -X POST "$BASE/api/v1/lecturer/gate-scan" -H 'Content-Type: application/json' -m 30 \
  -d "{\"session_id\":\"$SID\",\"staff_id\":\"OTT-LEC\",\"room_code\":\"$CC\",\"fingerprint\":\"ott-fp\"}")
has "the lecturer's gate scan works the same" '"status":"STARTED"' "$R"
# Gate 2: the room code is still the proximity proof.
R=$(body POST /api/v1/student/checkin "$S1" "{\"session_id\":\"$SID\",\"room_code\":\"000000\"}")
has "a wrong room code is still refused"     'PROXIMITY_FAILED' "$R"
# The real check-in.
R=$(body POST /api/v1/student/checkin "$S1" "{\"session_id\":\"$SID\",\"room_code\":\"$CODE\",\"device_fingerprint\":\"phone-1\"}")
has "the student checks in"                  '"status":"PRESENT"' "$R"
R=$(body POST /api/v1/student/checkin "$S2" "{\"session_id\":\"$SID\",\"room_code\":\"$CODE\",\"device_fingerprint\":\"phone-2\"}")
has "and so does the second"                 '"status":"PRESENT"' "$R"
# Gate 3: one device cannot mark two people present.
R=$(body POST /api/v1/student/checkin "$S1" "{\"session_id\":\"$SID\",\"room_code\":\"$CODE\",\"device_fingerprint\":\"phone-2\"}")
has "one device still cannot mark two students" 'DEVICE_ALREADY_USED' "$R"
check "exactly two are on the roll, no duplicates" \
  "$(sql "SELECT count(*)||'/'||count(DISTINCT student_id) FROM attendance_logs al JOIN sessions s ON s.session_id=al.session_id WHERE s.unit_id='OTT-UNIT'")" "2/2"

echo; echo "── 4. the record says what kind of lecture it was ──"
check "students attended is the REGISTER, not the estimate" \
  "$(sql "SELECT count(*) FROM attendance_logs al JOIN sessions s ON s.session_id=al.session_id WHERE s.unit_id='OTT-UNIT'")" "2"
check "…and the monitor's estimate stays separate at 40" \
  "$(sql "SELECT students_counted FROM lecturer_patrol_logs WHERE unit_id='OTT-UNIT'")" "40"

body POST "/api/v1/sessions/$SID/close" "$CRD" '{}' >/dev/null
echo; echo "=== $pass passed, $fail failed ==="
[ "$fail" -eq 0 ]
