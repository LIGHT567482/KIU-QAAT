**How the system works (and why attendance is fully offline)**
Setup (online, done ahead of time). A super‑admin creates a university (tenant, isolated by Postgres RLS). The tenant admin builds the curriculum — a course (level‑independent), then levels inside it, each with its own year×semester unit roadmap — and creates cohorts that can be applied across all courses at once. People are registered leanly: students by registration number only, lecturers by staff ID. Each gets a permanent signed QR; an email is optional and used solely to email them that QR.

Taking attendance — this is the offline part. The coordinator's laptop is the room's Wi‑Fi hotspot, the LAN server, and the database, all at once. There is no internet in the loop:

The coordinator opens the session on the laptop → it starts a room code that rotates every few seconds and stamps its own LAN IP on the session.
The lecturer (phone on the hotspot) passes a start gate: scan the coordinator's QR + staff ID + live code + on‑LAN (+ fingerprint if enrolled) → proves a real lecture is happening.
Students join the hotspot, scan their own QR (passwordless login), and enter the room code. The laptop records PRESENT only if all hold: session active & in window, room code valid, on the coordinator's LAN (NOT_SAME_NETWORK otherwise), and one‑device‑one‑person (DEVICE_ALREADY_USED otherwise). Every accepted log is written to the laptop's database the instant it's accepted — offline.
If the coordinator is absent, they can pre‑authorise an own‑cohort student with a one‑day code to run that session as their deputy.
Sync (the only step needing internet). On close, the session is sealed into an AES‑256‑GCM package with a device‑bound HMAC‑SHA256 and a SHA‑256 checksum, queued in an offline outbox, and later uploaded atomically (all‑or‑nothing, chunked, retries until acknowledged) to the central SaaS database — which drives exam‑eligibility (≥75% floor) and the QA/DQA/VC dashboards. Separately, students check their own % any time via the passwordless reg‑no portal.

So the room never depends on connectivity: the laptop is the network and the server, attendance is captured locally, and only the sealed result travels once a link is available.








The honest reality (drives everything below)
The bottleneck is the Wi-Fi radio, not the server. The Go gateway + Postgres easily handle 1000 check-ins; a single access point cannot hold 1000 associated phones.
iPhone Personal Hotspot ≈ 5 clients · Android hotspot ≈ 10 (radio-limited) · Linux laptop hotspot (NetworkManager) ≈ 20–40 comfortably, ~60–100 with a good external AC/AX adapter but with heavy airtime contention. Not 1000 on one AP.
Phone vs laptop: a phone can run the coordinator PWA (open/close, show the room code) and be a small hotspot, but it cannot run the offline server/DB and cannot deauth clients. Fully-offline + auto-disconnect ⇒ the hub must be a Linux laptop (the documented nmcli hotspot at 10.42.0.1). This distinction must be stated honestly in the docs.
