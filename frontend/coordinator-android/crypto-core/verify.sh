#!/usr/bin/env bash
# Phase 0 crypto-parity proof — reproduces, without an Android device, that the
# Kotlin crypto-core is byte-compatible with the QAAT Go/Node backend:
#   1. Kotlin seals a session package  → the REAL sync-receiver crypto decrypts it.
#   2. Kotlin verifies a Node-signed RSA-2048 QR + rebuilds the exact canonical body.
#   3. Kotlin HMAC-SHA256 student-id hash matches the manifest roster scheme.
#
# Requires: a JDK and a standalone `kotlinc`. NOTE: a JDK whose version string ends
# in "-ea" (e.g. the host's 25.0.4-ea) crashes kotlinc's version parser, so run
# kotlinc under a release JDK <= 21:
#   export JAVA_HOME=/path/to/jdk21 ; export KOTLINC=/path/to/kotlinc/bin/kotlinc
# (kotlinc 2.0.21 + Temurin JDK 21 was used to verify this.)
set -euo pipefail
cd "$(dirname "$0")"
ROOT="$(cd ../../.. && pwd)"

# Auto-bootstrap the toolchain (linux x64) if KOTLINC/JAVA_HOME aren't provided.
# kotlinc crashes on a JDK whose version ends in "-ea", so we fetch a release JDK 21.
if [ -z "${KOTLINC:-}" ] || [ -z "${JAVA_HOME:-}" ]; then
  CACHE="${XDG_CACHE_HOME:-$HOME/.cache}/qaat-verify"; mkdir -p "$CACHE"
  if [ ! -x "$CACHE/jdk21/bin/java" ]; then
    echo "[bootstrap] fetching Temurin JDK 21…"
    curl -sL -o "$CACHE/jdk21.tgz" "https://api.adoptium.net/v3/binary/latest/21/ga/linux/x64/jdk/hotspot/normal/eclipse"
    mkdir -p "$CACHE/jdk21"; tar xzf "$CACHE/jdk21.tgz" -C "$CACHE/jdk21" --strip-components=1
  fi
  if [ ! -x "$CACHE/kotlinc/bin/kotlinc" ]; then
    echo "[bootstrap] fetching kotlinc 2.0.21…"
    curl -sL -o "$CACHE/kotlin.zip" "https://github.com/JetBrains/kotlin/releases/download/v2.0.21/kotlin-compiler-2.0.21.zip"
    unzip -q -o "$CACHE/kotlin.zip" -d "$CACHE"
  fi
  JAVA_HOME="$CACHE/jdk21"; KOTLINC="$CACHE/kotlinc/bin/kotlinc"
  chmod +x "$CACHE/kotlinc/bin/"* 2>/dev/null || true
fi
export JAVA_HOME   # kotlinc must run under this release JDK (its child `java`)
JAVA="$JAVA_HOME/bin/java"
JAR=/tmp/qaat-crypto-core.jar

