#!/usr/bin/env bash
#
# Quality Assurance writing to its own field staff, end to end against a running stack.
#
# The QA officer and the DQA director run the patrol round and could not send a patroller a single
# line. Every other role with people under it had a channel; the two roles responsible for the
# round had none, so reassigning a round or calling a handset in was a phone call, off the record.
#
# It goes through /api/v1/app-notifications — the inbox the patroller's phone app already polls and
# badges — so a briefing lands where they are actually looking.
#
# The assertions are the ones that matter if this is to be trusted: a broadcast reaches every
# ACTIVE patroller and no inactive one; a message to one person reaches exactly that person and
# nobody else; the sender is named; and a patroller cannot send in the other direction.
#
# Usage:  ./qa_patroller_messaging_test.sh        (against https://localhost:8443)
# Seeds and removes its own QPTEST fixtures via the postgres container.
set -uo pipefail
BASE="${1:-https://localhost:8443}"; PG="${PG_CONTAINER:-infra-postgres-1}"; pass=0; fail=0
check(){ if [ "$2" = "$3" ]; then echo "   ✓ $1"; pass=$((pass+1)); else echo "   ✗ $1 (got '$2' want '$3')"; fail=$((fail+1)); fi; }
has(){ if grep -qF -- "$2" <<<"$3"; then echo "   ✓ $1"; pass=$((pass+1)); else echo "   ✗ $1 — '$2' not in: ${3:0:300}"; fail=$((fail+1)); fi; }

cleanup(){ docker exec -i "$PG" psql -U qaat -d qaat -q >/dev/null 2>&1 <<'SQL'
DELETE FROM notification_recipients WHERE notification_id IN (SELECT notification_id FROM app_notifications WHERE subject LIKE 'QPTEST%');
DELETE FROM app_notifications WHERE subject LIKE 'QPTEST%';
DELETE FROM users WHERE email LIKE 'qptest.%';
SQL
}
trap cleanup EXIT; cleanup

docker exec -i "$PG" psql -U qaat -d qaat -q -v ON_ERROR_STOP=1 >/dev/null <<'SQL'
CREATE EXTENSION IF NOT EXISTS pgcrypto;
DO $$
DECLARE v_tenant uuid; v_dom text;
BEGIN
  SELECT tenant_id, domain INTO v_tenant, v_dom FROM tenants
   WHERE tenant_id <> '00000000-0000-0000-0000-000000000000' ORDER BY created_at LIMIT 1;
  INSERT INTO users (tenant_id, email, password_hash, role, full_name, is_active, staff_id, force_password_change) VALUES
   (v_tenant,'qptest.qa@'||v_dom,  crypt('QaPass12345',  gen_salt('bf',10)),'QA_OFFICER',  'QPTest QA Officer', true,'QPTEST-QA',  false),
   (v_tenant,'qptest.dqa@'||v_dom, crypt('DqaPass12345', gen_salt('bf',10)),'DQA_DIRECTOR','QPTest DQA',        true,'QPTEST-DQA', false),
   (v_tenant,'qptest.p1@'||v_dom,  crypt('PatPass12345', gen_salt('bf',10)),'QA_PATROLLER','QPTest Patroller 1',true,'QPTEST-P1',  false),
   (v_tenant,'qptest.p2@'||v_dom,  crypt('PatPass12345', gen_salt('bf',10)),'QA_PATROLLER','QPTest Patroller 2',true,'QPTEST-P2',  false),
   (v_tenant,'qptest.p3@'||v_dom,  crypt('PatPass12345', gen_salt('bf',10)),'QA_PATROLLER','QPTest Retired',    false,'QPTEST-P3', false);
END $$;
SQL

login(){ curl -sk -X POST "$BASE/api/v1/auth/app-login" -H 'Content-Type: application/json' \
  -d "{\"identifier\":\"$1\",\"password\":\"$2\"}" -m 30 | python3 -c "import json,sys;print(json.load(sys.stdin).get('access_token',''))"; }
