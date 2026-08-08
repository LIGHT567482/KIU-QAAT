#!/usr/bin/env bash
#
# Three things, end to end against a running stack.
#
#   1. THE TLC OWNS ONE DEPARTMENT'S TIMETABLE, and staffs it. They place the hours AND name the
#      lecturers — the two halves of the same job, which used to sit on two desks, so a timetable
#      got published with blank slots while the TLC waited for someone else to fill them. Bounded
#      to their own department; the administrator keeps the institution.
#
#   2. THE TIMETABLE IS READABLE FROM EVERY OVERSIGHT DASHBOARD. It is the schedule every
#      attendance figure is measured against, and a number whose baseline you cannot see is a
#      number you cannot check.
#
#   3. MANUAL ATTENDANCE for a lecture that was taught but never timetabled. The round is
#      generated FROM the timetable, so a monitor standing in front of an untimetabled lecture
#      could previously record nothing, or tick whichever slot looked closest — filing a true
#      observation under the wrong lecture. Every field picks from a list or accepts typing, and
#      the assertions below cover BOTH paths, because the whole point is the lecture the
#      curriculum has not heard of yet.
#
# Usage:  ./manual_attendance_and_tlc_test.sh        (against https://localhost:8443)
# Seeds and removes its own MANT- fixtures via the postgres container.
set -uo pipefail
BASE="${1:-https://localhost:8443}"; PG="${PG_CONTAINER:-infra-postgres-1}"; pass=0; fail=0
check(){ if [ "$2" = "$3" ]; then echo "   ✓ $1"; pass=$((pass+1)); else echo "   ✗ $1 (got '$2' want '$3')"; fail=$((fail+1)); fi; }
has(){ if grep -qF -- "$2" <<<"$3"; then echo "   ✓ $1"; pass=$((pass+1)); else echo "   ✗ $1 — '$2' not in: ${3:0:400}"; fail=$((fail+1)); fi; }
hasnt(){ if grep -qF -- "$2" <<<"$3"; then echo "   ✗ $1 — found '$2'"; fail=$((fail+1)); else echo "   ✓ $1"; pass=$((pass+1)); fi; }
sql(){ docker exec -i "$PG" psql -U qaat -d qaat -tAc "$1"; }

cleanup(){ docker exec -i "$PG" psql -U qaat -d qaat -q >/dev/null 2>&1 <<'SQL'
DELETE FROM lecturer_patrol_logs  WHERE unit_id LIKE 'MANT-%' OR unit_id LIKE 'Ghost%';
DELETE FROM lecturer_assignments  WHERE unit_id LIKE 'MANT-%';
DELETE FROM timetable_slots       WHERE unit_id LIKE 'MANT-%';
DELETE FROM offering_unit_schedules WHERE unit_id LIKE 'MANT-%';
DELETE FROM course_units          WHERE unit_id LIKE 'MANT-%';
DELETE FROM course_offerings      WHERE course_id IN ('MANT-CS','MANT-NUR');
DELETE FROM courses               WHERE course_id IN ('MANT-CS','MANT-NUR');
DELETE FROM lecturers             WHERE staff_id LIKE 'MANT-%';
DELETE FROM venues                WHERE venue_id LIKE 'MANT-%';
DELETE FROM schools               WHERE name LIKE 'MANTEST%';
DELETE FROM users                 WHERE email LIKE 'mant.%';
SQL
}
trap cleanup EXIT; cleanup

TEN=$(sql "SELECT tenant_id FROM tenants WHERE tenant_id <> '00000000-0000-0000-0000-000000000000' ORDER BY created_at LIMIT 1")
OFFCS='aa333333-0000-4000-8000-0000000aa001'
TODAY=$(date +%F); DOW=$(date +%u)

docker exec -i "$PG" psql -U qaat -d qaat -q -v ON_ERROR_STOP=1 >/dev/null <<SQL
CREATE EXTENSION IF NOT EXISTS pgcrypto;
INSERT INTO schools (tenant_id, name, abbreviation) VALUES ('$TEN','MANTEST College','MANT');
INSERT INTO courses (course_id, tenant_id, name, department, school) VALUES
 ('MANT-CS','$TEN','MANtest Computing','MANTEST Computing','MANTEST College'),
 ('MANT-NUR','$TEN','MANtest Nursing','MANTEST Nursing','MANTEST College');
INSERT INTO course_units (unit_id, tenant_id, course_id, name, year, semester) VALUES
 ('MANT-UNIT','$TEN','MANT-CS','MANtest Known Unit',2,1),
 ('MANT-NURUNIT','$TEN','MANT-NUR','MANtest Nursing Unit',1,2);
