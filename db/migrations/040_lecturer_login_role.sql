-- Migration 040: Lecturer login role + lecturer<->user link.
--
-- Lecturers gain a password account (role LECTURER in users) so they can sign in
-- to a lecturer dashboard, the same way coordinators do. A nullable user_id on
-- lecturers links the lecturer record to its login account (set when the account
-- is created), so the dashboard can resolve "which lecturer am I" without relying
-- on email matching.

ALTER TYPE user_role_enum ADD VALUE IF NOT EXISTS 'LECTURER';

ALTER TABLE lecturers
    ADD COLUMN IF NOT EXISTS user_id UUID REFERENCES users(user_id) ON DELETE SET NULL;
