-- 068: Let a reader dismiss a QA message from their own inbox (the ✕ on every alert).
--
-- QA messages are addressed to an AUDIENCE (a school, a department, a role) and resolved to
-- readers at query time — there is no per-recipient row to flag, only `qa_message_reads`. Dismissal
-- reuses that table: a `dismissed_at` alongside `read_at` keeps it per user, so one dean clearing a
-- DQA broadcast never removes it from anyone else's inbox, and the message itself is untouched.

ALTER TABLE qa_message_reads
    ADD COLUMN IF NOT EXISTS dismissed_at TIMESTAMPTZ;

-- read_at has to become optional: dismissing without opening writes a row with only dismissed_at.
ALTER TABLE qa_message_reads ALTER COLUMN read_at DROP NOT NULL;