INSERT INTO course_offerings (offering_id, tenant_id, course_id, session_type, study_year, semester)
 VALUES ('$OFFCS','$TEN','MANT-CS','Day',2,1);
INSERT INTO venues (venue_id, tenant_id, name, building) VALUES ('MANT-LR9','$TEN','MANtest Lecture Room 9','Block M');
INSERT INTO users (tenant_id, email, password_hash, role, full_name, is_active, staff_id, department, school, force_password_change) VALUES
 ('$TEN','mant.admin@kiu.ac.ug',  crypt('AdmPass12345', gen_salt('bf',10)),'ADMIN',       'MANT Admin',  true,'MANT-ADM',NULL,NULL,false),
 ('$TEN','mant.tlc@kiu.ac.ug',    crypt('TlcPass12345', gen_salt('bf',10)),'TLC',         'MANT TLC',    true,'MANT-TLC','MANTEST Computing','MANTEST College',false),
 ('$TEN','mant.monitor@kiu.ac.ug',crypt('MonPass12345', gen_salt('bf',10)),'QA_PATROLLER','MANT Monitor',true,'MANT-MON',NULL,NULL,false),
 ('$TEN','mant.vc@kiu.ac.ug',     crypt('VcPass12345',  gen_salt('bf',10)),'VC',          'MANT VC',     true,'MANT-VC',NULL,NULL,false),
 ('$TEN','mant.dvc@kiu.ac.ug',    crypt('DvcPass12345', gen_salt('bf',10)),'DVC',         'MANT DVC',    true,'MANT-DVC',NULL,NULL,false),
 ('$TEN','mant.qa@kiu.ac.ug',     crypt('QaPass12345',  gen_salt('bf',10)),'QA_OFFICER',  'MANT QA',     true,'MANT-QA','MANTEST Computing','MANTEST College',false),
 ('$TEN','mant.dqa@kiu.ac.ug',    crypt('DqaPass12345', gen_salt('bf',10)),'DQA_DIRECTOR','MANT DQA',    true,'MANT-DQA',NULL,NULL,false);
INSERT INTO lecturers (tenant_id, full_name, email, staff_id, department) VALUES
 ('$TEN','MANtest Lecturer','mant.l1@kiu.ac.ug','MANT-L1','MANTEST Computing'),
 ('$TEN','MANtest Nurse Lecturer','mant.l2@kiu.ac.ug','MANT-L2','MANTEST Nursing');
SQL

login(){ curl -sk -X POST "$BASE/api/v1/auth/app-login" -H 'Content-Type: application/json' -m 30 \
  -d "{\"identifier\":\"$1\",\"password\":\"$2\",\"org\":\"\"}" \
  | python3 -c "import json,sys;print(json.load(sys.stdin).get('access_token',''))"; }
body(){ curl -sk -X "$1" "$BASE$2" -H "Authorization: Bearer $3" \
  ${4:+-H 'Content-Type: application/json' -d "$4"} -m 60; }
code(){ curl -sk -o /dev/null -w '%{http_code}' -X "$1" "$BASE$2" -H "Authorization: Bearer $3" \
  ${4:+-H 'Content-Type: application/json' -d "$4"} -m 60; }

ADM=$(login MANT-ADM AdmPass12345); TLC=$(login MANT-TLC TlcPass12345)
MON=$(login MANT-MON MonPass12345); VC=$(login MANT-VC VcPass12345)
DVC=$(login MANT-DVC DvcPass12345); QA=$(login MANT-QA QaPass12345); DQA=$(login MANT-DQA DqaPass12345)
echo "tokens: adm=${#ADM} tlc=${#TLC} mon=${#MON} vc=${#VC} dvc=${#DVC}"

echo; echo "── 1. the TLC designs AND staffs their own department ──"
check "the TLC places their department's lecture" \
  "$(code PUT /api/v1/dashboard/timetable "$TLC" "{\"offering_id\":\"$OFFCS\",\"unit_id\":\"MANT-UNIT\",\"day_of_week\":$DOW,\"session_start\":\"09:00\",\"session_duration_minutes\":60}")" "200"
R=$(body PUT /api/v1/dashboard/timetable "$TLC" "{\"offering_id\":\"$OFFCS\",\"unit_id\":\"MANT-NURUNIT\",\"day_of_week\":$DOW,\"session_start\":\"09:00\",\"session_duration_minutes\":60}")
has "…and is refused another department's" 'OUT_OF_DEPARTMENT' "$R"

