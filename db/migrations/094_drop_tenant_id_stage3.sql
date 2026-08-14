-- 094: Remove tenant_id — stage 3.
--
-- See 092 for the staging rationale and 093 for the pattern. The rule each stage follows: a column
-- comes out only after every Go query naming it has been rewritten, and any unique index that
-- contained it is recreated rather than allowed to vanish with the CASCADE.
--
-- Go rewritten for each:
--   patroller_pins            — patrol_pin.go: read, upsert, the failed-attempt UPDATE (whose $3/$4
--                               became $2/$3), the verify UPDATE, and the admin reset DELETE
--   student_device_bindings   — student_device.go upsert
--   semester_archives         — clear_semester.go insert; semester_archive.go list/download/delete
--
-- patroller_pins keeps its ON CONFLICT (user_id) target untouched: that index was already on
-- user_id alone, because a patroller has one PIN wherever they are. Nothing to recreate.
--
-- student_device_bindings likewise conflicts on student_id alone — one student, one bound handset.
--
-- semester_archives is keyed by its own archive_id UUID and carried no unique index on tenant_id.
-- The one thing that DID change is visible in the Go: listing archives no longer filters at all,
-- because with one institution "every archive" and "this institution's archives" are the same set.

ALTER TABLE patroller_pins          DROP COLUMN IF EXISTS tenant_id CASCADE;
ALTER TABLE student_device_bindings DROP COLUMN IF EXISTS tenant_id CASCADE;
ALTER TABLE semester_archives       DROP COLUMN IF EXISTS tenant_id CASCADE;

-- Belt and braces: prove the two ON CONFLICT targets the Go relies on still exist after the
-- CASCADE above. If either had been built as a composite including tenant_id it would now be gone,
-- and the upserts would fail at runtime rather than here.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_indexes
                    WHERE tablename = 'patroller_pins'
                      AND indexdef LIKE '%UNIQUE%' AND indexdef LIKE '%(user_id)%') THEN
        CREATE UNIQUE INDEX uq_patroller_pin_user ON patroller_pins (user_id);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_indexes
                    WHERE tablename = 'student_device_bindings'
                      AND indexdef LIKE '%UNIQUE%' AND indexdef LIKE '%(student_id)%') THEN
        CREATE UNIQUE INDEX uq_student_device_student ON student_device_bindings (student_id);
    END IF;
END $$;
