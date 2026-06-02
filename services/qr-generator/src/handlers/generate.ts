import type { Request, Response } from 'express'
import { generateTenantKeyPair, generateSerialNumber, signQRPayload } from '../crypto/rsa-keys.js'
import { renderQRImage } from '../crypto/qr-image.js'
import { getTenantKeys, storeTenantKeys } from '../store/tenant-keys.js'
import { deliverBatch, type QREmailJob } from '../email/delivery.js'
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
      ? `SELECT student_id, email, full_name FROM students_extended
         WHERE tenant_id = $1 AND academic_year = $2 AND course_id = $3
           AND enrollment_status = 'ACTIVE'`
      : `SELECT student_id, email, full_name FROM students_extended
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
          academic_year,
          serial_number: serial,
          expiry_date: expiryDate,
          issued_at: new Date().toISOString(),
        }
        const signed = signQRPayload(payload, tenantKeys!.privateKeyPem, tenantKeys!.hmacSecret)
        const qrBuffer = await renderQRImage(signed)

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
      'SELECT student_id, email, tenant_id, academic_year FROM students_extended WHERE student_id = $1 AND tenant_id = $2',
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
      academic_year: studentRow.academic_year,
      serial_number: serial,
      expiry_date: expiry.toISOString().split('T')[0],
      issued_at: new Date().toISOString(),
    }, tenantKeys.privateKeyPem, tenantKeys.hmacSecret)

    const qrBuffer = await renderQRImage(signed)

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
