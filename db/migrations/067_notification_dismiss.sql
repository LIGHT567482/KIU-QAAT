-- 067: Let a recipient dismiss a notification from their own inbox.
--
-- Every alert now carries an ✕. Dismissing is PER RECIPIENT, not per notification: one
-- notification row is fanned out to many `notification_recipients` rows, so a student clearing
-- their copy of a cohort-wide alert must not delete it for the rest of the cohort. The sender's
-- record and the audit trail are untouched — the row stays, it is only hidden from that inbox.

ALTER TABLE notification_recipients
    ADD COLUMN IF NOT EXISTS dismissed_at TIMESTAMPTZ;

-- The inbox query filters on it, so index the "still visible" case.
CREATE INDEX IF NOT EXISTS idx_notif_recipients_visible
    ON notification_recipients (tenant_id, recipient_user_id)
    WHERE dismissed_at IS NULL;
