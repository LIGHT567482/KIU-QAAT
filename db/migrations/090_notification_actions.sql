-- 090: a notification that can be answered, and the answer offered before it is needed.
--
-- THE SITUATION. A QA monitor walks a corridor, finds a room empty, and files "not taught" against
-- a named lecturer. The lecturer gets a notification saying so. That notification is, today, a dead
-- end: it states an accusation and offers nothing to do about it. The lecturer's own account — the
-- presence claim, filed from their phone — exists, but it lives on a different screen that they have
-- to know about, find, and reach before they forget which lecture was meant.
--
-- Worse, the monitor's tick syncs from a handset that may have been offline for a day. So the
-- notification frequently arrives long after the lecture, at which point "where were you at eleven
-- on Tuesday" is a question the lecturer answers from memory against a record made at the time. The
-- asymmetry is the whole problem: one side is contemporaneous and the other is a recollection.
--
-- TWO CHANGES, and they are the same change from two directions.
--
--   1. The notification carries an ACTION. "You were recorded as NOT TAUGHT" now travels with
--      "respond to this", pointing at the exact lecture, so the reply is one tap from the accusation
--      rather than a screen the lecturer has to go looking for.
--
--   2. The lecturer does not have to wait for it. As soon as a lecture's time has ELAPSED with no
--      gate record against it, it is offered to them to account for — while they still remember,
--      and usually before any monitor's tick has even synced. A record made at the time is worth
--      more than one made in an argument a week later, and this is the only way the lecturer gets
--      to make one.
--
-- action_ref identifies WHICH lecture, as unit|date|HH:MM — the same triple the round and the
-- monitor's log key on, so the reply and the tick are about a lecture both sides can name.

ALTER TABLE app_notifications
    ADD COLUMN IF NOT EXISTS action     TEXT,
    ADD COLUMN IF NOT EXISTS action_ref TEXT;

COMMENT ON COLUMN app_notifications.action IS
    'What the recipient can DO about this message, if anything. APPEAL_NOT_TAUGHT means a QA '
    'monitor recorded the lecturer as not teaching and the lecturer may file their own account of '
    'it. NULL for the ordinary message that is only to be read.';

COMMENT ON COLUMN app_notifications.action_ref IS
    'Which lecture the action is about, as unit_id|YYYY-MM-DD|HH:MM — the same key the round and '
    'lecturer_patrol_logs use, so a reply can be matched to the tick it answers.';

-- "Which of my messages need a response" is the query the lecturer's inbox runs on every open, and
-- actionable ones are a small minority of rows.
CREATE INDEX IF NOT EXISTS ix_app_notifications_action
    ON app_notifications (tenant_id, created_at DESC)
    WHERE action IS NOT NULL;
