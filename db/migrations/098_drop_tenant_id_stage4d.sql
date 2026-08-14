-- 098: Remove tenant_id — stage 4d: lecturer_presence_claims.
--
-- Column dropped and RLS disabled together, per 095.
--
-- THIS IS THE HEAVIEST PLACEHOLDER RENUMBER SO FAR, and the reason the whole removal was staged.
-- SubmitPresenceClaims inserts a claim across TWENTY bind parameters, with the tenant sitting at
-- $2 — so taking it out shifts $3…$20 down to $2…$19, every one of them, in a VALUES list whose
-- arguments are a different type at almost every position:
--
--     $2  tenant (uuid)      -> gone
--     $3  lecturer_user_id   -> $2      $10 captured_at    -> $9
--     $4  staff_id           -> $3      $14 day_of_week    -> $13
--     ...                               $16 session_date   -> $15
--
-- Get one wrong and Postgres does not necessarily complain: shift a text argument into a text slot
-- and the row is written with the wrong value in it. A claim filed with the staff id in the
-- lecturer-name column is a corrupted record of where somebody stood, and this table is APPEND-ONLY
-- by design (migration 081 revokes UPDATE and DELETE) — it could not be tidied up afterwards.
--
-- Two joins also lost their tenant equality:
--   lecturer_presence.go   the LATERAL that pairs each claim with the monitor's tick for the same
--                          lecturer, unit and day — matched on session_date + staff id, which is
--                          what actually identifies the pairing
--   lecturer_unrecorded.go the EXISTS that asks whether a lecturer filed a claim for a slot
--
-- And the list query lost its own $1, moving days -> $1 and staff -> $2.

ALTER TABLE lecturer_presence_claims DROP COLUMN IF EXISTS tenant_id CASCADE;
ALTER TABLE lecturer_presence_claims DISABLE ROW LEVEL SECURITY;

-- The two indexes migration 081 created both led with tenant_id, so the CASCADE above takes them.
-- Recreated on what the QA screens actually query: recent claims newest-first, and one lecturer's
-- claims for a day lined up against their patrol log.
CREATE INDEX IF NOT EXISTS idx_presence_claims_time
    ON lecturer_presence_claims (captured_at DESC);
CREATE INDEX IF NOT EXISTS idx_presence_claims_lecturer
    ON lecturer_presence_claims (lecturer_staff_id, session_date);
