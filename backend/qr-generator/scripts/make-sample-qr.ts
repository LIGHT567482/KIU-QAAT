/**
 * Generate SAMPLE student QR cards locally — no DB, no email, no network.
 * Proves the current code bakes the offline LAN check-in URL and gives you scannable PNGs to test
 * the rebuilt coordinator app end-to-end. Run:
 *
 *   cd backend/qr-generator && npx tsx scripts/make-sample-qr.ts
 *
 * NOTE: these are signed with a THROWAWAY tenant key, so a real device running a real manifest will
 * reject them as INVALID_SIGNATURE — they verify the SCAN → PAGE → SUBMIT transport + URL, not full
 * roster validation. To push REAL, valid cards to students, use POST /api/v1/qr/generate/batch
 * against your deployment (see the notes printed at the end).
 */
import { mkdirSync, writeFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { generateTenantKeyPair, signQRPayload, generateSerialNumber, verifyQRSignature } from '../src/crypto/rsa-keys.js'
import { renderQRImage, checkinUrl } from '../src/crypto/qr-image.js'

const outDir = join(dirname(fileURLToPath(import.meta.url)), 'out')
mkdirSync(outDir, { recursive: true })

const keys = generateTenantKeyPair()
const tenantId = '11111111-1111-1111-1111-111111111111'
const expiry = new Date(); expiry.setFullYear(expiry.getFullYear() + 1)

const students = [
  { student_id: 'STU-ALICE', full_name: 'Alice Aine',  course_id: 'CS101' },
  { student_id: 'STU-BOB',   full_name: 'Bob Bua',     course_id: 'CS101' },
]

const run = async () => {
  for (const s of students) {
    const signed = signQRPayload(
      {
        student_id: s.student_id, tenant_id: tenantId, course_id: s.course_id, full_name: s.full_name,
        academic_year: '2026', serial_number: generateSerialNumber(),
        expiry_date: expiry.toISOString().split('T')[0], issued_at: new Date().toISOString(),
      },
      keys.privateKeyPem, 'sample-hmac-secret',
    )
    const url = checkinUrl(signed)
    const png = await renderQRImage(signed)
    const file = join(outDir, `sample-${s.student_id}.png`)
    writeFileSync(file, png)
    const ok = verifyQRSignature(signed, keys.publicKeyPem)
    console.log(`\n${s.student_id} (${s.full_name})`)
    console.log(`  QR encodes : ${url}`)
    console.log(`  signature  : ${ok ? 'valid' : 'INVALID'}`)
    console.log(`  PNG        : ${file}`)
    if (!url.startsWith('http://192.168.43.1:8080/checkin?t=')) {
      throw new Error(`QR is NOT baking the LAN hub URL — got ${url}`)
    }
  }
  console.log(`\nAll sample QR codes bake the offline LAN hub URL (http://192.168.43.1:8080/checkin?t=…). ✅`)
  console.log(`Scan them to confirm the rebuilt app opens the check-in page (they will be rejected as`)
  console.log(`INVALID_SIGNATURE on a real manifest — that still proves the scan→page→submit path).`)
  console.log(`\nTo update REAL students' cards on your deployment:`)
  console.log(`  1. (optional) set CHECKIN_BASE_URL=http://<your-hotspot-ip>:8080 if not 192.168.43.1`)
  console.log(`  2. ensure STUDENT_PORTAL_URL is UNSET (it forces the online serial-only URL)`)
  console.log(`  3. POST /api/v1/qr/generate/batch { academic_year, course_id? } (admin JWT)`)
  console.log(`  4. IMPORTANT: this assigns NEW serials — coordinators must RE-DOWNLOAD the manifest`)
  console.log(`     before the next session, or check-ins fail with SERIAL_REVOKED.`)
}

run().catch((e) => { console.error(e); process.exit(1) })
