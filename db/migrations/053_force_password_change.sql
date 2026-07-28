-- 053: Force a password change on first sign-in for accounts seeded with a shared default
-- password ("Student" / "Lecturer" — migration 052). Until they change it, the unified app blocks
-- them on a mandatory change-password screen. This closes the "shared default password" gap: even
-- if someone logs in first with the default, they must immediately set a private password.

ALTER TABLE users ADD COLUMN IF NOT EXISTS force_password_change BOOLEAN NOT NULL DEFAULT false;

-- Flag the never-signed-in students/lecturers (same population 052 seeded).
UPDATE users
   SET force_password_change = true
 WHERE role IN ('STUDENT', 'LECTURER') AND last_login_at IS NULL;
