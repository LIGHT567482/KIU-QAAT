# QAAT — Phase 4 Pilot Deployment Checklist
**Target:** Weeks 13–14 · One department · Real students · Real sessions

---

> **⚠️ Proximity model changed since this checklist was written.** BLE beacons /
> RSSI were **removed** (migration 039). Ignore every "beacon"/"RSSI"/"Bluetooth"
> step below — there is **no beacon hardware to place, power, or calibrate**.
> Instead, proximity is proven by the phone being **on the coordinator's Wi-Fi
> hotspot LAN** plus the **live rotating room code**. The only "hardware" is the
> coordinator's laptop, which is the room hotspot + offline server. See
> [`../flow.md`](../flow.md) and [`FLOWCHART.md`](FLOWCHART.md).

---

## Pre-Pilot (Week 13)

### Infrastructure
- [ ] Staging cluster deployed: `kubectl apply -k infra/k8s/overlays/staging`
- [ ] All 5 service pods running and healthy (`kubectl get pods -n qaat-staging`)
- [ ] PostgreSQL migrations applied and verified (RLS enabled on all tables)
- [ ] Two test tenants seeded and cross-tenant isolation confirmed (run `tests/security`)
- [ ] RSA-2048 key pairs generated for pilot tenant: `make keys`
- [ ] `KEY_ENCRYPTION_KEY` set in K8s secrets (never committed)
- [ ] TLS certificate provisioned (cert-manager or Cloudflare origin cert)
- [ ] Prometheus scraping all services; Grafana accessible at monitoring URL

### Beacon Setup
- [ ] BLE beacon UUIDs registered in `ble_beacons` table for pilot venues
- [ ] RSSI threshold calibrated per venue (aim for ≥-65 dBm at ~3 m)
- [ ] Beacon battery levels checked (`battery_level` field populated)

### Coordinator Setup
- [ ] Coordinator accounts created in `users` table for pilot department
- [ ] Coordinator PWA installed on pilot Coordinator devices (Chrome Android recommended)
- [ ] Daily Manifest fetch tested from each Coordinator device
- [ ] BLE scan confirmed in each pilot venue (at least 3 readings before session start)
- [ ] Offline mode verified: disconnect device from internet, run full session, reconnect and confirm sync

### Student Setup  
- [ ] QR codes generated and emailed: `POST /api/v1/qr/generate/batch`
- [ ] 5 test students confirm QR delivery and can display QR from email
- [ ] Hardware fingerprint binding tested on 3 different device types (iOS, Android, tablet)
- [ ] Student Portal accessible and shows correct attendance %

### Admin Setup
- [ ] DQA Director account created with TOTP MFA enrolled
- [ ] VC account created with TOTP MFA enrolled
- [ ] QA Officer account created
- [ ] Attendance threshold set to pilot value (default 75%)
- [ ] Admin Dashboards accessible from desktop browsers

---

## Pilot Week 1 (Week 14) — Live Sessions

### Session 1 (Day 1 — supervised)
- [ ] Coordinator performs full Daily Fetch before session
- [ ] BLE beacon detected in venue before session start
- [ ] Lecturer scans Gate-Open QR (PENDING_LECTURER → ACTIVE transition)
- [ ] 5+ students check in successfully via QR scan
- [ ] At least 1 DEVICE_MISMATCH test performed (known test device)
- [ ] Session closed by Coordinator (not auto-close)
- [ ] Session package sealed and placed in outbox
- [ ] Sync completes within 5 minutes of session close
- [ ] QA Officer Dashboard shows session with correct student count within 60 seconds of sync

### Session 2 (Day 2 — Coordinator unassisted)
- [ ] Coordinator runs session independently without support
- [ ] All validation steps pass (BLE + QR + fingerprint)
- [ ] Sync completes without errors

### Warden Delegation Test
- [ ] Coordinator generates Warden delegation link
- [ ] Warden opens link on their phone (GPS check passes)
- [ ] Warden can scan students
- [ ] Session submits correctly after Warden closes

### Monitoring Checks (Throughout Week 14)
- [ ] No P1/P2 bugs
- [ ] Sync success rate > 99%
- [ ] No SYNC_OVERDUE alerts triggered
- [ ] Ghost lecture flag: 0 (expected — all sessions genuine)
- [ ] Grafana: `qaat_api_http_requests_total` shows expected request patterns
- [ ] Grafana: `qaat_sync_uploads_total{status="SYNCED"}` incrementing

---

## RSSI Calibration Protocol (per venue)

Walk the venue with a beacon and record RSSI at:
1. Coordinator desk (expected ≥ -55 dBm)
2. 5 m from Coordinator (expected ≥ -65 dBm)
3. Classroom door (expected < -70 dBm — outside boundary)

Set `rssi_threshold_dbm` to the minimum recorded value at the 5 m mark.
Update via: `PUT /api/v1/dashboard/dqa/thresholds`

---

## Exit Criteria (before Week 15)

- [ ] UC-01 (Standard Session): full flow passes with ≥ 10 real students
- [ ] UC-02 (Warden Delegation): passes end-to-end
- [ ] UC-03 (Device Binding Reset): QA Officer resets a student's binding successfully
- [ ] Sync success rate: **≥ 99%** over 5 sessions
- [ ] Zero cross-tenant data visible in any query (run `tests/security` against staging DB)
- [ ] All P1/P2 bugs from pilot resolved
- [ ] Coordinator training time: **< 2 hours** (target from plan.md M9)
