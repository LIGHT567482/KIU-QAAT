-- 065: Re-stamp every session with its coordinator's actual cohort.
--
-- Two sources put the wrong `offering_id` on a session:
--
--   • Migration 024's backfill joined `course_offerings` on course_id alone. For a course running
--     more than one study session (Day + Weekend), that join matches several rows and Postgres
--     picks an arbitrary one — so a Weekend coordinator's sessions were stamped "Day".
--   • sync-receiver created offline-synced sessions with no offering_id at all (fixed alongside
--     this migration). Since nearly every session is held offline, that is most of them.
--
-- Either way the session logs grouped by cohort put a Weekend session's students in among the Day
-- cohort's. The coordinator is the reliable signal: `ux_offerings_tenant_coordinator` makes
-- (tenant, coordinator) unique, so a coordinator owns exactly one offering.
--
-- Only rows that are wrong or missing are touched, and only when the coordinator resolves to an
-- offering — a session whose coordinator no longer runs a cohort is left exactly as it is rather
-- than being nulled out.

UPDATE sessions s
SET offering_id = o.offering_id
FROM course_offerings o
WHERE o.tenant_id = s.tenant_id
  AND o.coordinator_id = s.coordinator_id
  AND s.offering_id IS DISTINCT FROM o.offering_id;

-- Sessions whose unit belongs to a course with exactly ONE offering are unambiguous even when the
-- coordinator can't be resolved (a delegate or a since-deleted account ran it). Fill only those.
UPDATE sessions s
SET offering_id = only_one.offering_id
FROM (
    SELECT cu.unit_id, cu.tenant_id, MIN(o.offering_id::text)::uuid AS offering_id
    FROM course_units cu
    JOIN course_offerings o ON o.course_id = cu.course_id AND o.tenant_id = cu.tenant_id
    GROUP BY cu.unit_id, cu.tenant_id
    HAVING COUNT(*) = 1
) only_one
WHERE s.offering_id IS NULL
  AND s.unit_id = only_one.unit_id
  AND s.tenant_id = only_one.tenant_id;
