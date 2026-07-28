-- QAAT — production cleanup for the single-institution (KIU) build.
-- Idempotent: safe to run more than once.
--
-- What it does:
--   1. Sets the default ADMIN login to  admin@kiu.ac.ug / Admin1234!
--   2. Removes the retired SUPER_ADMIN user(s) and the platform tenant.
--
-- Run against PROD Postgres (open the external allow-list temporarily as in light.md §5):
--   psql "postgres://qaat:<owner-pw>@dpg-…-a.oregon-postgres.render.com/qaat?sslmode=require" \
--        -f scripts/prod-kiu-cleanup.sql
--
-- NOTE: the bcrypt hash below is for the plaintext password "Admin1234!" (cost 12).
--       It was verified end-to-end (login returns a valid RS256 JWT). bcrypt hashes are
--       portable, so it validates the same password on any environment.
--       Ask the admin to change this password after first login.

BEGIN;

-- ── 1. Default admin credentials: admin@kiu.ac.ug / Admin1234! ────────────────
-- Updates the existing KIU admin row (matched by email). If your KIU admin uses a
-- different email, change it here.
UPDATE users
   SET password_hash = '$2b$12$94aOId3Qt3gAuUdh8gYV1u4Yac01ot9k0JzbWD6479wIGp7qoGpX.'
 WHERE email = 'admin@kiu.ac.ug'
   AND role  = 'ADMIN';

-- Sanity: how many admin rows were targeted (expect 1).
DO $$
DECLARE n int;
BEGIN
  SELECT count(*) INTO n FROM users WHERE email = 'admin@kiu.ac.ug' AND role = 'ADMIN';
  IF n = 0 THEN
    RAISE NOTICE 'No admin@kiu.ac.ug ADMIN row found — create one, then re-run, or adjust the email above.';
  ELSE
    RAISE NOTICE 'admin@kiu.ac.ug password set to Admin1234! (rows: %)', n;
  END IF;
END $$;

-- ── 2. Remove the retired super-admin ─────────────────────────────────────────
-- Delete SUPER_ADMIN user(s) first (they belong to the platform tenant), then any
-- RSA keys tied to the platform tenant, then the platform tenant itself.
DELETE FROM users   WHERE role = 'SUPER_ADMIN';
DELETE FROM tenant_rsa_keys WHERE tenant_id = '00000000-0000-0000-0000-000000000000';
DELETE FROM tenants WHERE tenant_id = '00000000-0000-0000-0000-000000000000';

-- Verify nothing is left.
DO $$
DECLARE sa int; pt int;
BEGIN
  SELECT count(*) INTO sa FROM users   WHERE role = 'SUPER_ADMIN';
  SELECT count(*) INTO pt FROM tenants WHERE tenant_id = '00000000-0000-0000-0000-000000000000';
  RAISE NOTICE 'super_admins remaining: %, platform tenant remaining: %', sa, pt;
END $$;

COMMIT;