# Staffing it: the same job, previously on someone else's desk.
LID=$(sql "SELECT lecturer_id FROM lecturers WHERE staff_id='MANT-L1'")
LNUR=$(sql "SELECT lecturer_id FROM lecturers WHERE staff_id='MANT-L2'")
has "the TLC can see who is assignable in their department" 'MANtest' "$(body GET /api/v1/hod/assignable "$TLC")"
R=$(body POST /api/v1/hod/assignments "$TLC" "{\"lecturer_id\":\"$LID\",\"unit_id\":\"MANT-UNIT\",\"academic_year\":\"2025/2026\",\"year\":2,\"semester\":1,\"intake_session\":\"Morning\"}")
echo "   $R"
check "the TLC ADDS a lecturer to their department's unit" \
  "$(sql "SELECT count(*) FROM lecturer_assignments WHERE unit_id='MANT-UNIT'")" "1"
R=$(body POST /api/v1/hod/assignments "$TLC" "{\"lecturer_id\":\"$LNUR\",\"unit_id\":\"MANT-NURUNIT\",\"academic_year\":\"2025/2026\",\"year\":1,\"semester\":2,\"intake_session\":\"Morning\"}")
echo "   out-of-department attempt -> $R"
has "…but NOT to another department's unit" 'OUT_OF_SCOPE' "$R"
check "…and nothing was written there" \
  "$(sql "SELECT count(*) FROM lecturer_assignments WHERE unit_id='MANT-NURUNIT'")" "0"
AID=$(sql "SELECT assignment_id FROM lecturer_assignments WHERE unit_id='MANT-UNIT'")
check "the TLC REMOVES a lecturer they added" \
  "$(code DELETE "/api/v1/hod/assignments/$AID" "$TLC")" "200"
check "…and it is gone" "$(sql "SELECT count(*) FROM lecturer_assignments WHERE unit_id='MANT-UNIT'")" "0"
# The administrator is not confined to a department, so a department with no TLC stays staffable.
# The admin route is the unscoped one and takes the course explicitly.
R=$(body POST "/api/v1/admin/tenants/$TEN/lecturer-assignments" "$ADM" \
   "{\"lecturer_id\":\"$LNUR\",\"unit_id\":\"MANT-NURUNIT\",\"course_id\":\"MANT-NUR\",\"academic_year\":\"2025/2026\",\"year\":1,\"semester\":2,\"intake_session\":\"Morning\"}")
echo "   admin cross-department -> $R"
check "the admin can still staff ANY department" \
  "$(sql "SELECT count(*) FROM lecturer_assignments WHERE unit_id='MANT-NURUNIT'")" "1"

echo; echo "── 2. the timetable is readable from every oversight dashboard ──"
for pair in "TLC:$TLC" "ADMIN:$ADM" "VC:$VC" "DVC:$DVC" "QA:$QA" "DQA:$DQA"; do
  n="${pair%%:*}"; t="${pair#*:}"
  check "$n reads the timetable"       "$(code GET /api/v1/dashboard/timetable "$t")" "200"
  check "$n reads the weekly grid"     "$(code GET /api/v1/dashboard/timetable/slots "$t")" "200"
done
# Reading is institution-wide even for a departmental TLC: rooms are shared, and you cannot avoid
# a clash you cannot see. What they may EDIT is the department named back to them.
G=$(body GET /api/v1/dashboard/timetable/slots "$TLC" | tr -d '\n')
has "the TLC still SEES the whole institution's grid" 'MANT-UNIT' "$G"
has "…and is told which department is theirs to change" '"tlc_department":"MANTEST Computing"' "$G"

echo; echo "── 3. manual attendance — the lecture nobody timetabled ──"
curl -sk -X POST "$BASE/api/v1/patrol/bind-device" -H "Authorization: Bearer $MON" \
  -H 'Content-Type: application/json' -d '{"device_fingerprint":"mant-handset"}' -m 30 >/dev/null
REF=$(body GET /api/v1/patrol/reference "$MON")
has "the monitor gets the ROOM list"     'MANT-LR9'            "$REF"
has "…the COURSE UNIT list"              'MANT-UNIT'           "$REF"
has "…the LECTURER list"                 'MANtest Lecturer'    "$REF"
has "…the SCHOOL list"                   'MANTEST College'     "$REF"
has "…and each unit's class/group"       '"class_group":"2:1"' "$REF"

# (a) Everything PICKED from the lists. The college and class/group are inherited from the unit,
#     so the monitor is not asked to retype what the curriculum already knows.
R=$(body POST /api/v1/patrol/manual "$MON" \
  '{"room_id":"MANT-LR9","unit_id":"MANT-UNIT","lecturer_staff_id":"MANT-L1",
    "students_counted":37,"taught":true,"time_of_day":"08:30","remarks":"MANT picked"}')
