#!/usr/bin/env bash
#
# The lecturer's own record of being in the room, end to end, against a running stack.
#
# WHAT THIS EXISTS FOR. A QA patrol tick is one person's account of a moment — timestamped, filed
# against a named lecturer — and it used to be the only account there was. A lecturer teaching in a
# room the patroller never reached could answer only with their word, days later. So they get a
# record made at the time too: one button in the app captures where the phone is, when, and which
# timetabled slot that lands in, files it offline, and syncs later.
#
# The assertions are the JOURNEY, not the endpoints:
#
#   the phone can cache the week  ->  a claim filed offline uploads  ->  a retry does not duplicate
#   it  ->  QA sees it WITH the patrol tick beside it  ->  the lecturer cannot read the review page
#   ->  nobody can edit or delete a filed record
#
# The last two are the ones that make the record worth anything. A claim a lecturer could quietly
# revise, or an office could quietly delete, is not evidence.
#
# Usage:  ./presence_claims_test.sh [BASE_URL]        (default https://localhost:8443)
#
# Seeds and removes its own PCTEST fixtures via the postgres container. Needs APP_DB_PASSWORD in
# the environment for the append-only checks (they run as the data-plane role, on purpose):
#   PGPASSWORD=$(grep ^APP_DB_PASSWORD= .env | cut -d= -f2-) ./tests/e2e/presence_claims_test.sh
set -uo pipefail
BASE="${1:-https://localhost:8443}"
PG="${PG_CONTAINER:-infra-postgres-1}"
pass=0; fail=0
check(){ if [ "$2" = "$3" ]; then echo "   ✓ $1"; pass=$((pass+1)); else echo "   ✗ $1 (got '$2' want '$3')"; fail=$((fail+1)); fi; }
has(){   if grep -qF -- "$2" <<<"$3"; then echo "   ✓ $1"; pass=$((pass+1)); else echo "   ✗ $1 — '$2' not in: ${3:0:400}"; fail=$((fail+1)); fi; }

cleanup(){ docker exec -i "$PG" psql -U qaat -d qaat -q >/dev/null 2>&1 <<'SQL'
DELETE FROM lecturer_presence_claims WHERE lecturer_staff_id LIKE 'PCTEST%';
DELETE FROM lecturer_patrol_logs WHERE unit_id LIKE 'PCTEST%';
DELETE FROM timetable_slots WHERE unit_id LIKE 'PCTEST%';
DELETE FROM lecturer_assignments WHERE unit_id LIKE 'PCTEST%';
DELETE FROM course_units WHERE unit_id LIKE 'PCTEST%';
DELETE FROM lecturers WHERE staff_id LIKE 'PCTEST%';
DELETE FROM users WHERE email LIKE 'pctest.%';
SQL
}
trap cleanup EXIT
cleanup

