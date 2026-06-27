-- 025 — Coordinator contacts, lecturer phone/staff-ID generation (next.txt batch)
--
-- * Coordinators (and other staff users) get phone / whatsapp / registration_number
--   for the coordinators directory.
-- * Lecturers already have a `phone` column (migration 011) — surfaced in the UI.
-- * Tenants get a `staff_id_prefix` (short name, e.g. "KIU") used to AUTO-GENERATE
--   a lecturer staff ID (<prefix>/STAFF/00001) when one isn't entered manually.

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS phone               VARCHAR(40),
    ADD COLUMN IF NOT EXISTS whatsapp            VARCHAR(40),
    ADD COLUMN IF NOT EXISTS registration_number VARCHAR(64);

ALTER TABLE tenants
    ADD COLUMN IF NOT EXISTS staff_id_prefix VARCHAR(16);

-- A generated staff ID must be unique within a tenant (manual ones too).
CREATE UNIQUE INDEX IF NOT EXISTS ux_lecturers_tenant_staffid
    ON lecturers (tenant_id, staff_id)
    WHERE staff_id IS NOT NULL AND staff_id <> '';
