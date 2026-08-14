-- QAAT — the institution, and the one account that can let anybody else in.
--
-- WHY THIS FILE HAS TO EXIST. Applying every migration to an empty database produces a correct
-- schema with NOBODY IN IT: 42 tables, the qaat_app role, zero tenants and zero users. Migration
-- 038 seeds a platform super-admin and migration 064 deletes it again, because QAAT stopped being
-- multi-tenant and the cross-tenant role was removed rather than left dormant. Nothing since
-- inserts an institution or an account. So a fresh deploy came up healthy, answered /health, and
-- could not be logged into by anyone — which reads as a broken deployment rather than an empty one.
--
-- The tenants row is not merely a login requirement either: it is where the institution's own
-- settings live — the attendance threshold, the session window, the intakes and levels and titles,
-- the branding, and student_hash_key, which is the secret every coordinator's phone derives the
-- combined-class code from. An install without this row has no settings to inherit.
--
-- IDEMPOTENT. Both inserts are ON CONFLICT DO NOTHING, so re-running it against a database that
-- already has the institution changes nothing and, in particular, will NOT reset a password that
-- has since been changed.

-- ── The institution ───────────────────────────────────────────────────────────
-- Fixed UUID rather than gen_random_uuid(): it is the same id the development database uses, so a
-- dump taken there restores here without every foreign key having to be rewritten.
--
-- student_hash_key is deliberately NOT set. Its column default generates 32 fresh random bytes, so
-- each deployment gets its own — which is what you want, because it keys the student-id hashes and
-- the combined-class codes. Pinning it in a committed file would publish it.
INSERT INTO tenants (tenant_id, name, domain, rsa_key_id)
VALUES (
    '13ab41a8-0a50-401c-a095-23203a8e41be',
    'KAMPALA INTERNATIONAL UNIVERSITY',
    'kiu.ac.ug',
    'kiu-rsa-key-v1'
)
ON CONFLICT (tenant_id) DO NOTHING;

-- ── The first administrator ───────────────────────────────────────────────────
-- admin@kiu.ac.ug / Admin1234!   (bcrypt cost 12, the cost the auth service itself uses)
--
-- READ THIS BEFORE THE SYSTEM CARRIES REAL ATTENDANCE. This password is written in a file in the
-- repository, and the repository is public — so it is not a secret from anybody, it is a way to get
-- the first administrator in so they can create the real accounts. force_password_change is set,
-- which the Android app enforces; the web dashboard does not read it yet, so on the dashboard this
-- is a reminder rather than a gate. CHANGE IT BY HAND at first sign-in.
--
-- Every other account is made by this one: coordinators, the DQA office, lecturers and students are
-- all created from the admin dashboard or an import, and those get their own seeded first-login
-- words (see internal/handlers/default_passwords.go). This is the only account that has to exist
-- before anyone can do anything.
INSERT INTO users (user_id, tenant_id, email, password_hash, role, full_name, force_password_change)
VALUES (
    'a0000000-0000-0000-0000-000000000001',
    '13ab41a8-0a50-401c-a095-23203a8e41be',
    'admin@kiu.ac.ug',
    '$2a$12$/UjJjPps2yKqV.4TeZxOp.jE7axR3xvOTxk66AGUgfVcockoqZC5K',
    'ADMIN',
    'System Administrator',
    true
)
ON CONFLICT (tenant_id, email) DO NOTHING;
