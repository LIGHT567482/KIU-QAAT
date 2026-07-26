import type { Request, Response } from 'express'
import QRCode from 'qrcode'
import { generateTenantKeyPair, generateSerialNumber, signQRPayload } from '../crypto/rsa-keys.js'
import { renderQRImage } from '../crypto/qr-image.js'
import { getTenantKeys, storeTenantKeys } from '../store/tenant-keys.js'
import { deliverBatch, sendLinkQREmail, type QREmailJob } from '../email/delivery.js'
import { withTenant } from '../db.js'

const UUID_RE = /^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$/

// requireJWT (server.ts) has already verified the token and pinned X-Tenant-ID to
// the authenticated claim, so this header is now a trustworthy source of tenant
// identity — never the body.
function callerTenant(req: Request, res: Response): string | null {
  const tenantId = req.header('X-Tenant-ID') ?? ''
  if (!UUID_RE.test(tenantId)) {
    res.status(403).json({ error: 'INVALID_TENANT', message: 'missing or malformed tenant context' })
    return null
  }
  return tenantId
}

// POST /api/v1/qr/generate/batch
// Roles: DQA_DIRECTOR, ADMIN (enforced by API Gateway RBAC)
export async function generateBatch(req: Request, res: Response): Promise<void> {
  const tenant_id = callerTenant(req, res)
  if (!tenant_id) return

  const { academic_year, course_id } = req.body as {
    academic_year: string
    course_id?: string
  }

  if (!academic_year) {
    res.status(400).json({ error: 'INVALID_REQUEST', message: 'academic_year required' })
    return
  }

  // All DB work runs on one connection scoped to the tenant (RLS / qaat_app).
  await withTenant(tenant_id, async (db) => {
    // Ensure tenant has an RSA key pair; generate one if not.
    let tenantKeys = await getTenantKeys(db, tenant_id)
    if (!tenantKeys) {
      const pair = generateTenantKeyPair()
      await storeTenantKeys(db, tenant_id, pair.privateKeyPem, pair.publicKeyPem)
      tenantKeys = await getTenantKeys(db, tenant_id)
    }

    // Fetch students for this tenant + academic_year (+ optional course).
    const query = course_id
      ? `SELECT student_id, email, full_name, course_id FROM students_extended
         WHERE tenant_id = $1 AND academic_year = $2 AND course_id = $3
           AND enrollment_status = 'ACTIVE'`
      : `SELECT student_id, email, full_name, course_id FROM students_extended
         WHERE tenant_id = $1 AND academic_year = $2
           AND enrollment_status = 'ACTIVE'`

    const params = course_id ? [tenant_id, academic_year, course_id] : [tenant_id, academic_year]
    const students = (await db.query(query, params)).rows

    const jobId = `qr-gen-${Date.now()}`

    // Return immediately; the rest runs in the background but stays on this
    // tenant-scoped connection (held until the batch finishes).
    res.status(202).json({
      job_id: jobId,
      status: 'QUEUED',
      estimated_count: students.length,
    })

    try {
      const tenantRow = (await db.query(
        'SELECT name, domain FROM tenants WHERE tenant_id = $1', [tenant_id]
      )).rows[0]
      if (!tenantRow) {
        console.error(`[qr-generator] job=${jobId} aborted: tenant ${tenant_id} not found`)
        return
      }

      const expiry = new Date()
      expiry.setFullYear(expiry.getFullYear() + 1)
      const expiryDate = expiry.toISOString().split('T')[0]

      const jobs: QREmailJob[] = []

      for (const student of students) {
        const serial = generateSerialNumber()
        const payload = {
          student_id: student.student_id,
          tenant_id,
          course_id: student.course_id || '',
          full_name: student.full_name || '',
          academic_year,
          serial_number: serial,
          expiry_date: expiryDate,
          issued_at: new Date().toISOString(),
        }
        const signed = signQRPayload(payload, tenantKeys!.privateKeyPem, tenantKeys!.hmacSecret)
        const qrBuffer = await renderQRImage(signed, tenantRow?.domain)

        await db.query(
          `UPDATE students_extended SET qr_serial_number = $1, updated_at = now()
           WHERE student_id = $2 AND tenant_id = $3`,
          [serial, student.student_id, tenant_id],
        )

        jobs.push({
          to: student.email,
          studentId: student.student_id,
          tenantName: tenantRow.name,
          tenantDomain: tenantRow.domain,
          academicYear: academic_year,
          qrImageBuffer: qrBuffer,
        })
      }

      const { sent, failed } = await deliverBatch(jobs)
      console.info(`[qr-generator] job=${jobId} sent=${sent} failed=${failed}`)
    } catch (err) {
      console.error(`[qr-generator] job=${jobId} failed:`, err instanceof Error ? err.message : err)
    }
  }).catch((err) => {
    console.error('[qr-generator] generateBatch failed:', err instanceof Error ? err.message : err)
    if (!res.headersSent) {
      res.status(500).json({ error: 'INTERNAL_ERROR', message: 'generation failed' })
    }
  })
}

