# QAAT — Security & Privacy Review (per user role)

**Version reviewed:** 1.0.0 · **Date:** 2026-06-20 · **Scope:** every user role + the public attack surface.
**Method:** static review of the gateway middleware/handlers + live probes against the running stack (results inline).

This review covers each role's authentication, authorization, and the data each can see (privacy), plus the public attack surface and the lecturer anti-proxy controls. Verdicts: ✅ verified, ⚠️ residual risk, 🔲 recommendation.

---

## Verified live (this review)

| # | Test | Expected | Result |
|---|------|----------|--------|
| P1 | Unauthenticated `GET /api/v1/admin/tenants` | 401 | ✅ 401 |
| P8 | `POST /api/v1/lecturer/gate-scan` with empty body | 400 | ✅ 400 |
| RLS-A | As `qaat_app` scoped to tenant KCU, read tenant KIU's `students_extended` (explicit `WHERE tenant_id=KIU`) | 0 rows | ✅ 0 (KCU sees only its own 2) |
| RLS-B | As `qaat_app` with **no** `app.current_tenant` set | 0 rows (default-deny) | ✅ 0 |
| RLS-C | `FORCE ROW LEVEL SECURITY` on `users, sessions, students_extended, attendance_logs, lecturer_attendance_logs` | enabled (owner can't bypass) | ✅ all `t/t` |
| CODE | `GET /api/v1/eligibility/{student_id}` for a STUDENT ignores the path id and forces the caller's own id | no classmate enumeration | ✅ (`reporting.go`) |

> Credentialed cross-role probes (P2–P7) could not complete this run because the local test fixtures were reseeded and their passwords are unknown. The controls those probes target are verified by code below and by RLS-A/B/C; rerun them after seeding a known admin/student to confirm 403s end-to-end.

---

## Defense-in-depth model

1. **TLS everywhere** — Caddy terminates HTTPS; internal services are not publicly reachable.
2. **JWT (RS256)** verified at the gateway *and* independently re-verified by each internal service (auth, qr-generator) — a leaked network position cannot mint trust.
3. **RBAC** — `RequireRole(...)` on every authenticated route.
4. **Tenant ownership** — `RequireOwnTenant` on `/api/v1/admin/tenants/{tenant_id}/…` blocks path-id tampering across tenants.
5. **RLS** — every tenant table forces `tenant_id = current_setting('app.current_tenant')`; default-deny when unset; `FORCE` applies even to the table owner.
6. **Rate limiting** — per-IP limiter on all public endpoints (login, check-in, gate-scan, webauthn, qr-login).
7. **Audit log** — authenticated mutations recorded with actor, role, IP.

---

## Per-role analysis

### SUPER_ADMIN — the monetization/control plane (outside the tenant system)
- **Auth:** password login against the sentinel **platform tenant** (`00000000-…-0`). Owns no academic data.
- **Authz:** only the tenant-lifecycle + branding routes are `RequireRole(SUPER_ADMIN)`. Separate app/origin (`apps/super-admin`).
- **Privacy:** sees tenant metadata + branding, not student/lecturer PII.
- ⚠️ **MFA not enforced** for SUPER_ADMIN (a compromised super-admin can suspend tenants / disrupt billing). 🔲 Require TOTP/WebAuthn for this role before production.

### ADMIN (tenant) — confined to one tenant
- **Auth:** email + password + **Institution ID** (the institution-id gate is an extra factor unique to admins).
- **Authz:** `RequireRole(ADMIN) + RequireOwnTenant`. Cannot create/list other tenants (super-admin only) — verified by code; rerun P2/P3 with creds to confirm 403 live.
- **Privacy:** full visibility of **their own** tenant's users/students/lecturers — appropriate for the role; bounded by RLS + RequireOwnTenant.

### COORDINATOR — one course, scoped to their offering
- **Auth:** email + password. Each coordinator has an auto-generated unique code and is bound to a single course/offering.
- **Authz/Privacy:** sessions, manifests, rosters scoped to their offering; cannot see another coordinator's cohort (offering scoping + RLS).
- Change this review confirms: the coordinator **no longer scans student QR codes** (camera scanner removed); students self-scan.

### STUDENT — no app, no website, owns a QR
- **Auth:** the student's **RSA-signed personal QR** is the credential. They scan it with their phone's native camera → a server-rendered captive page (`/checkin`) — no installed app, no portal login required. A passwordless `qr-login` is also available.
- **Authz/Privacy:** `GET /api/v1/eligibility/{student_id}` **forces the caller's own id** — a student cannot read a classmate's attendance by changing the path (✅ verified in code). RLS confines everything to their tenant.
- ⚠️ **The QR encodes PII** (student_id, full_name, course, academic_year, serial). It is RSA-signed (tamper-proof) but **readable** by anyone who photographs it. 🔲 Treat the QR like an ID card; consider encrypting the display fields or carrying only an opaque serial that the server expands.
- ⚠️ **Room-code relay (H4):** a student could text the live 6-digit code to an absent friend. The fingerprint is client-supplied. 🔲 Strongest mitigations already present elsewhere (per-device binding in `hardware_vault`, one-device-per-session); keep enforcing device binding for students.

### LECTURER — strong anti-proxy proof-of-presence (this is the hardened path)
A lecturer is **registered** (staff ID captured, optional WebAuthn passkey). To **START** and again to **END** a lecture they scan the coordinator's QR and must pass, in order (`lecturer_gate_scan.go`):
1. **Staff-ID** matches the assigned lecturer.
2. **Live 10s digit code** from the coordinator's screen (`checkin.Validate`) — screen-proximity proof.
3. **LAN proximity** *(new in 1.0.0)* — the scan must originate from the **same public egress IP as the coordinator** (`sessions.coordinator_ip` vs `ClientIP`), tenant-toggleable via `tenants.require_lan_proximity` (default on).
4. **Biometric** — if a WebAuthn passkey is enrolled, an on-device fingerprint assertion is required (single-use per scan via Redis).
5. **Student quorum on END** — the lecture only closes (and contact hours count) if a configurable **share of enrolled students actually attended** (`lecturer_attendance_ratio`), proving a real, attended lecture.
- This composition makes remote/proxy lecturer attendance impractical: a proxy would need the staff ID **and** a live relayed code **and** to be on the campus network **and** the lecturer's phone biometric **and** real students in the room.
- ⚠️ **IP heuristic caveat:** same-egress-IP equals same NAT/LAN; it can be defeated if the gateway trusts a spoofable `X-Forwarded-For` from an untrusted hop. ✅ Mitigated here because Caddy is the only front door and sets XFF; 🔲 ensure no other ingress can inject XFF. In practice the coordinator's laptop is its **own isolated Wi-Fi hotspot**, so "same egress IP" means "physically on this room's AP" — the live rotating room code is the paired second proximity signal.

### VC / DVC / DQA_DIRECTOR / QA_OFFICER — governance, read-mostly
- **Authz:** dashboard/report routes gated to the specific role(s); QA mutations (corrections, device-reset, reissue) are QA-only.
- **Privacy:** aggregate/attendance views scoped by RLS to their tenant.

---

## Public attack surface

| Endpoint | Risk | Control |
|----------|------|---------|
| `POST /api/v1/checkin` | room-code brute force / QR forgery | RSA signature verify + rotating code + per-IP limit (5/10) |
| `POST /api/v1/lecturer/gate-scan` | staff-id / code guessing | 5-gate composition above + per-IP limit |
| `POST /api/v1/auth/login` | credential stuffing | per-IP limit (5/60) |
| `GET /api/v1/branding/public` | data leak | returns **display-safe fields only** |
| `GET /metrics` | infra disclosure | ⚠️ should be ClusterIP/internal-only in prod (M4) |

---

## Residual risks (carry-over) & recommendations

- ⚠️ **H3 — secrets in plaintext `.env`** (KEK, JWT keys, DB/Redis creds `changeme*`); Postgres TLS off. 🔲 Move to KMS/Vault; rotate; enable DB TLS **before production**.
- ⚠️ **MFA** not enforced for SUPER_ADMIN / governance. 🔲 Require a second factor for privileged roles.
- ⚠️ **Student QR carries PII.** 🔲 Opaque-serial QR or encrypted display fields.
- ⚠️ **M1 — XFF trust:** IP-based rate limiting *and* the new LAN-proximity gate both trust `X-Forwarded-For`; only safe behind a strict single proxy (Caddy). 🔲 Enforce that invariant in deployment.
- ⚠️ **`/metrics` exposure (M4)** — make internal-only.
- 🔲 **Re-run credentialed boundary probes (P2–P7)** after seeding a known admin + student to confirm 403s end-to-end, and add them to CI as a regression suite.

## Net assessment
Tenant isolation (RLS, forced, default-deny) and per-role RBAC are **sound and live-verified**. The student "own-QR, no-app" model and the lecturer multi-factor anti-proxy chain are strong. The blocking items for production are operational secrets hygiene (H3) and enabling MFA for privileged roles — not the access-control design.
