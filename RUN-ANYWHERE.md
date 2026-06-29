# Run QAAT on any laptop (quick, offline)

The whole system is self-contained — no cloud. To move it to a different laptop
and run it **the same way**:

## 1. Copy the folder
Copy this entire `QAAT/` folder to the other laptop (USB, scp…). It already
contains everything: source, auth RSA keys (`keys/`), settings (`infra/.env`),
TLS cert (`infra/certs/`), built dashboards (`apps/*/dist`), and all DB
migrations + the platform super-admin seed.

## 2. Install prerequisites (once)
- **Docker** + **Docker Compose** (Docker Desktop on Win/macOS; on Linux:
  `docker.io` + the compose plugin).
- For the offline **phone hotspot** (step 4): a **Linux** laptop with
  NetworkManager and an AP-capable Wi-Fi card (most are).

## 3. One command
```bash
./setup.sh
```
Builds the images and starts everything. **On first boot the database builds its
entire schema + the platform super-admin automatically.** It then prints the URLs
and this default login:

- Super-Admin: `superadmin@qaat.platform` / `Super1234!`
  → log in, register your institution (tenant) + its admin; the admin adds
  coordinators, lecturers, students, courses.

On the laptop: `https://localhost:3000`–`3003`.

## 4. Phones, fully offline, fixed address
```bash
echo 'address=/qaat.local/10.42.0.1' | sudo tee /etc/NetworkManager/dnsmasq-shared.d/qaat.conf
sudo nmcli device wifi hotspot ifname wlan0 ssid QAAT-Attendance password qaat12345
```
Phones join Wi-Fi **QAAT-Attendance** (`qaat12345`) → open `https://10.42.0.1:3000`.
The laptop is always `10.42.0.1` on its own hotspot, so it works in any location
with no internet and QR codes never break. Stop: `sudo nmcli connection down Hotspot`.
(macOS/Windows have no `nmcli` — use the OS "share Wi‑Fi" or a small travel router;
the app is identical, only this step differs.)

## Coordinator hub: laptop vs phone, and capacity
- **Fully-offline hub today = this Linux laptop** (it runs the server + DB + hotspot). A phone can
  run the coordinator *PWA* (open/close, show the code) but **cannot** run this stack — for a
  phone-as-hub see the native Android app plan in
  [apps/coordinator-pwa/ANDROID.md](apps/coordinator-pwa/ANDROID.md).
- **One access point ≈ one classroom.** A hotspot holds ~**10** phones on a stock Android, ~**20–40**
  on this laptop. Students **rotate**: the check-in screen tells each student to turn Wi-Fi **off**
  the moment they see ✓, freeing a slot for the next. Big groups take several rotation cycles
  (minutes). For more capacity: add an external Wi-Fi router/AP in the room, run several
  coordinators/APs in parallel, or put this laptop's server on campus Wi-Fi. See
  [docs/DEVICE_TESTING.md](docs/DEVICE_TESTING.md) to find your hardware's real ceiling.

## Notes
- Self-signed cert → on each device **Advanced ▸ Proceed** once per port (or trust
  `infra/certs/qaat.crt`).
- Changed frontend code? Rebuild that app first: `cd apps/<app> && pnpm install &&
  pnpm exec vite build` (build WITHOUT `VITE_API_URL` — dashboards detect the host
  at runtime).
- Fresh empty DB: `docker compose -f infra/docker-compose.yml down -v` then `./setup.sh`.