// POST /api/v1/qr/token
// Returns a student's signed QR token + portal URL for live DISPLAY (no email,
// no serial change). Used by the admin students table to render the QR with the
// tenant logo/colour. Generates+stores a serial on first call if the student has
// none yet, so the displayed QR is immediately valid for portal login.
export async function qrToken(req: Request, res: Response): Promise<void> {
  const tenant_id = callerTenant(req, res)
  if (!tenant_id) return
  const { student_id } = req.body as { student_id: string }
  if (!student_id) {
    res.status(400).json({ error: 'INVALID_REQUEST', message: 'student_id required' })
    return
  }

  await withTenant(tenant_id, async (db) => {
    const student = (await db.query(
      'SELECT student_id, course_id, full_name, academic_year, qr_serial_number FROM students_extended WHERE student_id = $1 AND tenant_id = $2',
      [student_id, tenant_id],
    )).rows[0]
    if (!student) {
      res.status(404).json({ error: 'NOT_FOUND', message: 'student not found' })
      return
    }

    let tenantKeys = await getTenantKeys(db, tenant_id)
    if (!tenantKeys) {
      const pair = generateTenantKeyPair()
      await storeTenantKeys(db, tenant_id, pair.privateKeyPem, pair.publicKeyPem)
      tenantKeys = await getTenantKeys(db, tenant_id)
    }

    let serial: string = student.qr_serial_number
    if (!serial) {
      serial = generateSerialNumber()
      await db.query(
        `UPDATE students_extended SET qr_serial_number = $1, updated_at = now() WHERE student_id = $2 AND tenant_id = $3`,
        [serial, student_id, tenant_id],
      )
    }

    const expiry = new Date()
    expiry.setFullYear(expiry.getFullYear() + 1)
    const signed = signQRPayload({
      student_id,
      tenant_id,
      course_id: student.course_id || '',
      full_name: student.full_name || '',
      academic_year: student.academic_year,
      serial_number: serial,
      expiry_date: expiry.toISOString().split('T')[0],
      issued_at: new Date().toISOString(),
    }, tenantKeys!.privateKeyPem, tenantKeys!.hmacSecret)

    const token = Buffer.from(JSON.stringify(signed)).toString('base64url')
    // SaaS-adaptive: build the portal URL from the host the TENANT is actually
    // using (forwarded by the gateway), so the QR follows the tenant's address
    // instead of a single baked host. Falls back to the configured env otherwise.
    const fwdHost = ((req.headers['x-forwarded-host'] as string) || '').split(',')[0].trim()
    const base = fwdHost
      ? `https://${fwdHost.split(':')[0]}:${process.env.STUDENT_PORTAL_PORT || '3003'}`
      : (process.env.STUDENT_PORTAL_URL || process.env.STUDENT_CHECKIN_BASE_URL || '').replace(/\/$/, '')
    // Render the QR server-side in the tenant's brand colour (the admin overlays
    // the tenant logo in the centre via CSS). No client QR lib needed. Also fetch the
    // domain so we can tag the URL with &org (below).
    const tenant = (await db.query(
      'SELECT COALESCE(brand_color,\'\') AS brand_color, COALESCE(logo_url,\'\') AS logo_url, COALESCE(domain,\'\') AS domain FROM tenants WHERE tenant_id = $1',
      [tenant_id],
    )).rows[0] || { brand_color: '', logo_url: '', domain: '' }
    // Encode only the globally-unique serial in the QR (not the full signed token)
    // so the QR stays low-density and scans reliably on any phone. The portal posts
    // the serial to /student/qr-login, which resolves + verifies it server-side.
    // &org lets the portal resolve the institution for the attendance lookup even when
    // the QR is scanned off any coordinator hotspot (the public progress call needs it).
    const orgQS = tenant.domain ? `&org=${encodeURIComponent(tenant.domain)}` : ''
    const url = base ? `${base}/?qr=${serial}${orgQS}` : `/?qr=${serial}${orgQS}`
    const dark = /^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6})$/.test(tenant.brand_color) ? tenant.brand_color : '#0f172a'
    const image = await QRCode.toDataURL(url, {
      width: 320, margin: 2, errorCorrectionLevel: 'H', color: { dark, light: '#ffffff' },
    })
    res.status(200).json({ student_id, serial_number: serial, token, url, image, brand_color: tenant.brand_color, logo_url: tenant.logo_url })
  }).catch((err) => {
    console.error('[qr-generator] qrToken failed:', err instanceof Error ? err.message : err)
    if (!res.headersSent) res.status(500).json({ error: 'INTERNAL_ERROR', message: 'token failed' })
  })
}

