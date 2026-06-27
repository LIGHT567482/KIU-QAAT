-- 026 — Titles (Prof./Dr./Eng. …) + gender for staff (next.txt batch)
--
-- Titles are an admin-defined list (like intakes/levels). Gender is a small fixed
-- set chosen in the UI. Applied to staff users (coordinators) and to lecturers.

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS title  VARCHAR(40),
    ADD COLUMN IF NOT EXISTS gender VARCHAR(20);

ALTER TABLE lecturers
    ADD COLUMN IF NOT EXISTS title  VARCHAR(40),
    ADD COLUMN IF NOT EXISTS gender VARCHAR(20);

ALTER TABLE tenants
    ADD COLUMN IF NOT EXISTS titles TEXT[] NOT NULL
        DEFAULT ARRAY['Prof.', 'Dr.', 'Eng.', 'Mr.', 'Mrs.', 'Ms.'];
