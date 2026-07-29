-- DQA ⇄ QA-officer messaging (in-app inbox). The Director of Quality Assurance can
-- share reports/notifications to QA officers — all of them, or scoped by department or
-- by college/school — and QA officers can message the DQA back. Optional file attachment
-- (e.g. a report PDF/spreadsheet) stored inline.
CREATE TABLE IF NOT EXISTS qa_messages (
    message_id      UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID        NOT NULL REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    sender_id       UUID        NOT NULL,
    sender_name     TEXT        NOT NULL,
    sender_role     TEXT        NOT NULL,
    -- Who receives it:
    --   ALL_QA      → every QA officer (DQA → all)
    --   DEPARTMENT  → QA officers whose department = audience_value (DQA → dept)
    --   SCHOOL      → QA officers whose school = audience_value (DQA → college/school)
    --   DQA         → the DQA director(s) (QA officer → DQA)
    audience        TEXT        NOT NULL CHECK (audience IN ('ALL_QA','DEPARTMENT','SCHOOL','DQA')),
    audience_value  TEXT,
    subject         TEXT        NOT NULL,
    body            TEXT        NOT NULL DEFAULT '',
    attachment_name TEXT,
    attachment_mime TEXT,
    attachment_data BYTEA,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_qa_messages_tenant_created ON qa_messages (tenant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_qa_messages_audience       ON qa_messages (tenant_id, audience, audience_value);

-- Per-recipient read state (unread badges).
CREATE TABLE IF NOT EXISTS qa_message_reads (
    message_id  UUID        NOT NULL REFERENCES qa_messages(message_id) ON DELETE CASCADE,
    tenant_id   UUID        NOT NULL REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    user_id     UUID        NOT NULL,
    read_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (message_id, user_id)
);

-- RLS: ENABLE (not FORCE) so the owner-based privileged services still work on managed
-- Postgres, while the qaat_app data-plane role is tenant-isolated (see light.md §5).
ALTER TABLE qa_messages      ENABLE ROW LEVEL SECURITY;
ALTER TABLE qa_message_reads ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON qa_messages
    FOR ALL USING (tenant_id = current_setting('app.current_tenant', true)::uuid);
CREATE POLICY tenant_isolation ON qa_message_reads
    FOR ALL USING (tenant_id = current_setting('app.current_tenant', true)::uuid);