echo "[1/4] compile crypto-core + parity harnesses"
"$KOTLINC" src/main/kotlin/ug/qaat/crypto/*.kt src/parity/kotlin/*.kt -include-runtime -d "$JAR" 2>/dev/null

echo "[2/4] seal a package in Kotlin, decrypt with the real sync-receiver crypto"
BK="0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
PT='{"session":{"id":"s1"},"attendance_records":[],"sealed_at":"2026-01-01T00:00:00Z","coordinator_id":"c1","package_version":"1.0"}'
SEALED=$("$JAVA" -cp "$JAR" ParityMainKt "$BK" "$PT")
EP=$(python3 -c "import json,sys;print(json.loads(sys.argv[1])['encryptedPayload'])" "$SEALED")
HM=$(python3 -c "import json,sys;print(json.loads(sys.argv[1])['hmac'])" "$SEALED")
CS=$(python3 -c "import json,sys;print(json.loads(sys.argv[1])['packageChecksum'])" "$SEALED")
( cd "$ROOT/services/sync-receiver" && go run ./cmd/parity "$BK" "$EP" "$HM" "$CS" "$PT" )

echo "[3/4] generate a Node-signed QR + HMAC vector"
QR=$(docker exec infra-qr-generator-1 node -e '
const c=require("crypto");
const {privateKey,publicKey}=c.generateKeyPairSync("rsa",{modulusLength:2048,publicKeyEncoding:{type:"spki",format:"pem"},privateKeyEncoding:{type:"pkcs8",format:"pem"}});
const p={student_id:"NUT/CS/2024/001",tenant_id:"t1",course_id:"CS",full_name:"Jane Doe",academic_year:"2024/2025",serial_number:"abc123",expiry_date:"2027-01-01",issued_at:"2026-01-01T00:00:00Z"};
const body=JSON.stringify(p);
console.log(JSON.stringify({publicKey,body,signature:c.createSign("RSA-SHA256").update(body).sign(privateKey,"base64"),payload:p,rosterHash:c.createHmac("sha256","tenant-hash-key-123").update("NUT/CS/2024/001").digest("hex")}));')

echo "[4/6] verify QR signature + canonical body + roster hash in Kotlin"
python3 - "$JAR" "$QR" <<'PY'
import json,subprocess,sys
jar=sys.argv[1]; d=json.loads(sys.argv[2]); p=d['payload']
a=["java","-cp",jar,"QrParityMainKt",d['publicKey'],d['body'],d['signature'],
   p['student_id'],p['tenant_id'],p['course_id'],p['full_name'],p['academic_year'],p['serial_number'],p['expiry_date'],p['issued_at'],
   "tenant-hash-key-123","NUT/CS/2024/001",d['rosterHash']]
import os; a[0]=os.path.join(os.environ['JAVA_HOME'],'bin','java')
r=subprocess.run(a,capture_output=True,text=True); print(r.stdout.strip()); sys.exit(r.returncode)
PY

echo "[5/6] full check-in engine: PRESENT + every rejection path vs a real signed QR"
ENGJAR=/tmp/qaat-engine.jar
"$KOTLINC" src/main/kotlin/ug/qaat/crypto/*.kt ../engine/src/main/kotlin/ug/qaat/engine/*.kt ../engine/src/test/kotlin/*.kt -include-runtime -d "$ENGJAR" 2>/dev/null
FULLQR=$(docker exec infra-qr-generator-1 node -e '
const c=require("crypto");const {privateKey,publicKey}=c.generateKeyPairSync("rsa",{modulusLength:2048,publicKeyEncoding:{type:"spki",format:"pem"},privateKeyEncoding:{type:"pkcs8",format:"pem"}});
const p={student_id:"NUT/CS/2024/001",tenant_id:"kiu",course_id:"CS",full_name:"Jane Doe",academic_year:"2024/2025",serial_number:"SER-1",expiry_date:"2027-01-01",issued_at:"2026-01-01T00:00:00Z"};
const body=JSON.stringify(p);
console.log(JSON.stringify({publicKey,rawQr:JSON.stringify({...p,signature:c.createSign("RSA-SHA256").update(body).sign(privateKey,"base64"),hmac:""}),hashKey:"k"}));')
python3 - "$ENGJAR" "$FULLQR" <<'PY'
import json,subprocess,sys,os
jar=sys.argv[1]; d=json.loads(sys.argv[2])
r=subprocess.run([os.path.join(os.environ['JAVA_HOME'],'bin','java'),"-cp",jar,"EngineTestMainKt",d['publicKey'],d['rawQr'],d['hashKey']],capture_output=True,text=True)
print(r.stdout.strip().splitlines()[-1]); sys.exit(r.returncode)
PY

echo "[6/7] rotating room code matches the Go server"
SECRET="parity-secret"; T=1782604800
GOCODE=$( cd "$ROOT/services/api-gateway" && go run ./cmd/roomcode "$SECRET" "$T" )
"$JAVA" -cp "$ENGJAR" RoomCodeParityMainKt "$SECRET" "$T" "$GOCODE" | tail -1

echo "[7/8] lecturer gate: START/END + quorum + biometric + every rejection"
"$JAVA" -cp "$ENGJAR" LecturerGateTestMainKt | tail -1

echo "[8/9] session package JSON parses into the server's attendance struct"
PKG=$("$JAVA" -cp "$ENGJAR" PackageBuildMainKt "11111111-1111-1111-1111-111111111111" "coord-1" "hashAAA" "fpAAA" 1 "hashBBB" "fpBBB" 2)
if echo "$PKG" | ( cd "$ROOT/services/sync-receiver" && go run ./cmd/pkgparse ) | grep -q "^records=2$"; then
  echo "PACKAGE_CONTRACT_OK: server parses 2 records with the exact keys"
else echo "PACKAGE_CONTRACT_FAIL"; exit 1; fi

echo "[9/9] analytics: chronic-absentee + attendance-trend logic"
"$JAVA" -cp "$ENGJAR" FeaturesTestMainKt | tail -1

echo "✅ ALL PARITY PROOFS PASSED (crypto + QR + check-in + room code + lecturer gate + package contract + analytics)"
