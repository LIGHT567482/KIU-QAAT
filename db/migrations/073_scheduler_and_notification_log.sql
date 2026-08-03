-- 073: A scheduler that can be trusted, and a record of what it has already sent.
--
-- WHY THIS EXISTS. Four of the features that follow are timed: remind a lecturer 10 minutes
-- before their lecture, chase them 10 minutes after if nobody recorded attendance, escalate at
-- 20 minutes, and nag an employee who has not checked in. None of them can be built yet, because
-- nothing scheduled actually runs in this system. `/internal/cron/notify-no-shows` exists, but it
-- is gated on CRON_SECRET, which is set in no env file, and render.yaml declares no cron service.
-- The endpoint has never fired in production.
--
-- WHY NOT JUST A TICKER. An in-process ticker is the obvious fix, and it is what the gateway now
-- runs — but the services are on Render's free plan and free services SLEEP after ~15 minutes
-- idle. A sleeping process misses every window it was supposed to fire in, and wakes with no idea
-- that it did. A reminder that silently does not happen is worse than no reminder, because the
-- lecturer has been told the system will chase them.
--
-- So the scheduler does not ask "is it time to run?". It asks "which windows have elapsed since I
-- last succeeded?" and works through all of them. scheduled_job_runs is that watermark. A gateway
-- that slept for two hours wakes, sees two hours of unprocessed windows, and catches up.
--
-- Catching up means re-examining windows, which means the same lecture can be considered several
-- times. notification_log is what stops that being visible to the person: it records the
-- (subject, day, kind) of everything actually sent, so a replay is a no-op rather than a second
-- identical alert at 3am.

-- ─── The scheduler's watermark ────────────────────────────────────────────────
-- Deliberately NOT tenant-scoped: the scheduler runs across all tenants in one sweep, and a
-- per-tenant watermark would mean a tenant with no activity holds back nobody but itself. This is
-- infrastructure state, not institution data, so it carries no RLS policy and is not granted to
-- qaat_app — only the owner connection the scheduler uses can touch it.
CREATE TABLE IF NOT EXISTS scheduled_job_runs (
    job_name       VARCHAR(64) PRIMARY KEY,
    -- The end of the last window fully processed. The scheduler resumes from here.
    last_run_at    TIMESTAMPTZ NOT NULL,
    -- Bookkeeping for operators: did the last sweep finish, and how far behind was it?
    last_status    VARCHAR(20) NOT NULL DEFAULT 'OK',
    last_error     TEXT,
    last_duration_ms INTEGER,
    windows_caught_up INTEGER NOT NULL DEFAULT 0,
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ─── What has already been sent ───────────────────────────────────────────────
-- One row per notification the scheduler has decided to send. The UNIQUE constraint is the whole
-- point: it is what makes a catch-up sweep idempotent. `subject_key` is deliberately free-text
-- rather than a foreign key, because the subject differs per job — a timetable slot id for the
-- lecture reminders, an employee's AC-No for the check-in nags — and a FK per job would mean a
-- migration every time a job is added.
CREATE TABLE IF NOT EXISTS notification_log (
    log_id       UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    UUID        NOT NULL REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    -- What kind of notification: LECTURE_REMINDER, ATTENDANCE_MISSING, QA_ESCALATION,
    -- VENUE_CHANGED, EMPLOYEE_NO_CHECKIN, EMPLOYEE_CHECKOUT_REMINDER, EMPLOYEE_LATE,
    -- EMPLOYEE_EARLY_OUT.
    kind         VARCHAR(40) NOT NULL,
    -- What it was about: a slot_id, a staff_id, an ac_no — whatever identifies the thing so that
    -- "already told them about this" is answerable.
    subject_key  VARCHAR(128) NOT NULL,
    -- The civil day the notification belongs to, in institution time. Part of the key so the same
    -- lecture on Tuesday and Wednesday are separate notifications.
    subject_date DATE        NOT NULL,
    recipient_user_id UUID   REFERENCES users(user_id) ON DELETE CASCADE,
    channels     TEXT        NOT NULL DEFAULT 'APP',  -- APP, EMAIL, WHATSAPP, comma-joined
    sent_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, kind, subject_key, subject_date)
);

CREATE INDEX IF NOT EXISTS idx_notification_log_tenant_date
    ON notification_log (tenant_id, subject_date, kind);

ALTER TABLE notification_log ENABLE ROW LEVEL SECURITY;
ALTER TABLE notification_log FORCE  ROW LEVEL SECURITY;
-- DROP-then-CREATE: `CREATE POLICY` has no IF NOT EXISTS, so a database where this table was
-- created by hand would fail here and leave the migration half-applied.
DROP POLICY IF EXISTS "tenant_isolation" ON notification_log;
CREATE POLICY "tenant_isolation" ON notification_log
    FOR ALL USING (tenant_id = current_setting('app.current_tenant', true)::uuid);
GRANT SELECT, INSERT, UPDATE, DELETE ON notification_log TO qaat_app;
