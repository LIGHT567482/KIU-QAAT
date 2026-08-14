-- 096: Remove tenant_id — stage 4.
--
-- FROM THIS MIGRATION ON, dropping the column and disabling row-level security happen together.
-- 095 explains why: DROP COLUMN CASCADE takes the tenant_isolation POLICY but leaves RLS switched
-- ON, and a table with RLS enabled and no policy denies every row to qaat_app — silently, with no
-- error, on reads and writes alike. Doing both in one statement pair is what stops that recurring.
--
-- Go rewritten for user_schools:
--   school_alias.go  userSchools()  — the WHERE lost `us.tenant_id = $2` and its bind
--   qa_rep.go        the reps' school list
--   patrol_notify.go the monitor-notification school lookup
--
-- All three joined `schools s ON s.school_id = us.school_id AND s.tenant_id = us.tenant_id`. The
-- right-hand side of that equality no longer exists, so the condition goes; school_id is a UUID
-- primary key and identifies the school on its own. `schools.tenant_id` is untouched here — it
-- comes out in a later stage with the rest of the org tables.

ALTER TABLE user_schools DROP COLUMN IF EXISTS tenant_id CASCADE;
ALTER TABLE user_schools DISABLE ROW LEVEL SECURITY;

-- user_schools carried no unique index containing tenant_id (it was not among the 23 surveyed), so
-- there is nothing to recreate. Its (user_id, school_id) pairing is enforced by its own key.
