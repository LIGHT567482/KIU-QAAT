-- 097: Remove tenant_id — stage 4c: admin_audit_log.
--
-- Column dropped and RLS disabled together, per the rule established in 095.
--
-- Go rewritten:
--   handlers/audit.go     RecordAudit insert; the ListAudit query; the DISTINCT action lookup
--   middleware/audit.go   the fire-and-forget insert on every mutating request
--
-- THE ONE WORTH READING TWICE is ListAudit, because it is the first query in this migration series
-- that numbers its placeholders AT RUNTIME:
--
--     args := []interface{}{tenantID}
--     ... q += " AND a.action = $" + strconv.Itoa(len(args))
--
-- Every optional filter appends to args and takes its number from the new length. Deleting the
-- tenant predicate but leaving args seeded with one element would have left every later filter
-- numbered one too high — binding the action to $2 while the statement asked for $1. That does not
-- error; it silently filters on the wrong thing. The fix is to start the slice EMPTY and open with
-- `WHERE TRUE`, so the " AND ..." fragments below still concatenate and the numbering starts at 1.
--
-- The users join also lost `AND u.tenant_id = a.tenant_id`; user_id is a UUID primary key and
-- identifies the actor without help.

ALTER TABLE admin_audit_log DROP COLUMN IF EXISTS tenant_id CASCADE;
ALTER TABLE admin_audit_log DISABLE ROW LEVEL SECURITY;
