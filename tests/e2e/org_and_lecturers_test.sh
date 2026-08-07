#!/usr/bin/env bash
#
# The org chart, the lecturer's college, and removing a lecturer — end to end against a running
# stack.
#
# Three things an administrator does on day one and cannot do without:
#
#   · load the whole org chart from one file, and load a corrected version over it
#   · file every lecturer under a college, by short form or full title
#   · remove a lecturer, one or many, without taking the teaching record with them
#
# The assertions that matter are the negative ones: a re-import must not duplicate, a re-import
# with a blank school column must not UNFILE everyone it touches, an unrecognised college must not
# skip the lecturer, and an empty bulk selection must be refused rather than reported as a
# successful deletion of nothing.
#
# Usage:  ./org_and_lecturers_test.sh        (against https://localhost:8443)
# Seeds and removes its own ORGTEST/ORGT fixtures via the postgres container.
set -uo pipefail
BASE="${1:-https://localhost:8443}"; PG="${PG_CONTAINER:-infra-postgres-1}"; pass=0; fail=0
check(){ if [ "$2" = "$3" ]; then echo "   ✓ $1"; pass=$((pass+1)); else echo "   ✗ $1 (got '$2' want '$3')"; fail=$((fail+1)); fi; }
has(){ if grep -qF -- "$2" <<<"$3"; then echo "   ✓ $1"; pass=$((pass+1)); else echo "   ✗ $1 — '$2' not in: ${3:0:300}"; fail=$((fail+1)); fi; }
hasnt(){ if grep -qF -- "$2" <<<"$3"; then echo "   ✗ $1 — found '$2'"; fail=$((fail+1)); else echo "   ✓ $1"; pass=$((pass+1)); fi; }

cleanup(){ docker exec -i "$PG" psql -U qaat -d qaat -q >/dev/null 2>&1 <<'SQL'
DELETE FROM lecturer_assignments WHERE lecturer_id IN (SELECT lecturer_id FROM lecturers WHERE staff_id LIKE 'ORGT-%');
DELETE FROM lecturers WHERE staff_id LIKE 'ORGT-%';
DELETE FROM users WHERE email LIKE 'orgt.%';
DELETE FROM departments WHERE name LIKE 'ORGTEST%';
DELETE FROM schools WHERE name LIKE 'ORGTEST%' OR abbreviation LIKE 'OGT%';
SQL
}
trap cleanup EXIT; cleanup

TEN=$(docker exec -i "$PG" psql -U qaat -d qaat -tAc "SELECT tenant_id FROM tenants WHERE tenant_id <> '00000000-0000-0000-0000-000000000000' ORDER BY created_at LIMIT 1")
docker exec -i "$PG" psql -U qaat -d qaat -q >/dev/null <<SQL
INSERT INTO users (tenant_id, email, password_hash, role, full_name, is_active, force_password_change)
VALUES ('$TEN','orgt.admin@kiu.ac.ug', crypt('AdmPass12345', gen_salt('bf',10)),'ADMIN','ORGT Admin',true,false);
SQL
TOK=$(curl -sk -X POST "$BASE/api/v1/auth/login" -H 'Content-Type: application/json' -m 30 \
  -d "{\"email\":\"orgt.admin@kiu.ac.ug\",\"password\":\"AdmPass12345\",\"tenant_id\":\"$TEN\"}" \
  | python3 -c "import json,sys;print(json.load(sys.stdin).get('access_token',''))")
echo "admin token ${#TOK}"

echo; echo "── 3. org chart import (schools + departments in one file) ──"
cat > /tmp/org.csv <<'CSV'
school,abbreviation,department,kind
ORGTEST School of Computing,OGTC,ORGTEST Computer Science,ACADEMIC
ORGTEST School of Computing,OGTC,ORGTEST Information Technology,ACADEMIC
ORGTEST College of Business,OGTB,,
,,ORGTEST Finance Office,SUPPORT
CSV
R=$(curl -sk -X POST "$BASE/api/v1/admin/tenants/$TEN/org/import" -H "Authorization: Bearer $TOK" -F "roster=@/tmp/org.csv" -m 60)
echo "   $R"
S=$(docker exec -i "$PG" psql -U qaat -d qaat -tAc "SELECT COUNT(*) FROM schools WHERE name LIKE 'ORGTEST%'")
D=$(docker exec -i "$PG" psql -U qaat -d qaat -tAc "SELECT COUNT(*) FROM departments WHERE name LIKE 'ORGTEST%'")
check "2 colleges created"                 "$S" "2"
check "3 departments created"              "$D" "3"
SUP=$(docker exec -i "$PG" psql -U qaat -d qaat -tAc "SELECT COALESCE(school_id::text,'none')||'/'||kind FROM departments WHERE name='ORGTEST Finance Office'")
check "the support unit has no school"     "$SUP" "none/SUPPORT"
ABB=$(docker exec -i "$PG" psql -U qaat -d qaat -tAc "SELECT abbreviation FROM schools WHERE name='ORGTEST School of Computing'")
check "the short form is stored"           "$ABB" "OGTC"

