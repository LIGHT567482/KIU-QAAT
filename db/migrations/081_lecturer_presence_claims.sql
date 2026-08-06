-- 081: A lecturer's own record of being in the room.
--
-- THE PROBLEM. A QA patrol tick is currently the only account of whether a lecture happened, and
-- it is one-sided by construction: the patroller records what they found when they arrived. A
-- lecturer marked NOT TAUGHT who was, in fact, teaching — in a room the patroller never reached,
-- or reached after the class had moved on — has nothing to answer with except their word, offered
-- after the fact against a timestamped record. That is not a fair hearing, and it is the single
-- most common complaint about the patrol round.
--
-- WHAT THIS IS. The lecturer's side of the same moment, captured at the moment: they press one
-- button in the app while standing in the room, and the phone writes down where it is
-- (latitude/longitude from GPS), when (the phone's clock), and which timetabled slot that
-- combination lands in. It is filed immediately and synced when there is signal.
--
-- WHAT IT IS NOT. It is not proof and it does not overturn anything. A phone reports the
-- coordinates it is given, and a determined person can give it false ones. Its value is that it is
-- CONTEMPORANEOUS and SPECIFIC: a claim filed at 14:07 at a fixed point, matched to the 14:00 slot
-- on the timetable, is a very different object from a recollection offered a week later. Two
-- independent records that disagree is exactly the thing quality assurance exists to look at — so
-- this is stored alongside the patrol log, never merged with it, and neither one silently wins.
--
-- OFFLINE IS THE POINT. Lecture rooms are where signal is worst, and a lecturer disputing a patrol
-- tick is precisely someone who could not reach the server at the time. GPS needs no network. The
-- phone resolves the slot against its own cached weekly timetable, files the claim into encrypted
-- local storage, and uploads later — so the record exists from the moment it is made, not from the
-- moment connectivity returns. captured_at is therefore the PHONE's clock, and is kept as given
-- (see the clamp below for the one case where it is not).

CREATE TABLE IF NOT EXISTS lecturer_presence_claims (
    claim_id        UUID        PRIMARY KEY,
    tenant_id       UUID        NOT NULL REFERENCES tenants(tenant_id) ON DELETE CASCADE,

    -- Who. Both, deliberately: user_id is the account that filed it and is what RLS and the
    -- dashboards join on; staff_id is what the patrol log records, and the two records have to be
    -- lined up against each other by a human reading a screen.
    lecturer_user_id UUID       NOT NULL,
    lecturer_staff_id VARCHAR(50) NOT NULL DEFAULT '',
    lecturer_name    TEXT       NOT NULL DEFAULT '',

    -- Where. NULL when the phone could not get a fix — a claim with no coordinates is still a
    -- claim ("I was here at 14:07"), and refusing to file it would punish the lecturer for being
    -- in a basement lecture theatre. location_status says which case this is, so a reviewer is
    -- never left guessing whether 0,0 means the Gulf of Guinea.
    latitude        DOUBLE PRECISION,
    longitude       DOUBLE PRECISION,
    accuracy_metres DOUBLE PRECISION,
    location_status TEXT        NOT NULL DEFAULT 'OK'
        CHECK (location_status IN ('OK', 'NO_FIX', 'PERMISSION_DENIED', 'DISABLED')),

    -- When, by the phone that filed it.
    captured_at     TIMESTAMPTZ NOT NULL,

    -- Which lecture the phone matched it to, from its cached timetable. All nullable/blank-able:
    -- a claim filed at a time nothing is timetabled is itself informative (it may be the lecturer
    -- who is wrong about the day), and dropping it would hide that.
    unit_id         VARCHAR(50),
    unit_name       TEXT        NOT NULL DEFAULT '',
    room            TEXT        NOT NULL DEFAULT '',
    day_of_week     SMALLINT,
    scheduled_time  VARCHAR(5)  NOT NULL DEFAULT '',   -- 'HH:MM'
    session_date    DATE,
    -- How the phone arrived at that slot, so a reviewer can weigh it:
    --   IN_SLOT   — captured between the slot's start and end. The strong case.
    --   NEAR_SLOT — captured within the grace window either side (a class that ran over).
    --   NEAREST   — nothing was running; the closest slot that day is named for context.
    --   NONE      — nothing timetabled for this lecturer that day at all.
    match_kind      TEXT        NOT NULL DEFAULT 'NONE'
        CHECK (match_kind IN ('IN_SLOT', 'NEAR_SLOT', 'NEAREST', 'NONE')),
    -- Signed minutes from the slot's start: negative = filed early, positive = filed late. The
    -- number a reviewer actually wants when the patrol log says 14:00 and this says 14:07.
    minutes_from_start INTEGER,

    note            TEXT        NOT NULL DEFAULT '',

    -- Provenance. The handset that filed it, hashed the same way the patrol bindings are, so a
    -- burst of claims from one device for several lecturers is visible without storing anything
    -- that identifies a phone in the clear.
    device_hash     TEXT        NOT NULL DEFAULT '',
    -- When it reached the server. The gap between this and captured_at is how long the phone was
    -- offline, which is itself part of the story.
    received_at     TIMESTAMPTZ NOT NULL DEFAULT now(),

    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- The claim_id is minted ON THE PHONE, which is what makes the upload idempotent: a batch that is
-- retried after a timeout — the normal case on a bad connection — must not file the same moment
-- twice. The primary key does that work; the upload does ON CONFLICT DO NOTHING against it.

-- What the QA screens actually query: the recent claims for an institution, newest first.
CREATE INDEX IF NOT EXISTS idx_presence_claims_tenant_time
    ON lecturer_presence_claims (tenant_id, captured_at DESC);
-- And the other direction: one lecturer's claims, which is how a dispute is read — line this up
-- against that lecturer's patrol logs for the same day.
CREATE INDEX IF NOT EXISTS idx_presence_claims_lecturer
    ON lecturer_presence_claims (tenant_id, lecturer_staff_id, session_date);

-- Tenant isolation, same rule as every other tenant table, and FORCED — see migration 079. The
-- policy comes first because forcing a table that has none denies every row to every non-superuser
-- silently, and this one is read by the data-plane role on the QA dashboards.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_policy WHERE polrelid = 'lecturer_presence_claims'::regclass) THEN
        CREATE POLICY tenant_isolation ON lecturer_presence_claims
            USING (tenant_id = (current_setting('app.current_tenant', true))::uuid)
            WITH CHECK (tenant_id = (current_setting('app.current_tenant', true))::uuid);
    END IF;
END $$;

ALTER TABLE lecturer_presence_claims ENABLE ROW LEVEL SECURITY;
ALTER TABLE lecturer_presence_claims FORCE  ROW LEVEL SECURITY;

GRANT SELECT, INSERT ON lecturer_presence_claims TO qaat_app;

-- No UPDATE and no DELETE. This is a contemporaneous record, and the whole reason it is worth
-- anything in a dispute is that it cannot be tidied up afterwards — not by the lecturer whose
-- account filed it, and not by the office it is filed against. Corrections are new rows, exactly
-- as with attendance_logs.
REVOKE UPDATE, DELETE ON lecturer_presence_claims FROM qaat_app;

COMMENT ON TABLE lecturer_presence_claims IS
    'A lecturer''s own contemporaneous record of being in the room: GPS fix, phone clock, and the '
    'timetabled slot the phone matched them to. Filed offline, synced later, read beside the QA '
    'patrol log for the same slot. Append-only.';