// POST /api/v1/qr/issue
// Generates a student's QR the moment they are registered and emails it to the
// registered address plus an optional additional address (#4c). Called server-
// to-server by the API gateway right after a student is created; idempotent —
// re-issues a fresh serial if called again.
export async function issueForStudent(req: Request, res: Response): Promise<void> {
  const tenant_id = callerTenant(req, res)
  if (!tenant_id) return

  const { student_id, additional_email } = req.body as {
    student_id: string
    additional_email?: string
  }
  if (!student_id) {
    res.status(400).json({ error: 'INVALID_REQUEST', message: 'student_id required' })
    return
  }

  await withTenant(tenant_id, async (db) => {
    const studentRow = (await db.query(
      'SELECT student_id, email, full_name, academic_year, course_id FROM students_extended WHERE student_id = $1 AND tenant_id = $2',
      [student_id, tenant_id],
    )).rows[0]
    if (!studentRow) {
      res.status(404).json({ error: 'NOT_FOUND', message: 'student not found' })
      return
    }

    // Ensure the tenant has a key pair (a brand-new tenant may not yet).
    let tenantKeys = await getTenantKeys(db, tenant_id)
    if (!tenantKeys) {
      const pair = generateTenantKeyPair()
      await storeTenantKeys(db, tenant_id, pair.privateKeyPem, pair.publicKeyPem)
      tenantKeys = await getTenantKeys(db, tenant_id)
    }

    const tenantRow = (await db.query(
      'SELECT name, domain FROM tenants WHERE tenant_id = $1', [tenant_id]
    )).rows[0]

    const serial = generateSerialNumber()
    const expiry = new Date()
    expiry.setFullYear(expiry.getFullYear() + 1)
    const signed = signQRPayload({
      student_id,
      tenant_id,
      course_id: studentRow.course_id || '',
      full_name: studentRow.full_name || '',
      academic_year: studentRow.academic_year,
      serial_number: serial,
      expiry_date: expiry.toISOString().split('T')[0],
      issued_at: new Date().toISOString(),
    }, tenantKeys!.privateKeyPem, tenantKeys!.hmacSecret)

    const qrBuffer = await renderQRImage(signed, tenantRow?.domain)

    await db.query(
      `UPDATE students_extended SET qr_serial_number = $1, updated_at = now() WHERE student_id = $2 AND tenant_id = $3`,
      [serial, student_id, tenant_id],
    )

    // Email the QR to the registered address + an optional second address.
    const recipients = [studentRow.email]
    const extra = (additional_email || '').trim()
    if (extra && extra.toLowerCase() !== String(studentRow.email).toLowerCase() && /.+@.+\..+/.test(extra)) {
      recipients.push(extra)
    }
    const jobs: QREmailJob[] = recipients.map((to) => ({
      to,
      studentId: student_id,
      tenantName: tenantRow?.name ?? 'QAAT',
      tenantDomain: tenantRow?.domain ?? 'qaat.local',
      academicYear: studentRow.academic_year,
      qrImageBuffer: qrBuffer,
    }))
    const { sent, failed } = await deliverBatch(jobs)

    res.status(200).json({
      student_id,
      serial_number: serial,
      recipients,
      delivery_status: sent > 0 ? 'EMAIL_SENT' : 'EMAIL_FAILED',
      sent,
      failed,
    })
  }).catch((err) => {
    console.error('[qr-generator] issueForStudent failed:', err instanceof Error ? err.message : err)
    if (!res.headersSent) {
      res.status(500).json({ error: 'INTERNAL_ERROR', message: 'issue failed' })
    }
  })
}