echo; echo "── re-importing the SAME file must not duplicate ──"
curl -sk -o /dev/null -X POST "$BASE/api/v1/admin/tenants/$TEN/org/import" -H "Authorization: Bearer $TOK" -F "roster=@/tmp/org.csv" -m 60
S2=$(docker exec -i "$PG" psql -U qaat -d qaat -tAc "SELECT COUNT(*) FROM schools WHERE name LIKE 'ORGTEST%'")
D2=$(docker exec -i "$PG" psql -U qaat -d qaat -tAc "SELECT COUNT(*) FROM departments WHERE name LIKE 'ORGTEST%'")
check "still 2 colleges"                   "$S2" "2"
check "still 3 departments"                "$D2" "3"

echo; echo "── 1. lecturer import carries the school, by SHORT FORM ──"
cat > /tmp/lect.csv <<'CSV'
staff_id,full_name,email,phone,title,gender,school
ORGT-001,ORGT Alpha,,0700000001,Dr.,Female,OGTC
ORGT-002,ORGT Beta,,0700000002,Mr.,Male,ORGTEST College of Business
ORGT-003,ORGT Gamma,,0700000003,Ms.,Female,NO SUCH COLLEGE
CSV
R=$(curl -sk -X POST "$BASE/api/v1/admin/tenants/$TEN/lecturers/import" -H "Authorization: Bearer $TOK" -F "roster=@/tmp/lect.csv" -m 60)
echo "   $R"
has "the unknown college is reported"      "NO SUCH COLLEGE" "$R"
A=$(docker exec -i "$PG" psql -U qaat -d qaat -tAc "SELECT s.abbreviation FROM lecturers l JOIN schools s ON s.school_id=l.school_id WHERE l.staff_id='ORGT-001'")
B=$(docker exec -i "$PG" psql -U qaat -d qaat -tAc "SELECT s.abbreviation FROM lecturers l JOIN schools s ON s.school_id=l.school_id WHERE l.staff_id='ORGT-002'")
C=$(docker exec -i "$PG" psql -U qaat -d qaat -tAc "SELECT COALESCE(school_id::text,'none') FROM lecturers WHERE staff_id='ORGT-003'")
check "matched by short form"              "$A" "OGTC"
check "matched by full name"               "$B" "OGTB"
check "unknown college -> imported without one, not skipped" "$C" "none"

echo; echo "── the API returns the short form ──"
L=$(curl -sk "$BASE/api/v1/admin/tenants/$TEN/lecturers" -H "Authorization: Bearer $TOK" -m 30)
has "school_abbrev present"                '"school_abbrev":"OGTC"' "$L"

echo; echo "── a re-import with a BLANK school must not unfile anyone ──"
printf 'staff_id,full_name,phone\nORGT-001,ORGT Alpha,0700000009\n' > /tmp/lect2.csv
curl -sk -o /dev/null -X POST "$BASE/api/v1/admin/tenants/$TEN/lecturers/import" -H "Authorization: Bearer $TOK" -F "roster=@/tmp/lect2.csv" -m 60
A2=$(docker exec -i "$PG" psql -U qaat -d qaat -tAc "SELECT s.abbreviation FROM lecturers l JOIN schools s ON s.school_id=l.school_id WHERE l.staff_id='ORGT-001'")
check "college kept after a blank re-import" "$A2" "OGTC"

echo; echo "── 2. delete one lecturer ──"
ID=$(docker exec -i "$PG" psql -U qaat -d qaat -tAc "SELECT lecturer_id FROM lecturers WHERE staff_id='ORGT-003'")
R=$(curl -sk -X DELETE "$BASE/api/v1/admin/tenants/$TEN/lecturers/$ID" -H "Authorization: Bearer $TOK" -m 30)
echo "   $R"
check "one deleted" "$(python3 -c "import json,sys;print(json.load(sys.stdin)['deleted'])" <<<"$R")" "1"
check "gone from the table" "$(docker exec -i "$PG" psql -U qaat -d qaat -tAc "SELECT COUNT(*) FROM lecturers WHERE staff_id='ORGT-003'")" "0"

echo; echo "── bulk delete the rest ──"
IDS=$(docker exec -i "$PG" psql -U qaat -d qaat -tAc "SELECT string_agg('\"'||lecturer_id::text||'\"', ',') FROM lecturers WHERE staff_id LIKE 'ORGT-%'")
R=$(curl -sk -X POST "$BASE/api/v1/admin/tenants/$TEN/lecturers/bulk-delete" -H "Authorization: Bearer $TOK" \
    -H 'Content-Type: application/json' -m 30 -d "{\"lecturer_ids\":[$IDS]}")
echo "   $R"
check "two deleted" "$(python3 -c "import json,sys;print(json.load(sys.stdin)['deleted'])" <<<"$R")" "2"
check "none left"   "$(docker exec -i "$PG" psql -U qaat -d qaat -tAc "SELECT COUNT(*) FROM lecturers WHERE staff_id LIKE 'ORGT-%'")" "0"

echo; echo "── an empty selection is refused, not a silent success ──"
CODE=$(curl -sk -o /dev/null -w '%{http_code}' -X POST "$BASE/api/v1/admin/tenants/$TEN/lecturers/bulk-delete" \
    -H "Authorization: Bearer $TOK" -H 'Content-Type: application/json' -m 20 -d '{"lecturer_ids":[]}')
check "empty list rejected" "$CODE" "400"

echo; echo "=== $pass passed, $fail failed ==="
[ "$fail" -eq 0 ]
