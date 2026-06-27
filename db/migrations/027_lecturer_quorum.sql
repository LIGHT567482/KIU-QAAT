-- 027 — Lecturer attendance quorum ratio (next.txt batch)
--
-- A lecturer's presence is only recorded if a sufficient SHARE of the enrolled
-- students actually attended — a ratio, not an absolute count, so it works for
-- units with only one or two students. required = GREATEST(1, CEIL(ratio*enrolled)).

ALTER TABLE tenants
    ADD COLUMN IF NOT EXISTS lecturer_attendance_ratio NUMERIC NOT NULL DEFAULT 0.5;
