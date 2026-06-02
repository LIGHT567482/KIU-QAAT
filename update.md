# QAAT — Test, Vulnerability & Production-Readiness Report

_Last updated: 2026-06-02. This supersedes the 2026-06-01 log below the line in §9.
Scope of this pass: actually build/test what is runnable, examine the security
surface across **all** services (not just QR/sync/PWA), empirically probe the
multi-tenant isolation, and produce a risk analysis. Reflects the online
check-in pivot (rotating room code + HTTPS, migrations 007/008)._

## TL;DR — Production ready? **NO.**

One **critical** finding made the platform's central security promise — tenant
isolation — non-functional at runtime, and I **proved it empirically** (read
*and* wrote another tenant's data while scoped to a different tenant). There are
two further **high** issues (forgeable signed-QR / session credentials via
unauthenticated internal services; plaintext production secrets) and a
structural **anti-proxy weakness** introduced by the pivot. CI is "green" for
the wrong reasons. Details and a ranked risk matrix below.

> **Update — fixes applied (2026-06-02, second pass).** The critical RLS bypass
> (C1), the two highs H1/H2, and the build/CI gaps (L1/L2/L3) are now fixed and
> verified. See **§0 Fixes applied** immediately below. Production readiness is
> still **NO** until the residual items there (secret management, end-to-end sync
> round trip, the H4 anti-proxy design, the load/e2e suites) are closed — but the
> showstopper is resolved.

---

## 0. Fixes applied (2026-06-02)

All services still **build / vet / typecheck clean**; the checkin unit tests and
auth-service tests pass; and I re-ran the live Postgres probe to confirm the fix.

### C1 — RLS now actually enforced ✅ (re-proven)
- New migration **`db/migrations/009_force_rls_and_app_role.sql`**: hardens
  `qaat_app` to `NOSUPERUSER NOBYPASSRLS`, applies **`FORCE ROW LEVEL SECURITY`**
  to all 14 tenant tables, (re)grants the privileges qaat_app needs on objects
  from migrations 005/007/008, and re-revokes DELETE/UPDATE on the append-only
  tables.
- The **data plane now connects as `qaat_app`** (non-superuser): api-gateway
  data plane, qr-generator, session-manager. `auth-service` and `sync-receiver`
  intentionally remain on the privileged role (identity/sync authorities that
  scope tenant themselves — auth by PK/explicit filter, sync from the verified
  JWT). The gateway's **ADMIN-gated, cross-tenant handlers use a separate
  privileged `ADMIN_DB_URL` pool** (`admin.go` via `router.New`); the tenant
  data-plane pool is `qaat_app`.
- **Live re-proof** (migrations 001–009, connected as `qaat_app`, scoped to
  Tenant A):
  ```
  qaat_app: super=false  bypassrls=false      venues: enabled=true forced=true
  SELECT … FROM venues            -> only Hall A           (Tenant B hidden)   ✅
  UPDATE  … WHERE tenant=B         -> UPDATE 0  (rejected)  Hall B untouched    ✅
  DELETE  FROM attendance_logs     -> ERROR: permission denied                 ✅
  ```
  Compare to the pre-fix run in §1, where both the cross-tenant read and write
  succeeded.

### H1 — internal services verify the JWT themselves ✅
- **qr-generator**: new `src/middleware/auth.ts` verifies the RS256 token with
  `node:crypto` (no new dependency), checks iss/aud/exp, and pins `X-Tenant-ID`
  from the verified claim. All DB work now runs through `src/db.ts withTenant()`,
  which sets the tenant GUC on a dedicated client (required now that it connects
  as `qaat_app` under RLS). Wired in `server.ts`.
- **session-manager**: new `internal/auth` package (mirrors sync-receiver),
  middleware wired in `main.go` over `warden-link` and `clearance-token`. Tenant
  is taken from the verified claim, and the venue/eligibility queries are scoped
  via a new parameterised `setTenantConn`.
- Both services were also added to the **deny-by-default trust posture**: tenant
  comes from the token, never a header.

### H2 — gateway strips inbound identity headers ✅
- `proxy.New` now `Del`s `X-Tenant-ID`/`X-User-ID`/`X-Role` before setting the
  gateway-derived values, so a client can't smuggle identity headers downstream.

### Bonus security fixes folded in
- **SQL injection + inert `SET LOCAL` in `session-manager/clearance.go`** (the
  same class as the gateway's earlier C3) — replaced with the parameterised
  session-scoped GUC.
- **qr-generator all-zero KEK fallback removed** from compose (`KEY_ENCRYPTION_KEY`
  is now required, matching the code's fail-closed loader).

### L1 — admin-dashboards builds ✅
- Added `src/vite-env.d.ts` (`vite/client` types → `import.meta.env` typed) and
  flattened the `useApi` result type so destructuring keeps `data`; four pages
  got one-line `?? []` / `&& data` guards. `tsc --noEmit` is now clean.

### L2/L3 — CI is a real gate now ✅
- Both CI migration steps apply **all** migrations `001–009` (were 001–004).
- `db-migrations` job now: verifies RLS is **ENABLED *and* FORCED** on every
  tenant table, asserts `qaat_app` is `NOSUPERUSER+NOBYPASSRLS`, loads the seeds,
  and **runs `tests/security` as `qaat_app`** — so the isolation suite can no
  longer pass vacuously.
- `tests/security` fixed to set the tenant via `set_config(...,false)` instead of
  the discarded `SET LOCAL`.
- (Note: admin-dashboards typecheck *was* already a CI job — so CI was actually
  RED on it until L1; the report's earlier "CI only typechecks coordinator-pwa"
  was imprecise.)

### Wiring
- `infra/docker-compose.yml`, `.env.example`, `infra/k8s/{api-gateway,qr-generator,secrets}.yaml`
  updated for the `qaat_app` data-plane DSN (`app-db-url`), the privileged
  `ADMIN_DB_URL`/`db-url`, and the RSA public-key mount + JWT env for qr-generator.

### Residual / NOT fixed in this pass (still required for prod)
- **H4 (anti-proxy design):** unchanged — a relayed room code still lets a remote
  student check in. This is a design decision, not a code bug; needs product
  input (shorter window / server-observed signals / coordinator-side ghost check).
- **H3 (secrets):** code no longer ships an all-zero KEK and docs/compose/k8s now
  separate roles, but **rotating the real KEK/VAPID in `.env`, moving to KMS/Vault,
  replacing every `changeme*`, and enabling Postgres TLS remain ops tasks.**
- **M1/M2/M3/M4** (XFF-spoofable public limiter, unbounded per-pod limiters, TOTP
  keyed by password hash + dead backup codes, public monitoring LB) — not yet
  addressed.
- **M5:** the PWA-seal → sync-receiver decrypt round trip is still unproven
  end-to-end (no integration test); k6 load + Playwright e2e were not run here.
- **Deploy gap:** there is no k8s manifest or compose service for
  `session-manager` (and none for it in compose) — it must be added before its
  hardening matters in those environments.

---

---

## 1. What I actually ran (test results)

| Area | Command | Result |
|---|---|---|
| Go build + `go vet` | all 5 modules | ✅ clean (auth-service, api-gateway, sync-receiver, session-manager, tests/security) |
| Go unit tests — auth-service | `go test ./...` | ✅ `ok` (1 package has tests; rest "no test files") |
| Go unit tests — api-gateway `checkin` | `go test ./internal/checkin -v` | ✅ **10/10 pass** (RSA sig verify, tamper rejection, room-code rotation/skew) |
| TS typecheck — coordinator-pwa | `tsc --noEmit` | ✅ clean |
| TS typecheck — student-portal | `tsc --noEmit` | ✅ clean |
| TS typecheck — qr-generator | `tsc --noEmit` | ✅ clean |
| TS typecheck — notification-service | `tsc --noEmit` | ✅ clean |
| TS typecheck — **admin-dashboards** | `tsc --noEmit` | ❌ **11 errors — app does not build** (see L1) |
| RLS isolation suite | `tests/security` | ⚠️ Not meaningfully runnable (see C1 / L2); I ran a **manual probe instead** |
| k6 load (300 scans/min, 10k sync) | — | ⛔ Not run: `k6` not installed on this host |
| Playwright e2e | — | ⛔ Not run: `tests/e2e/node_modules` absent; needs the full stack up |

**Coverage reality:** outside `internal/checkin` and one auth-service test file,
there are essentially **no unit tests** — almost every Go package reports "no
test files." Behavioural correctness of handlers, sync, and crypto is unproven.

### Empirical RLS probe (the important one)
I started `postgres:16`, applied the **real** migrations `001–008` as the
configured `qaat` user, then:

```
role = qaat   is_superuser = t
students_extended: rls_enabled = t   rls_forced = f
-- scoped to Tenant A:
SELECT set_config('app.current_tenant','<TENANT-A>', false);
SELECT * FROM venues;        -->  Hall A (tenant A)  AND  Hall B (tenant B)   ← cross-tenant READ
UPDATE venues SET name='HIJACKED' WHERE tenant_id = '<TENANT-B>';  -->  UPDATE 1   ← cross-tenant WRITE
```

Both succeeded. **Tenant isolation does not exist at runtime.** See C1.

---

## 2. CRITICAL

### C1 — Multi-tenant RLS is completely bypassed in production config
**The single blocker.** The RLS design in `003_rls_policies.sql` / `007` is
correct *on paper*, but:

- Every service connects with `DB_URL=postgres://qaat:…` — the **`qaat`
  superuser** (`POSTGRES_USER=qaat` in `infra/docker-compose.yml`, and the same
  in `.github/workflows/ci.yml`). Migrations are also applied as `qaat`, so
  `qaat` **owns** every table.
- **PostgreSQL superusers and table owners bypass RLS** unless the table is set
  to `FORCE ROW LEVEL SECURITY`. There is **no `FORCE ROW LEVEL SECURITY`
  anywhere** in `db/`.
- The limited-privilege role the policies were designed for — `qaat_app`
  (created in `003`, password `changeme_app`) — is **never used by any service,
  compose file, or k8s secret** (`grep qaat_app` across the repo: only the
  migration that creates it).

**Consequences (all proven or deterministic):**
1. Cross-tenant **read and write** succeed (proven above). A valid JWT for
   Tenant A can touch Tenant B's data through any query path that doesn't *also*
   hand-filter `tenant_id` — and many handlers rely on RLS as the only guard.
2. The **append-only / immutable** guarantees are void: the RESTRICTIVE
   `no_delete_attendance` / `no_update_attendance` / `no_*_audit` policies and
   the `REVOKE DELETE` are all ignored for a superuser. `attendance_logs` and
   `admin_audit_log` are fully mutable at runtime.

**Fix:** create and use `qaat_app` (non-superuser, non-owner, `NOSUPERUSER`,
no `BYPASSRLS`) as the application DB role; point every `DB_URL` at it; and add
`ALTER TABLE … FORCE ROW LEVEL SECURITY` to every tenant-scoped table as
defence-in-depth (so even an owner connection is constrained). Then re-run a
seeded isolation probe to confirm reads return only own-tenant rows and the
cross-tenant `UPDATE` is rejected.

---

## 3. HIGH

### H1 — Forgeable signed-QR & session credentials via unauthenticated internal services
- `qr-generator` derives the tenant **solely** from the `X-Tenant-ID` request
  header and performs **no JWT verification** of its own
  (`services/qr-generator/src/handlers/generate.ts:16`). `session-manager`
  likewise reads `X-Tenant-ID` from the header (`clearance.go:57`) with no token
  check.
- `sync-receiver` *was* hardened against exactly this (`internal/auth/jwt.go`
  re-verifies the RS256 JWT, "must not be exploitable if reachable directly …
  SSRF, in-cluster lateral movement, a misapplied NetworkPolicy"). The same
  hardening was **not** applied to qr-generator or session-manager.
- There are **no `NetworkPolicy` manifests** in `infra/k8s/`. So any pod/SSRF
  primitive that can reach these services on the cluster network can set
  `X-Tenant-ID: <victim>` and **mint validly-signed QR codes** (the identity
  credential the whole check-in trusts) or **issue clearance/warden tokens** for
  any tenant — no JWT required.

**Fix:** verify the JWT inside qr-generator and session-manager (mirror
sync-receiver), derive tenant only from the verified claim, and add deny-by-
default NetworkPolicies so only the gateway can reach internal services.

### H2 — Gateway does not strip inbound identity headers
`proxy.New` (`internal/proxy/proxy.go:26`) only `Set`s `X-Tenant-ID` /
`X-User-ID` / `X-Role` when the context value is non-empty; it never `Del`s an
attacker-supplied copy first. With JWTAuth those values are normally non-empty,
but the proxy should explicitly `Del` all three before setting them. This is the
defence-in-depth layer that, combined with H1's missing isolation, turns a
header into authority.

### H3 — Real secrets in plaintext; default `changeme` credentials
- `/.env` on disk holds a **real 32-byte `KEY_ENCRYPTION_KEY`** (distinct from
  the `change…` placeholder in `.env.example`) and a **real `VAPID_PRIVATE_KEY`**.
  (`.env` is gitignored and this dir is not a git repo, so nothing is committed —
  but live secrets sitting in a plaintext dotfile is the wrong handling.)
- `DB_PASSWORD` / `REDIS_PASSWORD` are still `changeme*`; compose defaults to
  `changeme_db`; the `qaat_app` role ships with `changeme_app`; Postgres runs
  `sslmode=disable`.
- No secrets manager / KMS / rotation is wired (DEPLOY/PILOT note this is
  unverified). `KEY_ENCRYPTION_KEY` is now mandatory and fails-closed (good), but
  it must come from Vault/KMS, not a file.

### H4 — The pivot weakens the core anti-proxy guarantee (design risk)
The product's reason to exist is "eliminate proxy attendance and ghost
lectures." Post-pivot, per-student proximity is a **6-digit room code** shown on
the projector (`internal/checkin/roomcode.go`, 15 s step, ±1 skew ≈ 45 s window).
A student physically present can **screenshot/text the code to an absent friend**,
who submits their own signed QR + that code within the window → marked `PRESENT`.
The device-fingerprint bind (validator steps 6/8) is the only remaining barrier,
and the fingerprint is a **client-supplied string** (`req.Fingerprint`), so it is
spoofable and, for a never-seen device, simply bound on first use. Net: **remote
proxy attendance is achievable.** This is the documented trade-off of moving off
BLE, but it must be treated as an open security risk, not a solved one. Consider
shortening the window, binding to server-observed signals, or a
coordinator-side ghost-lecture check.

---

## 4. MEDIUM

- **M1 — Room-code brute-force throttle is spoofable.** The public `/checkin`
  limiter (`PublicIPRateLimit(5,10)`) keys on `chi` `RealIP`, which trusts
  `X-Forwarded-For`/`X-Real-IP`. If the gateway is ever reachable not strictly
  behind a header-overwriting proxy, an attacker rotates XFF to defeat per-IP
  limiting and can brute the 6-digit code (10⁶ space, ~45 s window) — still needs
  a valid signed QR, but the only rate barrier is bypassable.
- **M2 — Rate limiters: unbounded + per-pod.** `ratelimit.go` stores a
  `*rate.Limiter` per coordinator/IP in maps that are **never evicted** (memory
  growth / DoS vector), and they are **in-process**, so with N gateway replicas
  the effective limit is N× the configured value. Use a shared (Redis) limiter
  with TTL eviction.
- **M3 — TOTP secret keyed by the password hash.** `EncryptSecret(secret,
  user.PasswordHash)` (`auth_handler.go:283`) means a **password change orphans
  the MFA secret and backup codes**, and the "key" lives in the same DB row as
  the ciphertext (defence-in-depth only). Also: backup codes are generated at
  enroll, **discarded and regenerated** in `VerifyMFA`, and there is **no
  login path that accepts a backup code** — the feature is dead.
- **M4 — Monitoring exposed publicly.** `infra/k8s/monitoring.yaml` uses
  `type: LoadBalancer` (alongside api-gateway). Prometheus/Grafana should be
  ClusterIP behind auth, not a public LB.
- **M5 — Sync crypto pipeline still unverified end-to-end.** PWA seal → HKDF →
  AES-GCM/HMAC → `sync-receiver` verify+decrypt was matched by reading code, never
  exercised by a real round trip. No test covers it. Carried from the prior log;
  still open.

---

## 5. LOW / quality / CI

- **L1 — admin-dashboards does not type-check (won't build).** 11 `tsc` errors:
  missing `vite/client` types so `import.meta.env` is untyped (`lib/api.ts`,
  `Login.tsx`, …), and pages read `.data` off the `useQuery` discriminated union
  without narrowing on `status === 'ok'` (`AdminTenants.tsx`, `VCOverview.tsx`,
  etc.). Its `build` script is `tsc && vite build`, so **deploy fails**. CI never
  catches this because CI only typechecks coordinator-pwa.
- **L2 — CI is green for the wrong reasons (false confidence).**
  - The migration/test job applies only **`001–004`** — it never applies
    `005_tenant_rsa_keys`, `006_sync_package_hmac`, `007_integrity_and_scale`,
    `008_online_checkin`. Tests run against a schema missing `checkin_secret`,
    `tenant_rsa_keys`, the vector-clock and dedup indexes, etc.
  - CI loads **no seed data**, so the RLS isolation test's `countCross(... ) == 0`
    assertions pass **vacuously on empty tables**.
  - CI connects as the **`qaat` superuser**, so even a correct RLS test would be
    bypassed (see C1).
  - CI does not run `tests/security`, Playwright e2e, or k6 against a live,
    seeded stack.
- **L3 — RLS test is itself inert.** `tests/security/rls_isolation_test.go:40`
  uses `SET LOCAL app.current_tenant=…` inside a bare `conn.Exec` — the exact
  pattern the production `middleware/tenant.go` comment says pgx discards
  immediately (own implicit transaction). So the test sets nothing; it must use
  `SetTenantConn`/`set_config(..., false)` like production.
- **L4 — `sequence_number = MAX+1` then insert** (`checkin.go:197`): any insert
  error is mapped to `DUPLICATE_SCAN` (`:212`). The `uq_attendance_session_student`
  (008) and `uq_attendance_vector_clock` (007) indexes correctly prevent true
  double-counting, but a *different* failure (or a seq collision between two
  different students racing) would be mis-reported to a legitimate student as a
  duplicate. Verify under the 300-scan/min load test.
- **L5 — Eligibility % is stale by design** until session close / scheduled job
  (`checkin.go` deliberately skips the per-scan summary refresh — correct for
  performance, but dashboards must not be read as real-time).

---

## 6. What was already solid (verified by reading + the checkin tests)

- **JWT handling** (`api-gateway/middleware/jwt.go`, `sync-receiver/auth/jwt.go`,
  `auth-service/crypto/jwt.go`): RS256 enforced via `SigningMethodRSA` type check
  (no `alg=none`/HS256 confusion), issuer+audience validated, `jti` blacklist
  checked, **fails closed** on Redis error.
- **Login** (`auth_handler.go`): constant-time dummy bcrypt for unknown accounts
  (no user enumeration), account lockout, MFA gate for VC/DQA roles.
- **Master-key crypto** (`auth-service/crypto/masterkey.go`): AES-256-GCM with
  AAD binding, **fails closed** on missing/malformed KEK.
- **Check-in signature + room code** (`internal/checkin`): real RSA-SHA256 verify,
  tamper rejection, HMAC-TOTP room code with constant-time compare — 10/10 tests
  pass.
- **sync-receiver** re-verifies its own JWT and derives tenant from the claim
  (the right pattern — H1 is that the *other* two services don't follow it).
- RLS policies use `current_setting('app.current_tenant', true)` which is
  **fail-closed** when unset (NULL → no rows) — the design is right; it's the
  *connection role* (C1) that defeats it.

---

## 7. Risk analysis (likelihood × impact)

| ID | Risk | Likelihood | Impact | Rating |
|----|------|-----------|--------|--------|
| C1 | Cross-tenant data read/write; mutable "append-only" logs (superuser bypasses RLS) | High (default config) | Critical (data breach, audit fraud, regulatory) | 🔴 **Critical** |
| H1 | Forged signed QRs / session tokens via unauthenticated internal services (no NetworkPolicy) | Medium (needs in-cluster/SSRF foothold) | Critical (fabricated attendance at scale) | 🔴 **Critical/High** |
| H4 | Remote proxy attendance via relayed room code | High (trivial for motivated students) | High (defeats core value prop) | 🟠 **High** |
| H3 | Secret exposure / default creds | Medium | High | 🟠 **High** |
| H2 | Spoofed identity headers reach internal services | Low–Med (couples with H1) | High | 🟠 **High** |
| M5 | Sync decrypt pipeline fails in prod / silently drops records | Medium (untested) | High (lost attendance) | 🟠 **High** |
| M1 | Room-code brute force via XFF spoof | Low–Med | Medium | 🟡 **Medium** |
| M2 | Rate-limiter memory growth / per-pod under-limiting | Medium | Medium | 🟡 **Medium** |
| M3 | MFA breaks on password change; dead backup codes | Medium | Medium (lockout/support load) | 🟡 **Medium** |
| M4 | Public Prometheus/Grafana | Medium | Medium (info disclosure) | 🟡 **Medium** |
| L1 | admin-dashboards won't build | High | Medium (no admin UI ships) | 🟡 **Medium** |
| L2/L3 | False-green CI; isolation untested | High | High (no safety net) | 🟠 **High** (process) |

---

## 8. Go / No-Go checklist before pilot

**Must fix (Go-blockers):**
1. **C1** — app connects as non-superuser `qaat_app`; add `FORCE ROW LEVEL
   SECURITY`; re-prove isolation with a **seeded** probe.
2. **H1/H2** — JWT verification in qr-generator + session-manager; deny-by-
   default NetworkPolicies; gateway `Del`s inbound identity headers.
3. **H3** — secrets out of `.env` into KMS/Vault; rotate the exposed KEK & VAPID
   key; replace all `changeme*`; enable TLS to Postgres.
4. **L2/L3** — CI applies **all** migrations, loads seeds, runs `tests/security`
   + e2e against the seeded stack as `qaat_app`; fix the test's `SET LOCAL`.
5. **M5** — execute one real PWA-seal → sync-receiver decrypt round trip.
6. **L1** — make admin-dashboards type-check and build; add it to CI.

**Should fix before scale:** H4 (proximity hardening), M1, M2, M3, M4, L4.

**Suggested order:** C1 → L2/L3 (so the fix is provable in CI) → H1/H2 →
H3 → M5 → L1 → the rest. Add regression tests alongside each (cross-tenant read
denied; cross-tenant `UPDATE` rejected; QR with wrong tenant header rejected by
qr-generator).

---

## 9. Prior log (2026-06-01) — retained for history

The earlier security-fix pass (C1–C3 / H1–H5 of *that* numbering: the `await`ed
QR signature, serial-revocation, the `SetTenantConn` parameterisation, KEK
fail-closed, AES-GCM-at-rest, sync JWT auth, etc.) remains valid and is the
reason the items in §6 are solid. Note its numbering is independent of this
report's. Its §4 (offline PWA-LAN transport problem) is **superseded** by the
online check-in pivot now reflected in migrations 007/008 and `internal/checkin`.
The "RLS isolation must be re-proven" caveat from that log is now **closed with a
negative result**: re-proven, and it **fails** (C1).
</content>