// POST /api/v1/qr/reissue
// Role: QA_OFFICER (enforced by API Gateway)
export async function reissueQR(req: Request, res: Response): Promise<void> {
  const tenant_id = callerTenant(req, res)
  if (!tenant_id) return

  const { student_id, reason_code, officer_id } = req.body as {
    student_id: string
    reason_code: string
    officer_id: string
  }

  if (!student_id || !reason_code || !officer_id) {
    res.status(400).json({ error: 'INVALID_REQUEST', message: 'student_id, reason_code, officer_id required' })
    return
  }

  await withTenant(tenant_id, async (db) => {
    // Scope the lookup to the caller's tenant so an officer cannot reissue (and
    // email a fresh QR for) a student belonging to another institution.
    const studentRow = (await db.query(
      'SELECT student_id, email, tenant_id, academic_year, course_id FROM students_extended WHERE student_id = $1 AND tenant_id = $2',
      [student_id, tenant_id],
    )).rows[0]

    if (!studentRow) {
      res.status(404).json({ error: 'NOT_FOUND', message: 'student not found' })
      return
    }

    const tenantKeys = await getTenantKeys(db, studentRow.tenant_id)
    if (!tenantKeys) {
      res.status(500).json({ error: 'INTERNAL_ERROR', message: 'tenant keys not found' })
      return
    }

    const tenantRow = (await db.query(
      'SELECT name, domain FROM tenants WHERE tenant_id = $1', [studentRow.tenant_id]
    )).rows[0]

    const serial = generateSerialNumber()
    const expiry = new Date()
    expiry.setFullYear(expiry.getFullYear() + 1)
    const signed = signQRPayload({
      student_id,
      tenant_id: studentRow.tenant_id,
      course_id: studentRow.course_id || '',
      full_name: studentRow.full_name || '',
      academic_year: studentRow.academic_year,
      serial_number: serial,
      expiry_date: expiry.toISOString().split('T')[0],
      issued_at: new Date().toISOString(),
    }, tenantKeys.privateKeyPem, tenantKeys.hmacSecret)

    const qrBuffer = await renderQRImage(signed, tenantRow?.domain)

    await db.query(
      `UPDATE students_extended SET qr_serial_number = $1, updated_at = now() WHERE student_id = $2 AND tenant_id = $3`,
      [serial, student_id, tenant_id],
    )

    const { sent } = await deliverBatch([{
      to: studentRow.email,
      studentId: student_id,
      tenantName: tenantRow.name,
      tenantDomain: tenantRow.domain,
      academicYear: studentRow.academic_year,
      qrImageBuffer: qrBuffer,
    }])

    res.status(200).json({
      student_id,
      new_serial_number: serial,
      delivery_status: sent > 0 ? 'EMAIL_QUEUED' : 'EMAIL_FAILED',
      logged_at: new Date().toISOString(),
    })
  }).catch((err) => {
    console.error('[qr-generator] reissueQR failed:', err instanceof Error ? err.message : err)
    if (!res.headersSent) {
      res.status(500).json({ error: 'INTERNAL_ERROR', message: 'reissue failed' })
    }
  })
}

// emailLink renders a QR for an arbitrary URL (e.g. a lecturer's permanent
// career-QR login link) and emails it to an explicit recipient. Unlike the
// student flows it does not mint or store a serial — the QR simply encodes the
// URL the caller supplies. Used for the optional QR-dispatch email that the
// gateway fires when a lecturer (or student) is created/imported with an email.
export async function emailLink(req: Request, res: Response): Promise<void> {
  const tenant_id = callerTenant(req, res)
  if (!tenant_id) return

  const { to, url, name, subject_id, heading, intro } = req.body as {
    to?: string; url?: string; name?: string
    subject_id?: string; heading?: string; intro?: string
  }
  if (!to || !url) {
    res.status(400).json({ error: 'INVALID_REQUEST', message: 'to and url required' })
    return
  }

  // Respond immediately; deliver in the background so import loops never block.
  res.status(202).json({ status: 'EMAIL_QUEUED' })

  withTenant(tenant_id, async (db) => {
    const tenantRow = (await db.query(
      'SELECT name, domain FROM tenants WHERE tenant_id = $1', [tenant_id],
    )).rows[0]
    if (!tenantRow) return

    const qrImageBuffer = await QRCode.toBuffer(url, {
      errorCorrectionLevel: 'M', margin: 2, width: 480,
    })

    await sendLinkQREmail({
      to,
      recipientName: name || '',
      subjectId: subject_id || '',
      tenantName: tenantRow.name,
      tenantDomain: tenantRow.domain,
      heading: heading || 'Your QR Code',
      intro: intro || 'Your permanent QR code is attached.',
      qrImageBuffer,
    })
    console.info(`[qr-generator] link-qr emailed to=${to} subject=${subject_id || ''}`)
  }).catch((err) => {
    console.error('[qr-generator] emailLink failed:', err instanceof Error ? err.message : err)
  })
}
