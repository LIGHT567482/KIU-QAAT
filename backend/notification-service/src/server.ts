import express from 'express'
import nodemailer from 'nodemailer'
import { syncOverdueEmail, qrReissuedEmail, wardenDataReceivedEmail } from './email/templates.js'
import { sendSyncOverduePush, sendWardenDataPush } from './push/web-push.js'

const app = express()
app.use(express.json())

const transporter = nodemailer.createTransport({
  host:   process.env.SMTP_HOST ?? 'mailhog',
  port:   Number(process.env.SMTP_PORT ?? 1025),
  secure: process.env.SMTP_SECURE === 'true',
  auth:   process.env.SMTP_USER
    ? { user: process.env.SMTP_USER, pass: process.env.SMTP_PASS }
    : undefined,
})

// ─── Internal endpoints — called by other services, not the API Gateway ───────

app.get('/health', (_req, res) => res.json({ status: 'ok', service: 'notification-service' }))

// POST /notify/sync-overdue
app.post('/notify/sync-overdue', async (req, res) => {
  const { to, coordinator_name, session_date, branding, push_subscription } = req.body
  try {
    await transporter.sendMail({
      from:    `noreply@${branding.domain}`,
      to,
      subject: `[${branding.name}] Sync Overdue — Action Required`,
      html:    syncOverdueEmail(branding, coordinator_name, session_date),
    })
    if (push_subscription) {
      await sendSyncOverduePush(push_subscription, session_date).catch(console.warn)
    }
    res.json({ status: 'SENT' })
  } catch (e) {
    console.error('sync-overdue notification failed', e)
    res.status(500).json({ error: 'SEND_FAILED' })
  }
})

// POST /notify/qr-reissued
app.post('/notify/qr-reissued', async (req, res) => {
  const { to, student_name, reason, branding, qr_attachment } = req.body
  try {
    await transporter.sendMail({
      from:    `noreply@${branding.domain}`,
      to,
      subject: `[${branding.name}] Your QR Code Has Been Reissued`,
      html:    qrReissuedEmail(branding, student_name, reason),
      attachments: qr_attachment ? [{
        filename:    `qr-${Date.now()}.png`,
        content:     Buffer.from(qr_attachment, 'base64'),
        contentType: 'image/png',
      }] : [],
    })
    res.json({ status: 'SENT' })
  } catch (e) {
    console.error('qr-reissued notification failed', e)
    res.status(500).json({ error: 'SEND_FAILED' })
  }
})

// POST /notify/warden-data
app.post('/notify/warden-data', async (req, res) => {
  const { to, coordinator_name, unit_name, branding, push_subscription } = req.body
  try {
    await transporter.sendMail({
      from:    `noreply@${branding.domain}`,
      to,
      subject: `[${branding.name}] Warden Attendance Data Ready`,
      html:    wardenDataReceivedEmail(branding, coordinator_name, unit_name),
    })
    if (push_subscription) {
      await sendWardenDataPush(push_subscription, unit_name).catch(console.warn)
    }
    res.json({ status: 'SENT' })
  } catch (e) {
    console.error('warden-data notification failed', e)
    res.status(500).json({ error: 'SEND_FAILED' })
  }
})

const port = process.env.PORT ?? 3004
app.listen(port, () => console.info(`notification-service listening on :${port}`))
