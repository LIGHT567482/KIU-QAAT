-- 070: Re-seed the default sign-in password for every student/lecturer who still cannot sign in.
--
-- WHY THIS IS NEEDED AGAIN, after 052 did the same thing.
--
-- 052 was a one-off repair of the accounts that existed when the unified app arrived. Everything
-- created AFTERWARDS went through the handlers — and one of them, "add a student" on the admin
-- dashboard, was still seeding `randPassword()`: a random throwaway from the era when a student's
-- QR was their key and the password was never meant to be typed. The QR subsystem was removed in
-- 063, and nothing replaced that password. So every student added from the dashboard since then
-- has an account that resolves perfectly by registration number and then fails authentication with
-- "invalid email or password", because the only string that opens it was random bytes discarded at
-- creation. The handler now seeds the documented default; this repairs the ones already stranded.
--
-- CASING. 052 seeded 'Student'/'Lecturer'; the documented and now-canonical default is lower-case
-- 'student'/'lecturer'. auth-service accepts either spelling for an account still flagged
-- force_password_change (see matchesSeededDefault), so this migration does not strand anyone who
-- was told the old capitalisation — but new hashes are written in the canonical form.
--
-- SAFETY. Same population and same guarantee as 052: only accounts that have NEVER signed in
-- (last_login_at IS NULL) are touched, so a password anyone has actually chosen is never
-- clobbered, and re-running on redeploy is a no-op in effect. pgcrypto's bcrypt ($2a$) verifies
-- with Go's bcrypt.

UPDATE users
   SET password_hash = crypt('student', gen_salt('bf', 10)),
       force_password_change = true
 WHERE role = 'STUDENT' AND last_login_at IS NULL;

UPDATE users
   SET password_hash = crypt('lecturer', gen_salt('bf', 10)),
       force_password_change = true
 WHERE role = 'LECTURER' AND last_login_at IS NULL;
