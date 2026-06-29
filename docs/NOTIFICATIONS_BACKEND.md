# Feature 5 — Admin ↔ Coordinator Notifications: backend spec (ready to implement)

Two-way messaging between admins (DQA / QA / VC / ADMIN) and coordinators: broadcasts, individual
messages, replies (threaded), system alerts, coordinator-initiated escalations, read receipts, and
scheduled delivery. Coordinators (native app) **poll** when online and queue replies offline.

> **Scope note:** this is the one coordinator-app feature that **touches existing apps** — it needs new
> gateway endpoints **and** an admin-dashboard "send" UI. Delivery is **poll-based** (no push infra):
> the app pulls the inbox every ~15 min when online and raises a local Android notification. Keep it
> additive — new tables + new routes; nothing existing changes.

## 1. Migration — `db/migrations/045_notifications.sql`
Mirror the RLS/grant pattern of recent migrations (e.g. `042_coordinator_delegations.sql`).

```sql
CREATE TYPE notification_type   AS ENUM ('BROADCAST','INDIVIDUAL','ALERT','RESPONSE','ESCALATION');
CREATE TYPE notification_scope  AS ENUM ('ALL','DEPARTMENT','COURSE','INDIVIDUAL');
CREATE TYPE notification_prio   AS ENUM ('NORMAL','URGENT');

CREATE TABLE notifications (
  notification_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id       UUID NOT NULL REFERENCES tenants(tenant_id) ON DELETE CASCADE,
  type            notification_type  NOT NULL,
  sender_id       VARCHAR(50) NOT NULL,         -- user_id, or 'SYSTEM'
  sender_role     VARCHAR(20) NOT NULL,
  recipient_id    VARCHAR(50),                  -- set for INDIVIDUAL/RESPONSE/ALERT; NULL for broadcasts
  scope           notification_scope NOT NULL DEFAULT 'INDIVIDUAL',
  scope_value     VARCHAR(100),                 -- department/course id when scope=DEPARTMENT/COURSE
  priority        notification_prio  NOT NULL DEFAULT 'NORMAL',
  subject         TEXT,
  message         TEXT NOT NULL,
  parent_id       UUID REFERENCES notifications(notification_id) ON DELETE CASCADE, -- threading
  context         JSONB,                        -- escalation payload (student_id, session_id, flags…)
  scheduled_for   TIMESTAMPTZ,                  -- NULL = deliver immediately
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Per-recipient delivery/read state (one row per coordinator a message targets).
CREATE TABLE notification_receipts (
  receipt_id      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id       UUID NOT NULL REFERENCES tenants(tenant_id) ON DELETE CASCADE,
  notification_id UUID NOT NULL REFERENCES notifications(notification_id) ON DELETE CASCADE,
  recipient_id    VARCHAR(50) NOT NULL,
  delivered_at    TIMESTAMPTZ,
  read_at         TIMESTAMPTZ,
  archived_at     TIMESTAMPTZ,
  UNIQUE (notification_id, recipient_id)
);
CREATE INDEX idx_notif_recipient ON notification_receipts (tenant_id, recipient_id, read_at);
CREATE INDEX idx_notif_created   ON notifications (tenant_id, created_at DESC);

-- RLS (mirror 042): tenant_isolation USING (tenant_id = current_setting('app.current_tenant')::uuid)
-- on both tables; ALTER TABLE … ENABLE + FORCE ROW LEVEL SECURITY; GRANT to qaat_app.
```

**Scope expansion at send time:** for a BROADCAST, the create handler resolves the matching coordinators
(scope ALL = all COORDINATOR users in the tenant; DEPARTMENT/COURSE = those whose offering matches
`scope_value`) and inserts one `notification_receipts` row per coordinator. INDIVIDUAL/RESPONSE/ALERT =
a single receipt for `recipient_id`. Coordinator lists are already derivable (see `AdminCoordinators`,
`course_offerings.coordinator_id`).

