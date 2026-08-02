# QAAT — Phase 4 Pilot Deployment Checklist
**Target:** Weeks 13–14 · One department · Real students · Real sessions

---

> **Proximity model.** There is **no proximity hardware to place, power, or
> calibrate.** A student is proven in the room by their phone being **on the
> coordinator's Wi-Fi hotspot LAN** plus the **live rotating room code**. The only
> "hardware" is the coordinator's laptop, which is the room hotspot + offline
> server. See [`../flow.md`](../flow.md) and [`FLOWCHART.md`](FLOWCHART.md).

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

### Room / Hotspot Setup
- [ ] Coordinator laptop hotspot reaches every seat in each pilot venue
- [ ] Rotating room code legible from the back row (projector or screen share)
- [ ] Hotspot handles the expected class size concurrently

### Coordinator Setup
- [ ] Coordinator accounts created in `users` table for pilot department
- [ ] Coordinator PWA installed on pilot Coordinator devices (Chrome Android recommended)
- [ ] Daily Manifest fetch tested from each Coordinator device
- [ ] Hotspot join confirmed in each pilot venue from a test phone before session start
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
- [ ] Hotspot up and rotating room code displayed before session start
- [ ] Lecturer scans Gate-Open QR (PENDING_LECTURER → ACTIVE transition)
- [ ] 5+ students check in successfully via QR scan
- [ ] At least 1 DEVICE_MISMATCH test performed (known test device)
- [ ] Session closed by Coordinator (not auto-close)
- [ ] Session package sealed and placed in outbox
- [ ] Sync completes within 5 minutes of session close
- [ ] QA Officer Dashboard shows session with correct student count within 60 seconds of sync

### Session 2 (Day 2 — Coordinator unassisted)
- [ ] Coordinator runs session independently without support
- [ ] All validation steps pass (same-LAN + room code + QR + fingerprint)
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

## Hotspot Coverage Protocol (per venue)

Walk the venue with a test phone and confirm at each point that it can join the
coordinator's hotspot and submit a check-in:
1. Coordinator desk
2. Middle of the room
3. Back row / far corner

Also confirm the rotating room code on the projector is readable from the back row.
From **outside** the closed classroom door the hotspot should be unusable — if a phone
in the corridor can still check in, reposition the laptop or lower the hotspot's
transmit range before the pilot.

---

## Exit Criteria (before Week 15)

- [ ] UC-01 (Standard Session): full flow passes with ≥ 10 real students
- [ ] UC-02 (Warden Delegation): passes end-to-end
- [ ] UC-03 (Device Binding Reset): QA Officer resets a student's binding successfully
- [ ] Sync success rate: **≥ 99%** over 5 sessions
- [ ] Zero cross-tenant data visible in any query (run `tests/security` against staging DB)
- [ ] All P1/P2 bugs from pilot resolved
- [ ] Coordinator training time: **< 2 hours** (target from plan.md M9)