echo "   $R"
has "a fully PICKED entry is recorded"        '"status":"RECORDED"'  "$R"
has "…the room resolves to its name"          'MANtest Lecture Room 9' "$R"
has "…the college is INHERITED from the unit" '"school":"MANTEST College"' "$R"
has "…and so is the class/group"              '"class_group":"2:1"'  "$R"
check "…with the headcount stored"            "$(sql "SELECT students_counted FROM lecturer_patrol_logs WHERE unit_id='MANT-UNIT' AND scheduled_time='08:30'")" "37"
check "…marked as a manual entry"             "$(sql "SELECT entry_method FROM lecturer_patrol_logs WHERE unit_id='MANT-UNIT' AND scheduled_time='08:30'")" "MANUAL"

# (b) Everything TYPED — the unit is not in the curriculum and the lecturer is not on the list.
#     This is the case the feature exists for, so it must not be refused.
R=$(body POST /api/v1/patrol/manual "$MON" \
  '{"room":"Old Hall","unit_name":"Ghost Unit 101","lecturer_name":"Visiting Lecturer",
    "class_group":"3:2","school":"MANTEST College","students_counted":12,"taught":true,
    "time_of_day":"14:15","remarks":"MANT typed"}')
echo "   $R"
has "a fully TYPED entry is recorded"    '"status":"RECORDED"' "$R"
has "…keeping the typed unit"            'Ghost Unit 101'      "$R"
has "…and the typed lecturer"            'Visiting Lecturer'   "$R"
check "…with the typed class/group"      "$(sql "SELECT class_group FROM lecturer_patrol_logs WHERE unit_name='Ghost Unit 101'")" "3:2"
check "…and the typed room"              "$(sql "SELECT room FROM lecturer_patrol_logs WHERE unit_name='Ghost Unit 101'")" "Old Hall"

# The two facts without which the record means nothing.
has "an entry with no unit at all is refused"     'course unit' \
  "$(body POST /api/v1/patrol/manual "$MON" '{"room":"X","lecturer_name":"Y","students_counted":3}')"
has "an entry with no lecturer at all is refused" 'lecturer' \
  "$(body POST /api/v1/patrol/manual "$MON" '{"room":"X","unit_name":"Z","students_counted":3}')"
check "a negative headcount is refused" \
  "$(code POST /api/v1/patrol/manual "$MON" '{"unit_name":"Z","lecturer_name":"Y","students_counted":-4}')" "400"
# Two manual entries for the same unit on the same day at DIFFERENT times are two lectures, not a
# duplicate — the observed time is what keeps them apart on ux_patrol_logs_slot.
body POST /api/v1/patrol/manual "$MON" \
  '{"unit_id":"MANT-UNIT","lecturer_staff_id":"MANT-L1","students_counted":8,"taught":true,"time_of_day":"16:45"}' >/dev/null
check "two lectures an hour apart stay two records" \
  "$(sql "SELECT count(*) FROM lecturer_patrol_logs WHERE unit_id='MANT-UNIT' AND session_date='$TODAY'")" "2"

echo; echo "── 3b. manual entries reach QA, named and distinguishable ──"
V=$(body GET /api/v1/dashboard/lecturer-attendance/patrol "$QA")
has "QA sees the manual record"          'Ghost Unit 101'      "$V"
has "…tagged as a manual entry"          '"entry_method":"MANUAL"' "$V"
has "…with the monitor named"            '"qa_monitor":"MANT Monitor"' "$V"
has "…and the monitor's headcount"       '"students_attended":12' "$V"
CSV=$(curl -sk "$BASE/api/v1/dashboard/lecturer-attendance/patrol/export.csv" -H "Authorization: Bearer $QA" -m 60)
has "the download has a Source column"   'Source'              "$CSV"
has "…saying 'Manual entry'"             'Manual entry'        "$CSV"
has "…and a School column"               'School'              "$CSV"
has "…carrying the typed lecture"        'Ghost Unit 101'      "$CSV"
for f in xlsx pdf; do
  check "the .$f download still works" \
    "$(curl -sk -o /dev/null -w '%{http_code}' "$BASE/api/v1/dashboard/lecturer-attendance/patrol/export.$f" -H "Authorization: Bearer $QA" -m 60)" "200"
done
# Only a monitor may file one — it is an observation about a named lecturer.
for pair in "ADMIN:$ADM" "TLC:$TLC" "VC:$VC"; do
  n="${pair%%:*}"; t="${pair#*:}"
  check "$n cannot file a monitor's observation" \
    "$(code POST /api/v1/patrol/manual "$t" '{"unit_name":"X","lecturer_name":"Y"}')" "403"
done

echo; echo "=== $pass passed, $fail failed ==="
[ "$fail" -eq 0 ]
