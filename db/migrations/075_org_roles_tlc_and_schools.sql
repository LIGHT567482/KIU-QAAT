-- 075: A QA handler covers several schools, a lecturer belongs to one, and the timetable
--      gets an owner who is not the admin.
--
-- THREE CHANGES, one migration, because they all touch how a person is filed under the org tree.
--
-- 1. A QA SCHOOL HANDLER HANDLES SCHOOLS, PLURAL. Scope today is a single free-text
--    `users.school`, matched by NAME against `courses.school`. One handler is routinely given
--    more than one school, and there was nowhere to put the second: the account could only ever
--    see one of them, and the rest of their work was invisible to them. A join table replaces the
--    single column. `users.school` is kept and still written, because DEAN is genuinely
--    single-school and every existing query reads it — this adds a capability rather than
--    rewriting the scoping model underneath the roles that were fine.
--
-- 2. A LECTURER BELONGS TO A SCHOOL. Migration e6a6874 removed the lecturer's own department and
--    derived it from the units they teach, so a lecturer spanning two colleges appears under
--    both. That is still true for DEPARTMENT and HOD scoping, which keeps working untouched.
--    But the institution's rule is that every lecturer sits under a college, including one who
--    has not been assigned any unit yet — and a derived field cannot express that, because an
--    unassigned lecturer derives nothing and is invisible to every org role (the join is inner).
--    So school is stored; department stays derived.
--
-- 3. TLC OWNS THE TIMETABLE. The Teaching & Learning Centre maintains the timetable, not the IT
--    administrator. Adding the role here lets the route guards move in the same deploy.

-- ─── 1. A user may handle many schools ────────────────────────────────────────
CREATE TABLE IF NOT EXISTS user_schools (
    user_id    UUID NOT NULL REFERENCES users(user_id)   ON DELETE CASCADE,
    tenant_id  UUID NOT NULL REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    school_id  UUID NOT NULL REFERENCES schools(school_id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, school_id)
);
CREATE INDEX IF NOT EXISTS idx_user_schools_tenant_user ON user_schools (tenant_id, user_id);

-- Backfill from the single column so nobody loses the school they already had. Matched on name
-- OR abbreviation because `users.school` is free text and 072 introduced abbreviations — a
-- handler recorded against "SOMAC" must still resolve to the School of Computing.
INSERT INTO user_schools (user_id, tenant_id, school_id)
SELECT u.user_id, u.tenant_id, s.school_id
  FROM users u
  JOIN schools s
    ON s.tenant_id = u.tenant_id
   AND (btrim(lower(s.name)) = btrim(lower(u.school))
        OR btrim(lower(COALESCE(s.abbreviation,''))) = btrim(lower(u.school)))
 WHERE COALESCE(u.school, '') <> ''
ON CONFLICT (user_id, school_id) DO NOTHING;

ALTER TABLE user_schools ENABLE ROW LEVEL SECURITY;
ALTER TABLE user_schools FORCE  ROW LEVEL SECURITY;
-- DROP-then-CREATE: CREATE POLICY has no IF NOT EXISTS, so a hand-built database would fail
-- here and leave the migration half-applied.
DROP POLICY IF EXISTS "tenant_isolation" ON user_schools;
CREATE POLICY "tenant_isolation" ON user_schools
    FOR ALL USING (tenant_id = current_setting('app.current_tenant', true)::uuid);
GRANT SELECT, INSERT, UPDATE, DELETE ON user_schools TO qaat_app;

-- ─── 2. A lecturer's home school ──────────────────────────────────────────────
ALTER TABLE lecturers ADD COLUMN IF NOT EXISTS school_id UUID REFERENCES schools(school_id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_lecturers_school ON lecturers (tenant_id, school_id);

-- Seed it from what the lecturer already teaches, so existing staff are filed without anyone
-- re-entering them. A lecturer teaching across two colleges gets the one they teach in most;
-- an admin can correct it, and their DEPARTMENT scoping is unaffected either way.
UPDATE lecturers l
   SET school_id = best.school_id
  FROM (
        SELECT la.lecturer_id, s.school_id,
               ROW_NUMBER() OVER (PARTITION BY la.lecturer_id ORDER BY COUNT(*) DESC, s.school_id) AS rn
          FROM lecturer_assignments la
          JOIN course_units cu ON cu.unit_id = la.unit_id AND cu.tenant_id = la.tenant_id
          JOIN courses c       ON c.course_id = cu.course_id AND c.tenant_id = cu.tenant_id
          JOIN schools s       ON s.tenant_id = c.tenant_id
                              AND btrim(lower(s.name)) = btrim(lower(c.school))
         GROUP BY la.lecturer_id, s.school_id
       ) best
 WHERE best.lecturer_id = l.lecturer_id
   AND best.rn = 1
   AND l.school_id IS NULL;

-- ─── 3. The TLC role ──────────────────────────────────────────────────────────
-- ADD VALUE is transaction-safe in PG12+ as long as the new label is not used in the same
-- transaction — it is not; the route guards read it at runtime.
ALTER TYPE user_role_enum ADD VALUE IF NOT EXISTS 'TLC';
