# crypto-core — QAAT coordinator app crypto (Phase 0, VERIFIED)

Pure JVM/JCA Kotlin (no Android deps) porting the coordinator's security primitives so the native
Android app produces output the **existing QAAT backend accepts unchanged**. This module is the
de-risking foundation of the native rewrite — the one part that *had* to be proven byte-compatible
before any Android work.

## What it does
- `VaultCrypto` — HKDF-SHA256 device-key derivation (from the server-issued binding secret),
  AES-256-GCM `base64(iv‖ct‖tag)`, HMAC-SHA256, SHA-256, keyed student-id hash. Mirrors
  `apps/coordinator-pwa/src/crypto/vault-crypto.ts` and the Go `services/sync-receiver/internal/crypto/vault.go`.
- `Sealer` — seals a closed session into the package the sync-receiver expects (encrypted payload +
  HMAC + SHA-256 checksum + chunk count). Mirrors `sync/sealer.ts`.
- `QrVerify` — RSA-2048 `SHA256withRSA` student-QR signature verify + the exact canonical body
  (`JSON.stringify(payload)` field order). Mirrors `qr-generator/src/crypto/rsa-keys.ts`.

## Proven (verifiable on this repo, no Android device)
`./verify.sh` reproduces all three, against the **running stack**:
1. **Seal → live `sync-receiver` accepts (`SYNCED`).** A Kotlin-sealed package, using a real
   coordinator's device key, passed checksum + HMAC + AES-GCM decrypt through the live
   `/api/v1/sync/{init,chunk,complete}` endpoints. Also decrypted directly by the receiver's own
   crypto package (`cmd/parity`).
2. **RSA QR verify + canonical body** match a Node-signed QR.
3. **Roster hash** (`HMAC-SHA256(student_hash_key, reg-no)`) matches the manifest scheme.

## Run the proof
```bash
# kotlinc's version parser crashes on a JDK whose version ends in "-ea" (e.g. 25.0.4-ea),
# so run it under a RELEASE JDK <= 21:
export JAVA_HOME=/path/to/jdk21
export KOTLINC=/path/to/kotlinc/bin/kotlinc   # kotlinc 2.0.21 verified
./verify.sh
```
(The running QAAT stack + Docker must be up; the script talks to `infra-qr-generator-1` and the
gateway/sync-receiver.)

## In the Android app
This module becomes a Gradle sub-module of `apps/coordinator-android/`; the Ktor server calls
`Sealer.seal(...)` on session close and `QrVerify.verify(...)` / `VaultCrypto.hmacHex(...)` in the
`POST /submit` check-in path. The binding secret is fetched at login (same as the PWA's
`initDeviceKey`) and kept in the Android Keystore.
