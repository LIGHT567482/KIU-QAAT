-- 087: Distance learning — attendance for a class with no room.
--
-- THE PROBLEM. Every proof of presence in this system is physical. A student is present because
-- their phone is on the coordinator's hotspot in that room (onSameLAN, MANDATORY) and because they
-- typed the code shown on the screen in front of them. A lecturer is present because they scanned
-- the coordinator's QR, from that same network, with the live 10-second code. That model is the
-- reason an attendance figure here means something, and it is worth defending.
--
-- A distance / e-learning cohort has none of it. There is no room, no hotspot, no coordinator
-- standing at a door. Run through the existing path, every one of those students is rejected with
-- NOT_SAME_NETWORK and every one of their lecturers is rejected with not_same_network. So today the
-- institution's distance students have no attendance at all — and attendance decides exam
-- eligibility. Their classes simply do not exist to this system.
--
-- WHAT THIS DOES NOT DO. It does not relax the LAN gate. A single global "allow remote check-in"
-- switch would silently destroy the on-campus guarantee: a Day student could then mark themselves
-- present from a hostel bed, and no report could tell that apart from someone who walked into the
-- hall. The gate is not weakened for anybody; instead a session is allowed to say, on the record,
-- that it was DELIVERED ONLINE, and only such a session takes the online path.
--
-- HONESTY ABOUT THE PROOF. No system can prove a body is in front of a screen. What an online
-- check-in can prove is: the account is the student's (JWT), the device is not marking a second
-- person (device binding, unchanged), the lecturer had actually started the class, the student
-- belongs to that cohort, and the code was live within the last ~30 seconds (the ROTATING code,
-- not the static one — a static code with no LAN gate is a WhatsApp message away from the whole
-- year group). That is weaker than standing in a room, and the record says so rather than
-- pretending otherwise: delivery_mode travels with the session everywhere it is reported, so an
-- online tick is never quietly counted as a physical one.
--
-- WHY ON THE COHORT AS WELL AS THE SESSION. course_offerings.delivery_mode is the standing fact
-- ("this is the e-learning run of the programme") and is what decides whether an online session may
-- be opened at all. sessions.delivery_mode is the fact about the day, recorded on the day, exactly
-- like room_is_provision (085) and unscheduled (086): a cohort's mode can be edited later, and that
-- must never rewrite what a student's attendance meant at the time it was taken.

-- ── The cohort ───────────────────────────────────────────────────────────────
ALTER TABLE course_offerings
    ADD COLUMN IF NOT EXISTS delivery_mode TEXT NOT NULL DEFAULT 'IN_PERSON';

DO $$ BEGIN
    ALTER TABLE course_offerings
        ADD CONSTRAINT course_offerings_delivery_mode_check
        CHECK (delivery_mode IN ('IN_PERSON', 'ONLINE'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

COMMENT ON COLUMN course_offerings.delivery_mode IS
    'IN_PERSON (default) or ONLINE. ONLINE cohorts are distance / e-learning runs: they have no '
    'room and no hotspot, so their sessions take the online check-in path. Only a cohort marked '
    'ONLINE may have an online session opened for it — that restriction is what stops the remote '
    'path being used to mark a campus class present from off-site.';

-- Backfill from what the institution already typed. session_type is free text (VARCHAR(40)) that
-- the timetable import writes, so the spellings in the wild are "Distance", "Distance Learning",
-- "E-Learning", "eLearning", "Online". Matching them here means the existing distance cohorts work
-- the moment this lands instead of waiting for someone to tick a box on each one. Anything the
-- pattern misses is still correctable from the admin screen — the default is the safe direction.
UPDATE course_offerings
   SET delivery_mode = 'ONLINE'
 WHERE delivery_mode = 'IN_PERSON'
   AND session_type ~* '(distance|e[[:space:]._-]*learning|online|virtual|remote)';

-- ── The day ──────────────────────────────────────────────────────────────────
ALTER TABLE sessions
    ADD COLUMN IF NOT EXISTS delivery_mode TEXT NOT NULL DEFAULT 'IN_PERSON';

DO $$ BEGIN
    ALTER TABLE sessions
        ADD CONSTRAINT sessions_delivery_mode_check
        CHECK (delivery_mode IN ('IN_PERSON', 'ONLINE'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

COMMENT ON COLUMN sessions.delivery_mode IS
    'How this session was actually delivered. ONLINE sessions have no venue and no coordinator IP: '
    'the LAN proximity gate is not applied to them, the ROTATING code is required instead of the '
    'static one, and the student must be enrolled in the session''s cohort. Recorded on the day so '
    'a later edit to the cohort cannot rewrite what an attendance record meant when it was taken.';

-- "Which classes ran online this term" is a QA question over a date range, and online sessions are
-- a minority of rows.
CREATE INDEX IF NOT EXISTS ix_sessions_online
    ON sessions (tenant_id, session_date)
    WHERE delivery_mode = 'ONLINE';

-- ── A duplicate-attendance hole, found while reading this path ────────────────
--
-- attendance_logs has exactly two unique indexes and BOTH are partial on entry_method = 'QR_SCAN'.
-- The authenticated cloud check-in writes entry_method = 'AUTHENTICATED', so it was covered by
-- neither. Its only protection against a double check-in was a SELECT EXISTS a few lines above the
-- INSERT — and the comment on the INSERT's error branch ("concurrent duplicate trips the unique
-- index — treat as success") was relying on an index that does not apply to it. Two taps landing
-- together, or a phone retrying on a bad connection, therefore inserted the same student twice,
-- inflating the count for the session and the percentage the exam board reads.
--
-- The existing indexes are left alone. MANUAL_OVERRIDE is deliberately outside all of them: an
-- officer's override is a second, deliberate row about a student who may already have one, and
-- collapsing those would destroy the audit trail it exists to leave.
CREATE UNIQUE INDEX IF NOT EXISTS uq_attendance_session_student_auth
    ON attendance_logs (tenant_id, session_id, student_id)
    WHERE entry_method = 'AUTHENTICATED';