QA=$(login QPTEST-QA QaPass12345); DQA=$(login QPTEST-DQA DqaPass12345)
P1=$(login QPTEST-P1 PatPass12345); P2=$(login QPTEST-P2 PatPass12345)
echo "tokens: qa=${#QA} dqa=${#DQA} p1=${#P1} p2=${#P2}"

send(){ curl -sk -X POST "$BASE/api/v1/app-notifications" -H "Authorization: Bearer $1" \
  -H 'Content-Type: application/json' -m 30 \
  -d "{\"audience\":\"$2\",\"target_id\":\"$3\",\"subject\":\"$4\",\"body\":\"$5\"}"; }
inbox(){ curl -sk "$BASE/api/v1/app-notifications" -H "Authorization: Bearer $1" -m 30; }

echo; echo "── 1. QA officer briefs every patroller ──"
# Counted from the database, not hardcoded: this institution already has patrollers of its own,
# and a fixed number would have failed for a reason that had nothing to do with the code.
# Scoped to the SENDER'S tenant, like the broadcast is: any other institution in the same
# database has patrollers of its own, and counting those made this fail for a reason that had
# nothing to do with the code.
ACTIVE=$(docker exec -i "$PG" psql -U qaat -d qaat -tAc \
  "SELECT COUNT(*) FROM users u WHERE u.role='QA_PATROLLER' AND COALESCE(u.is_active,true)
     AND u.tenant_id = (SELECT tenant_id FROM users WHERE staff_id='QPTEST-QA');")
R=$(send "$QA" PATROLLERS "" "QPTEST round change" "Start at the Science block today.")
echo "   $R  (active patrollers: $ACTIVE)"
check "reached every ACTIVE patroller and no inactive one" "$(python3 -c "import json,sys;print(json.load(sys.stdin)['recipients'])" <<<"$R")" "$ACTIVE"
has "patroller 1 has it" "QPTEST round change" "$(inbox "$P1")"
has "patroller 2 has it" "QPTEST round change" "$(inbox "$P2")"

echo; echo "── 2. DQA messages ONE patroller ──"
R=$(send "$DQA" PATROLLER QPTEST-P2 "QPTEST bring the handset in" "Swap it at the QA office.")
check "one recipient" "$(python3 -c "import json,sys;print(json.load(sys.stdin)['recipients'])" <<<"$R")" "1"
has "the addressed patroller has it" "QPTEST bring the handset in" "$(inbox "$P2")"
if grep -qF "QPTEST bring the handset in" <<<"$(inbox "$P1")"; then
  echo "   ✗ it also reached the OTHER patroller"; fail=$((fail+1))
else echo "   ✓ it did not reach the other patroller"; pass=$((pass+1)); fi

echo; echo "── 3. the sender is named, so a patroller knows who is asking ──"
has "sender name" "QPTest DQA" "$(inbox "$P2")"

echo; echo "── 4. a patroller cannot send ──"
CODE=$(curl -sk -o /dev/null -w '%{http_code}' -X POST "$BASE/api/v1/app-notifications" -H "Authorization: Bearer $P1" \
  -H 'Content-Type: application/json' -m 20 -d '{"audience":"PATROLLERS","subject":"nope"}')
check "patroller is refused the send route" "$CODE" "403"

echo; echo "── 5. the picker lists active patrollers only ──"
L=$(curl -sk "$BASE/api/v1/dashboard/qa/patrollers" -H "Authorization: Bearer $QA" -m 30)
has "lists patroller 1" "QPTEST-P1" "$L"
if grep -qF "QPTEST-P3" <<<"$L"; then echo "   ✗ lists the INACTIVE patroller"; fail=$((fail+1));
else echo "   ✓ omits the inactive patroller"; pass=$((pass+1)); fi

echo; echo "── 6. the org roles that were silently 403'd can now send ──"
CODE=$(curl -sk -o /dev/null -w '%{http_code}' -X POST "$BASE/api/v1/app-notifications" -H "Authorization: Bearer $QA" \
  -H 'Content-Type: application/json' -m 20 -d '{"audience":"STUDENTS","subject":"QPTEST wrong audience"}')
check "an audience QA may NOT use is still rejected" "$CODE" "400"

echo; echo "=== $pass passed, $fail failed ==="
[ "$fail" -eq 0 ]
