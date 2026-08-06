#!/usr/bin/env bash
#
# The first-login journey, end to end, against a running stack.
#
# This is the path every student, lecturer and patroller walks exactly once, and it used to be a
# dead end. Login tolerated a case-variant of the seeded default password; the mandatory change
# screen it then forced you onto did not, so the very word that had just signed you in was
# rejected as "current password is incorrect" — on the one screen in the app with no navigation
# of its own. You could get in and you could not get past it.
#
# So the assertions here are the journey, not the endpoints:
#
#   sign in with the role's default  →  told to set a password  →  set it  →  reach the app
#   →  sign in again with the NEW one  →  not asked again  →  the default is dead
#
# Usage:  ./first_login_test.sh [BASE_URL]           (default https://localhost:8443)
#
# Needs the three PWTEST accounts; it seeds and removes them itself via the postgres container.
set -uo pipefail

BASE="${1:-https://localhost:8443}"
PG="${PG_CONTAINER:-infra-postgres-1}"
ORG="${ORG:-kiu}"
pass=0; fail=0

check() { if [ "$2" = "$3" ]; then echo "      ✓ $1"; pass=$((pass+1)); else echo "      ✗ $1 (got '$2', want '$3')"; fail=$((fail+1)); fi; }

login() {
  curl -sk -X POST "$BASE/api/v1/auth/app-login" -H 'Content-Type: application/json' \
    -d "{\"identifier\":\"$1\",\"password\":\"$2\",\"org\":\"$ORG\"}" -m 30
}
change() {
  curl -sk -o /dev/null -w '%{http_code}' -X POST "$BASE/api/v1/auth/change-password" \
    -H "Authorization: Bearer $1" -H 'Content-Type: application/json' \
    -d "{\"current_password\":\"$2\",\"new_password\":\"$3\"}" -m 30
}
# Can this token actually open the app? The inbox is the right probe: every role has one, the
# phone loads it on the way into the dashboard, and it is one of the screens that used to answer
# "Not signed in".
opens_app() {
  curl -sk -o /dev/null -w '%{http_code}' "$BASE/api/v1/app-notifications" \
    -H "Authorization: Bearer $1" -m 20
}
jget() { python3 -c "import json,sys
try: d=json.load(sys.stdin)
except Exception: print(''); raise SystemExit
v=d.get('$1'); print('' if v is None else (str(v).lower() if isinstance(v,bool) else v))"; }

seed() {
  docker exec -i "$PG" psql -U qaat -d qaat -q >/dev/null <<'SQL'
CREATE EXTENSION IF NOT EXISTS pgcrypto;
DO $$
DECLARE v_tenant uuid; v_dom text; v_off uuid;
BEGIN
  SELECT tenant_id, domain INTO v_tenant, v_dom FROM tenants
   WHERE tenant_id <> '00000000-0000-0000-0000-000000000000' ORDER BY created_at LIMIT 1;
  SELECT offering_id INTO v_off FROM course_offerings WHERE tenant_id = v_tenant LIMIT 1;

  INSERT INTO students_extended (student_id, tenant_id, full_name, email, course_id, current_year,
                                 semester, academic_year, enrollment_status, offering_id)
  SELECT 'PWTEST-STU-1', v_tenant, 'PWTest Student', 'pwtest.stu1@' || v_dom,
         (SELECT course_id FROM course_offerings WHERE offering_id = v_off),
         1, 1, '2025/2026', 'ACTIVE', v_off
  ON CONFLICT (student_id) DO NOTHING;

  -- The STUDENT login is deliberately seeded with the OLD capitalised spelling while the test
  -- types the lowercase one. That mismatch IS the regression.
  INSERT INTO users (tenant_id, email, password_hash, role, full_name, is_active, staff_id, force_password_change)
  VALUES
    (v_tenant, 'pwtest.stu1@' || v_dom, crypt('Student',   gen_salt('bf', 10)), 'STUDENT',      'PWTest Student',   true, NULL,           true),
    (v_tenant, 'pwtest.lec1@' || v_dom, crypt('lecturer',  gen_salt('bf', 10)), 'LECTURER',     'PWTest Lecturer',  true, 'PWTEST-LEC-1', true),
    (v_tenant, 'pwtest.pat1@' || v_dom, crypt('patroller', gen_salt('bf', 10)), 'QA_PATROLLER', 'PWTest Patroller', true, 'PWTEST-PAT-1', true)
  ON CONFLICT (tenant_id, email) DO NOTHING;
END $$;
SQL
}

cleanup() {
  docker exec -i "$PG" psql -U qaat -d qaat -q >/dev/null <<'SQL'
DELETE FROM users              WHERE email LIKE 'pwtest.%';
DELETE FROM students_extended  WHERE student_id LIKE 'PWTEST-%';
SQL
}
trap cleanup EXIT

run() { # label identifier typed_default new_password
  echo "  --- $1 ---"
  local B TOK B2
  B=$(login "$2" "$3"); TOK=$(echo "$B" | jget access_token)
  if [ -z "$TOK" ]; then
    echo "      ✗ cannot sign in with the default: $(echo "$B" | head -c 200)"; fail=$((fail+1)); return
  fi
  check "signs in with the default"                    "yes" "yes"
  check "is told to set a password"                    "$(echo "$B" | jget force_password_change)" "true"
  check "the change screen ACCEPTS that same password" "$(change "$TOK" "$3" "$4")" "200"
  # "Save & proceed" has to PROCEED. The app keeps the token it already holds and immediately
  # signs in again with the password just chosen — so both have to work, or the person lands on a
  # dashboard where every panel says "Not signed in".
  check "the token it already holds still opens the app" "$(opens_app "$TOK")" "200"
  B2=$(login "$2" "$4")
  local TOK2; TOK2=$(echo "$B2" | jget access_token)
  check "signs in with the NEW password"               "$( [ -n "$TOK2" ] && echo yes )" "yes"
  check "and THAT session opens the app too"           "$(opens_app "$TOK2")" "200"
  check "is NOT asked to change again"                 "$(echo "$B2" | jget force_password_change)" "false"
  check "the old default no longer works"              "$( [ -z "$(login "$2" "$3" | jget access_token)" ] && echo dead )" "dead"
}

echo "=== First-login journey against $BASE ==="
seed
run "STUDENT   — account seeded 'Student', types 'student'" "PWTEST-STU-1" "student"   "NewStudentPass1"
run "LECTURER  — seeded 'lecturer'"                         "PWTEST-LEC-1" "lecturer"  "NewLecturerPass1"
run "PATROLLER — seeded 'patroller'"                        "PWTEST-PAT-1" "patroller" "NewPatrolPass1"
echo
echo "=== $pass passed, $fail failed ==="
[ "$fail" -eq 0 ]
