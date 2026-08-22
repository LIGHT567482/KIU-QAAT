-- 100: Persist U-Panel attendance so QAAT can use it as its own data.
--
-- Contabo U-Panel is a live ledger of student check-ins, lecturer class sittings / sign-ins,
-- and admin campus arrival/departure. Until now the gateway only proxied those documents for a
-- side-by-side view. This table is the copy QAAT owns: reports, filters and the student /
-- lecturer / employee pages read from here after each fetch, so a U-Panel outage does not empty
-- the screens, and the rows are dimensions inside this application rather than a foreign overlay.
--
-- WHY A SEPARATE TABLE. Student check-ins here are not QAAT session logs (attendance_logs is
-- append-only and keyed on a QAAT session_id). Lecturer sittings are not coordinator START/END
-- and must not be merged with the QA patrol witness. Admin campus presence is ingested into
-- employee_attendance_logs as well (source = UPANEL) so the employee report uses it; this table
-- remains the canonical import and the source for the student and lecturer U-Panel tabs.
--
-- No tenant_id: one institution. RLS is therefore off — see 095.

CREATE TABLE IF NOT EXISTS upanel_attendance (
    record_id     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    kind          VARCHAR(16) NOT NULL CHECK (kind IN ('student', 'lecturer', 'admin')),
    external_id   TEXT        NOT NULL,
    person_id     TEXT        NOT NULL DEFAULT '',
    person_name   TEXT        NOT NULL DEFAULT '',
    staff_id      TEXT        NOT NULL DEFAULT '',
    present       BOOLEAN     NOT NULL DEFAULT true,
    event_type    TEXT        NOT NULL DEFAULT '',
    occurred_at   TIMESTAMPTZ,
    closed_at     TIMESTAMPTZ,
    list_id       TEXT        NOT NULL DEFAULT '',
    session_id    TEXT        NOT NULL DEFAULT '',
    course        TEXT        NOT NULL DEFAULT '',
    unit_name     TEXT        NOT NULL DEFAULT '',
    lecturer      TEXT        NOT NULL DEFAULT '',
    room          TEXT        NOT NULL DEFAULT '',
    year_label    TEXT        NOT NULL DEFAULT '',
    semester      TEXT        NOT NULL DEFAULT '',
    program       TEXT        NOT NULL DEFAULT '',
    source        TEXT        NOT NULL DEFAULT 'u-panel',
    imported_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (kind, external_id)
);

CREATE INDEX IF NOT EXISTS ix_upanel_attendance_kind_when
    ON upanel_attendance (kind, occurred_at DESC NULLS LAST);

CREATE INDEX IF NOT EXISTS ix_upanel_attendance_person
    ON upanel_attendance (kind, person_id);

ALTER TABLE upanel_attendance DISABLE ROW LEVEL SECURITY;

GRANT SELECT, INSERT, UPDATE ON upanel_attendance TO qaat_app;
