-- Cross-role in-app notifications for the phone app:
--   • a LECTURER   → the students of his unit(s), or the coordinator(s) of those units
--   • a COORDINATOR → the students of his cohort, or the lecturer(s) of his course units
-- Recipients are materialised at send time (one row per recipient user), so a reader's
-- inbox is a trivial, fast lookup and read/unread is naturally per-recipient.
CREATE TABLE IF NOT EXISTS app_notifications (
    notification_id UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID        NOT NULL REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    sender_id       UUID        NOT NULL,
    sender_name     TEXT        NOT NULL,
    sender_role     TEXT        NOT NULL,
    audience        TEXT        NOT NULL,          -- STUDENTS | COORDINATOR | LECTURERS (by sender role)
    unit_id         VARCHAR(50),                   -- optional scope (a specific course unit)
    subject         TEXT        NOT NULL,
    body            TEXT        NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_app_notifications_tenant_created ON app_notifications (tenant_id, created_at DESC);

CREATE TABLE IF NOT EXISTS notification_recipients (
    notification_id   UUID        NOT NULL REFERENCES app_notifications(notification_id) ON DELETE CASCADE,
    tenant_id         UUID        NOT NULL REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    recipient_user_id UUID        NOT NULL,
    read_at           TIMESTAMPTZ,
    PRIMARY KEY (notification_id, recipient_user_id)
);
CREATE INDEX IF NOT EXISTS idx_notif_recipient ON notification_recipients (tenant_id, recipient_user_id, read_at);

-- RLS: ENABLE (not FORCE) so owner-based services keep working on managed Postgres.
ALTER TABLE app_notifications      ENABLE ROW LEVEL SECURITY;
ALTER TABLE notification_recipients ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON app_notifications
    FOR ALL USING (tenant_id = current_setting('app.current_tenant', true)::uuid);
CREATE POLICY tenant_isolation ON notification_recipients
    FOR ALL USING (tenant_id = current_setting('app.current_tenant', true)::uuid);
