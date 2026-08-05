-- 078: Stop locking a patroller to one handset.
--
-- Migration 069 bound a patrol account to the first phone that claimed it, and enforced it twice:
-- in the gateway (checkPatrolDevice refused a mismatched X-Device-Fingerprint) and here, with a
-- UNIQUE index on device_fingerprint_hash so one phone could serve only one patroller.
--
-- The gateway half is commented out in handlers/patrol.go. This is the other half — without
-- dropping the index, two patrollers sharing a handset still collide on it, and the second one's
-- bind fails with a unique violation. Commenting out the code alone would have left the lock
-- half-standing, which is worse than either state: it would refuse in a way no code explains.
--
-- WHAT IS KEPT. The table stays and the gateway still upserts into it, so "which phone filed this
-- tick" remains answerable, ListPatrolBindings still shows an administrator who is on what, and
-- ReleasePatrolBinding still works. The binding is now a RECORD rather than a CLAIM.
--
-- WHAT IT COSTS. The handset was one of the two factors behind a patrol tick, and a tick accuses a
-- named lecturer of not teaching. With it gone, a lifted token can be replayed from any phone, and
-- the patrol PIN (migration 071) is the only remaining factor. The PIN is verified server-side so
-- it still holds — but it is now holding alone. Restoring is: recreate the index below and
-- un-comment the two blocks in handlers/patrol.go.
--
--   CREATE UNIQUE INDEX uq_patrol_device_one_patroller
--       ON patroller_device_bindings (device_fingerprint_hash);

DROP INDEX IF EXISTS uq_patrol_device_one_patroller;

-- A non-unique index in its place: the lookup by fingerprint is still worth having for the admin
-- screens and for tracing which account a handset has been used by.
CREATE INDEX IF NOT EXISTS idx_patrol_device_fingerprint
    ON patroller_device_bindings (device_fingerprint_hash);