## 2. Endpoints (gateway — `internal/handlers/notifications.go` + routes in `router.go`)

**Admin plane** — `RequireRole(ADMIN, DQA_DIRECTOR, QA_OFFICER, VC) + RequireOwnTenant`:
| Method | Path | Body / purpose |
|---|---|---|
| POST | `/api/v1/notifications` | `{type, scope, scope_value?, recipient_id?, priority, subject, message, scheduled_for?}` → insert + expand receipts |
| GET  | `/api/v1/notifications/sent` | sender's sent list + aggregate counts (delivered / read / responded) |
| GET  | `/api/v1/notifications/{id}/thread` | original + threaded replies (by `parent_id`) |

**Coordinator plane** — `RequireRole(COORDINATOR)` (uses `GetUserID` from JWT — never trust a body id):
| Method | Path | Purpose |
|---|---|---|
| GET  | `/api/v1/notifications/inbox?since=<iso>` | this coordinator's deliverable messages (see query) + unread count |
| POST | `/api/v1/notifications/{id}/read` | stamp `read_at` on this coordinator's receipt |
| POST | `/api/v1/notifications/{id}/reply` | `{message}` → new `type=RESPONSE`, `parent_id=id`, `recipient_id=`original sender |
| POST | `/api/v1/notifications/{id}/archive` | stamp `archived_at` |
| POST | `/api/v1/escalations` | `{escalation_type, context, message}` → `type=ESCALATION` to DQA/QA with `context` JSON |

**Inbox query** (RLS-scoped; resolves broadcasts + individual, honours scheduling, hides archived):
```sql
SELECT n.*, r.read_at, r.delivered_at
FROM notification_receipts r JOIN notifications n ON n.notification_id = r.notification_id
WHERE r.recipient_id = $1                                  -- GetUserID()
  AND r.archived_at IS NULL
  AND COALESCE(n.scheduled_for, n.created_at) <= now()      -- scheduling, no cron needed
  AND ($2::timestamptz IS NULL OR n.created_at > $2)        -- 'since' cursor for the 15-min poll
ORDER BY n.created_at DESC;
```
Mark `delivered_at` on first fetch. Read-receipt aggregation for the admin "sent" view:
`SELECT count(*) FILTER (WHERE read_at IS NOT NULL) … GROUP BY notification_id`.

## 3. Native app side (additive)
- **Poll:** a WorkManager periodic job (~15 min, network-constrained) calls `/inbox?since=<lastSeen>`,
  stores rows in a new Room `notifications` table, raises a local Android notification for new URGENT
  ones (silenced during an ACTIVE session — queue, show on close).
- **Reply/escalate offline:** write to a local `notification_outbox`; flush on reconnect (reuse the
  sync outbox pattern in `SyncClient`). Escalation buttons on the absentee / sync-audit / session
  screens pre-fill `context` (student_id / session_id / flags) so the admin gets full context.
- **Read receipts:** POST `/read` when the coordinator opens a message (queued offline like replies).

## 4. Admin-dashboard side (the part that touches existing apps)
A "Notifications" panel in `apps/admin-dashboards`: compose (type/scope/priority/subject/message/
schedule), a sent list with Delivered/Read/Responded badges, and a thread view. This is the only piece
outside the coordinator app — implement it last / confirm before touching the dashboards.

## 5. Security
RLS tenant isolation on both tables (never the no-RLS admin pool for the coordinator plane). Coordinator
endpoints derive the actor from the **JWT** (`GetUserID`), so a coordinator can only read/ack their own
receipts and reply within their tenant. Validate `scope_value` against the tenant's real
departments/courses. Rate-limit `/escalations` per coordinator.

## 6. Verification
Unit: scope-expansion (ALL/DEPT/COURSE → correct receipts), inbox scheduling + `since` cursor, RLS
(coordinator A cannot read B's receipts), reply threading. E2E: admin POST broadcast → two coordinators'
inboxes show it → one reads (sent view shows 1/2 read) → one replies → admin thread shows the response.
