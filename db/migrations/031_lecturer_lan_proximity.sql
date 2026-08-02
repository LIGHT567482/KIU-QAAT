-- 031 — Lecturer LAN-proximity anti-proxy gate
--
-- A remote/proxy lecturer must not be markable present. In addition to the live
-- 10s digit code (screen proximity), the staff-ID, the device fingerprint, the
-- biometric passkey and the student quorum, we add a NETWORK-proximity check: the
-- lecturer's gate scan must originate from the same public egress IP as the
-- coordinator's device (the shared-LAN model — coordinator phone + students +
-- lecturer all share one campus network/NAT). Tenant-toggleable.

-- The coordinator's egress IP, captured when the session is opened and refreshed
-- while the coordinator's screen polls the live code.
ALTER TABLE sessions
    ADD COLUMN IF NOT EXISTS coordinator_ip VARCHAR(64);

-- When true (default), the lecturer gate scan is rejected unless it comes from
-- the same egress IP as the coordinator. Admins on split networks can disable it.
ALTER TABLE tenants
    ADD COLUMN IF NOT EXISTS require_lan_proximity BOOLEAN NOT NULL DEFAULT true;