# The INSTITUTION's date, not this machine's: the service stamps records in Africa/Kampala
# (internal/clock), so a host west of it spends several hours each night on yesterday.
TODAY=$(docker exec -i "$PG" psql -U qaat -d qaat -tAc "SELECT (now() AT TIME ZONE 'Africa/Kampala')::date")
DOW=$(docker exec -i "$PG" psql -U qaat -d qaat -tAc "SELECT EXTRACT(ISODOW FROM (now() AT TIME ZONE 'Africa/Kampala'))::int")
docker exec -i "$PG" psql -U qaat -d qaat -q -v ON_ERROR_STOP=1 >/dev/null <<SQL
CREATE EXTENSION IF NOT EXISTS pgcrypto;
DO \$\$
DECLARE v_tenant uuid; v_dom text; v_lec uuid; v_lecrow uuid; v_course varchar; v_qa uuid; v_off uuid;
BEGIN
  SELECT tenant_id, domain INTO v_tenant, v_dom FROM tenants
   WHERE tenant_id <> '00000000-0000-0000-0000-000000000000' ORDER BY created_at LIMIT 1;
  SELECT offering_id, course_id INTO v_off, v_course FROM course_offerings WHERE tenant_id = v_tenant LIMIT 1;

  INSERT INTO users (tenant_id, email, password_hash, role, full_name, is_active, staff_id, force_password_change)
  VALUES (v_tenant,'pctest.lec@'||v_dom, crypt('LecPass12345', gen_salt('bf',10)),'LECTURER','PCTest Lecturer',true,'PCTEST-LEC',false)
  RETURNING user_id INTO v_lec;
  INSERT INTO users (tenant_id, email, password_hash, role, full_name, is_active, staff_id, force_password_change)
  VALUES (v_tenant,'pctest.qa@'||v_dom, crypt('QaPass12345', gen_salt('bf',10)),'QA_OFFICER','PCTest QA Officer',true,'PCTEST-QA',false)
  RETURNING user_id INTO v_qa;

  INSERT INTO lecturers (tenant_id, staff_id, full_name, email, user_id)
  VALUES (v_tenant,'PCTEST-LEC','PCTest Lecturer','pctest.lec@'||v_dom, v_lec)
  RETURNING lecturer_id INTO v_lecrow;

  INSERT INTO course_units (tenant_id, unit_id, course_id, name, year, semester)
  VALUES (v_tenant, 'PCTEST-UNIT-1', v_course, 'Distributed Systems', 2, 1)
  ON CONFLICT DO NOTHING;

  -- Two slots today: one names the lecturer on the slot, one relies on the assignment fallback.
  INSERT INTO timetable_slots (tenant_id, offering_id, unit_id, day_of_week, start_time, duration_minutes, room, lecturer_id)
  VALUES (v_tenant, v_off, 'PCTEST-UNIT-1', ${DOW}, '14:00', 60, 'LR3', v_lecrow);
  INSERT INTO course_units (tenant_id, unit_id, course_id, name, year, semester)
  VALUES (v_tenant, 'PCTEST-UNIT-2', v_course, 'Compiler Design', 3, 1) ON CONFLICT DO NOTHING;
  INSERT INTO timetable_slots (tenant_id, offering_id, unit_id, day_of_week, start_time, duration_minutes, room, lecturer_id)
  VALUES (v_tenant, v_off, 'PCTEST-UNIT-2', ${DOW}, '16:00', 60, 'LR7', NULL);
  INSERT INTO lecturer_assignments (tenant_id, lecturer_id, unit_id, course_id, academic_year)
  VALUES (v_tenant, v_lecrow, 'PCTEST-UNIT-2', v_course, '2025/2026') ON CONFLICT DO NOTHING;

  -- The patrol tick the lecturer is disputing: NOT TAUGHT for the 14:00.
  INSERT INTO lecturer_patrol_logs
    (tenant_id, unit_id, unit_name, course_code, lecturer_id, lecturer_name, room,
     session_date, scheduled_time, taught, patroller_id, patroller_name, taken_at)
  VALUES (v_tenant, 'PCTEST-UNIT-1','Distributed Systems','CS3201','PCTEST-LEC','PCTest Lecturer','LR3',
          '${TODAY}','14:00', false, v_qa, 'Some Patroller', now());
END \$\$;
SQL

login(){ curl -sk -X POST "$BASE/api/v1/auth/app-login" -H 'Content-Type: application/json' \
  -d "{\"identifier\":\"$1\",\"password\":\"$2\"}" -m 30 \
  | python3 -c "import json,sys;print(json.load(sys.stdin).get('access_token',''))"; }

LTOK=$(login PCTEST-LEC LecPass12345)
QTOK=$(curl -sk -X POST "$BASE/api/v1/auth/login" -H 'Content-Type: application/json' -m 30 \
  -d "$(docker exec -i $PG psql -U qaat -d qaat -tAc "SELECT json_build_object('email', email, 'password','QaPass12345','tenant_id',tenant_id)::text FROM users WHERE email LIKE 'pctest.qa@%'")" \
  | python3 -c "import json,sys;print(json.load(sys.stdin).get('access_token',''))")
echo "lecturer token ${#LTOK}, qa token ${#QTOK}"

echo "=== The lecturer presence record against $BASE ==="
echo "── 1. the phone can cache the lecturer's week ──"
TT=$(curl -sk "$BASE/api/v1/lecturer/timetable" -H "Authorization: Bearer $LTOK" -m 30)
echo "$TT" | python3 -m json.tool | head -25
has "the slot that NAMES the lecturer is returned"        "PCTEST-UNIT-1" "$TT"
has "the slot reached only via an ASSIGNMENT is returned" "PCTEST-UNIT-2" "$TT"
has "with the room, so the phone can show it offline"     "LR3"           "$TT"

