# QAAT — Attendance Flow (current)

**Topology:** the **coordinator's hub** — a **Linux laptop** or a **native Android app** on a phone — is the room's **Wi‑Fi hotspot** *and* the **LAN server + hub** (gateway/embedded server + tenant DB, all offline). **One AP ≈ one classroom:** a hotspot holds ~10 phones (stock Android) to ~20–40 (laptop), so students **rotate** — each turns Wi‑Fi off once marked present to free a slot. Students' and the lecturer's phones join that hotspot and submit logs to it. Every log is stored on the hub the instant it's accepted, then **atomically synced** to a central server when one is online. Multi‑tenant SaaS: each tenant is reached at its own address; QR/links + the WebAuthn RP follow that host.

```mermaid
flowchart TD
    %% ---------- Provisioning ----------
    SA[Super-Admin] -->|registers| TEN[Tenant + Admin]
    TEN --> ADM[Admin: coordinators, lecturers,\nstudents+QRs, courses, units, offerings,\nstudy sessions, daily window]

    %% ---------- Coordinator hub ----------
    subgraph HUB["📡 Coordinator's laptop = Wi-Fi hotspot + LAN server + DB hub (OFFLINE)"]
      COOR[Coordinator logs in] --> OPEN{Open session}
      STB[Standby deputy: own-cohort student\n+ one-day code, if coordinator absent] -.-> OPEN
      OPEN -->|one open session\nper coordinator\n+ inside daily window| ACT[ACTIVE session:\nlive rotating code · lecturer QR · live roster]
      OPEN -->|else| BLK[Blocked: end current /\nwindow closed]
      DB[(attendance_logs\n+ lecturer logs\nstored on the hub the instant they're accepted)]
    end

    %% ---------- Student Path 1 ----------
    subgraph STU["👩‍🎓 Student phone (on the hotspot)"]
      S1[Scan OWN QR] --> S2[Portal opens → passwordless QR-login]
      S2 --> S3[Enter live room code]
    end
    S3 --> SC{Student check-in gates}
    SC -->|1. session ACTIVE & in window| SCa
    SCa[2. room code valid - proximity] --> SCb
    SCb{3. on coordinator's LAN? - network proximity} -->|no| RJ1[REJECT NOT_SAME_NETWORK]
    SCb -->|yes| SCc
    SCc{4. device already used this session?} -->|yes, other person| RJ2[REJECT DEVICE_ALREADY_USED]
    SCc -->|no| SCd
    SCd{5. student already present?} -->|yes| OKi[PRESENT - idempotent]
    SCd -->|no| REC1[Record PRESENT\none device · one person\n📴 “turn Wi-Fi OFF now” → frees a hotspot slot] --> DB

    %% ---------- Lecturer ----------
    subgraph LEC["👨‍🏫 Lecturer phone (on the hotspot)"]
      L1[Scan COORDINATOR's QR] --> L2[Captive portal: staff ID + live code + fingerprint]
    end
    L2 --> LC{Lecturer gate}
    LC -->|1. staff ID matches assigned| LCa
    LCa[2. live code valid] --> LCb
    LCb{3. on coordinator's LAN?} -->|no| RJ3[REJECT not_same_network]
    LCb -->|yes| LCc
    LCc{4. fingerprint - WebAuthn passkey, if enrolled} -->|fail| RJ4[REJECT biometric_required]
    LCc -->|ok| LCd{START or END?}
    LCd -->|START| REC2[Record lecture STARTED] --> DB
    LCd -->|END + student quorum met| REC3[Record ENDED + contact hours] --> DB

    %% ---------- Sync ----------
    DB -->|seal batch · all-or-nothing\nretry until ack| SYNC[[Atomic sync to central server\nwhen online]]
    SYNC --> CENTRAL[(Central SaaS DB)]

    classDef reject fill:#fde8e8,stroke:#b91c1c,color:#7f1d1d;
    classDef ok fill:#e7f7ec,stroke:#16a34a,color:#14532d;
    class RJ1,RJ2,RJ3,RJ4 reject;
    class REC1,REC2,REC3,OKi ok;
```

## The proof factors (anti‑proxy / anti‑ghost)
| Who | Identity | Presence | One‑per‑session |
|---|---|---|---|
| **Student** | personal signed QR → account | live room code **+** on the coordinator's Wi‑Fi | one **person** (account) **and** one **device** |
| **Lecturer** | staff ID **+** phone fingerprint (WebAuthn, if enrolled) | live room code **+** on the coordinator's Wi‑Fi | one assigned lecturer; END needs student quorum |

## Storage & sync
Every accepted log is written to the **coordinator's hub** (laptop DB) immediately and durably — **while completely offline**. On close, the session is sealed into an **AES‑256‑GCM** package, authenticated with a **device‑bound HMAC‑SHA256** and a **SHA‑256 checksum**, and queued in the PWA's offline outbox. The hub then **atomically** syncs the sealed package to the central server (all‑or‑nothing, chunked, retries until acknowledged) — on the offline hotspot it simply waits until the internet is reachable. Nothing is lost; nothing partial is ever committed centrally.

## Viewing progress (separate from check‑in)
Taking attendance uses the student's **QR** (above). To *see* their own attendance % and exam eligibility, a student opens their institution's **passwordless reg‑no portal** and types only their registration number — no account, no login. A reg‑no only ever resolves within its own institution.
