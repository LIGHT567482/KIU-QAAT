-- app_notifications.sender_id becomes nullable: some notifications have no person behind them.
--
-- The column was declared NOT NULL when every notification was one user writing to another. Two
-- kinds since then are not:
--
--   1. the scheduler's lecture alerts (jobs/lectures.go) — "starts in 15 minutes", "attendance not
--      recorded", "please visit the QA office". They are written by a cron tick with no signed-in
--      user, and the code has always inserted NULL for the sender. Every one of those INSERTs was
--      failing on this constraint, so the entire lecturer-reminder pipeline wrote nothing. The
--      failure was invisible because the job logs the error and the next tick tries again.
--
--   2. QA patrol alerts (handlers/patrol.go). A lecturer must not be told which colleague ticked
--      them absent, so these now carry the impersonal sender "QA Patrol". Leaving the patroller's
--      user id in sender_id would defeat that anyway: the inbox query LEFT JOINs users on sender_id
--      to prefix the sender's title, and would render their honorific onto the anonymous name.
--
-- Nothing is lost to accountability. lecturer_patrol_logs keeps patroller_id, patroller_name and
-- patroller_staff_id on every row, and the QA patrol reports read them — an audit trail belongs in
-- the record, not in the courtesy alert.
--
-- Safe in both directions: dropping NOT NULL never invalidates an existing row, and every row
-- already written has a real sender id.

ALTER TABLE app_notifications ALTER COLUMN sender_id DROP NOT NULL;

COMMENT ON COLUMN app_notifications.sender_id IS
    'The user who sent this, or NULL when the institution did: scheduler alerts and QA patrol '
    'observations have no personal sender. The inbox query LEFT JOINs users on this to prefix a '
    'title, so NULL is what keeps an impersonal sender_name impersonal.';
