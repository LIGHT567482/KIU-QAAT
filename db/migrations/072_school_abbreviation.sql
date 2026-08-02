-- 072: A school carries BOTH its full name and its short form.
--
-- "School of Mathematics and Computing" is what belongs on a report, a letter and an exam board
-- paper. "SOMAC" is what everyone actually says, what fits in a table column, and what gets typed
-- into a spreadsheet. Until now `schools` had one `name` and institutions resolved the conflict by
-- entering the abbreviation as the name — which is exactly what this tenant did: its only school is
-- literally called "SOMAC". So the full name existed nowhere in the system at all.
--
-- WHY THIS IS NOT JUST A LABEL. `users.school` and `courses.school` are free TEXT, and every
-- org-scoped query matches them by string: a dean sees their college because `courses.school`
-- equals `users.school`. Rows written before today therefore hold "SOMAC". The moment an admin
-- fills in the full name, a matcher that only knew `schools.name` would compare
-- "School of Mathematics and Computing" against a thousand rows saying "SOMAC" and the dean's
-- dashboard would silently empty — the worst possible failure, because it looks like an institution
-- with no data rather than a mismatch.
--
-- The abbreviation is therefore an ALIAS the matcher accepts, not a display string. Both forms
-- resolve to the same school for as long as historic rows carry either.

ALTER TABLE schools ADD COLUMN IF NOT EXISTS abbreviation VARCHAR(32);

-- Backfill: where the existing name is plainly already an abbreviation — short, no spaces, e.g.
-- "SOMAC" — record it as the abbreviation too, so matching keeps working the instant the admin
-- replaces the name with the full title. A name with spaces is a real name and gets no guess.
UPDATE schools
   SET abbreviation = btrim(name)
 WHERE abbreviation IS NULL
   AND btrim(name) <> ''
   AND length(btrim(name)) <= 12
   AND btrim(name) NOT LIKE '% %';

-- One abbreviation per institution, case-insensitively: two colleges answering to "SOMAC" would
-- make the alias ambiguous and silently merge their scopes. Partial, so the many schools that
-- have no abbreviation yet do not collide on NULL.
CREATE UNIQUE INDEX IF NOT EXISTS ux_schools_abbreviation
    ON schools (tenant_id, lower(btrim(abbreviation)))
 WHERE abbreviation IS NOT NULL AND btrim(abbreviation) <> '';