echo
echo "── 2. a claim filed offline uploads ──"
CID=$(python3 -c "import uuid;print(uuid.uuid4())")
UP=$(curl -sk -X POST "$BASE/api/v1/lecturer/presence-claims" -H "Authorization: Bearer $LTOK" \
  -H 'Content-Type: application/json' -m 30 -d "{\"claims\":[{
    \"claim_id\":\"$CID\",\"latitude\":0.3136,\"longitude\":32.5811,\"accuracy_metres\":12.5,
    \"location_status\":\"OK\",\"captured_at\":\"${TODAY}T14:07:00Z\",\"session_date\":\"$TODAY\",
    \"unit_id\":\"PCTEST-UNIT-1\",\"unit_name\":\"Distributed Systems\",\"room\":\"LR3\",
    \"day_of_week\":${DOW},\"scheduled_time\":\"14:00\",\"match_kind\":\"IN_SLOT\",
    \"minutes_from_start\":7,\"note\":\"Taught the whole hour, nobody came round\"}]}")
echo "upload -> $UP"
check "the claim is filed" "$(python3 -c "import json,sys;print(json.load(sys.stdin)['records_written'])" <<<"$UP")" "1"

echo
echo "── 3. re-sending the SAME claim does not duplicate it ──"
curl -sk -o /dev/null -X POST "$BASE/api/v1/lecturer/presence-claims" -H "Authorization: Bearer $LTOK" \
  -H 'Content-Type: application/json' -m 30 -d "{\"claims\":[{\"claim_id\":\"$CID\",\"captured_at\":\"${TODAY}T14:07:00Z\",\"session_date\":\"$TODAY\",\"match_kind\":\"IN_SLOT\"}]}"
N=$(docker exec -i "$PG" psql -U qaat -d qaat -tAc "SELECT COUNT(*) FROM lecturer_presence_claims WHERE lecturer_staff_id='PCTEST-LEC';")
check "still exactly one row after the retry" "$N" "1"

echo
echo "── 4. QA sees it, with the patrol tick beside it ──"
QA=$(curl -sk "$BASE/api/v1/dashboard/qa/presence-claims?days=2" -H "Authorization: Bearer $QTOK" -m 30)
echo "$QA" | python3 -c "
import json,sys
for c in json.load(sys.stdin):
    if c['lecturer_staff_id']!='PCTEST-LEC': continue
    print('lecturer :', c['lecturer_name'], '/', c['lecturer_staff_id'])
    print('claim    :', c['captured_at'], c['match_kind'], c['minutes_from_start'], 'min in')
    print('where    :', c['latitude'], c['longitude'], '±', c['accuracy_metres'], 'm', '(', c['location_status'], ')')
    print('lecture  :', c['unit_name'], c['scheduled_time'], c['room'], c['session_date'])
    print('patrol   : taught =', c['patrol_taught'], 'room', c['patrol_room'])
    print('note     :', c['note'])
"
has "the QA view carries the claim"          "PCTEST-LEC"   "$QA"
has "and the patrol verdict beside it"       "\"patrol_taught\":false" "$QA"
has "and the coordinates"                    "0.3136"       "$QA"

echo
echo "── 5. a lecturer cannot read the QA view ──"
CODE=$(curl -sk -o /dev/null -w '%{http_code}' "$BASE/api/v1/dashboard/qa/presence-claims" -H "Authorization: Bearer $LTOK" -m 20)
check "lecturer is refused the review page" "$CODE" "403"

echo
echo "── 6. the record cannot be edited or deleted by the data-plane role ──"
UPD=$(docker exec -i "$PG" psql -U qaat_app -d qaat -tAc "UPDATE lecturer_presence_claims SET note='tampered';" 2>&1 | head -1)
has "UPDATE is refused" "permission denied" "$UPD"
DEL=$(docker exec -i "$PG" psql -U qaat_app -d qaat -tAc "DELETE FROM lecturer_presence_claims;" 2>&1 | head -1)
has "DELETE is refused" "permission denied" "$DEL"

echo; echo "=== $pass passed, $fail failed ==="
[ "$fail" -eq 0 ]
