-- 052: Seed default sign-in passwords for the unified "KIU QAAT" app.
--
-- Students and lecturers historically had NO usable password (QR / passwordless login), so their
-- users.password_hash is a random throwaway. The unified app signs everyone in with a password, so
-- give them a known default they can change later: students -> "Student", lecturers -> "Lecturer".
--
-- Idempotent + non-destructive: only rows that have NEVER signed in (last_login_at IS NULL) are
-- seeded, so re-running on redeploy never clobbers a password the user has since changed.
-- Uses pgcrypto's bcrypt (crypt/gen_salt('bf')) — hashes are $2a$ and verify with Go's bcrypt.

UPDATE users
   SET password_hash = crypt('Student', gen_salt('bf', 10))
 WHERE role = 'STUDENT' AND last_login_at IS NULL;

UPDATE users
   SET password_hash = crypt('Lecturer', gen_salt('bf', 10))
 WHERE role = 'LECTURER' AND last_login_at IS NULL;
