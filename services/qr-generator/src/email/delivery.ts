import nodemailer from 'nodemailer'

// Rate-limited bulk QR delivery — 100 emails/second max (technicaldoc.md §6.1)

const transporter = nodemailer.createTransport({
  host: process.env.SMTP_HOST ?? 'mailhog',
  port: Number(process.env.SMTP_PORT ?? 1025),
  secure: process.env.SMTP_SECURE === 'true',
  auth: process.env.SMTP_USER
    ? { user: process.env.SMTP_USER, pass: process.env.SMTP_PASS }
    : undefined,
})

export interface QREmailJob {
  to: string
  studentId: string
  tenantName: string
  tenantDomain: string
  academicYear: string
  qrImageBuffer: Buffer
}

export async function sendQREmail(job: QREmailJob): Promise<void> {
  await transporter.sendMail({
    from: `noreply@${job.tenantDomain}`,
    to: job.to,
    subject: `Your ${job.tenantName} Attendance QR Code — ${job.academicYear}`,
    html: buildEmailHTML(job),
    attachments: [{
      filename: `qr-${job.studentId}.png`,
      content: job.qrImageBuffer,
      contentType: 'image/png',
      cid: 'qr-code',
    }],
  })
}

// Deliver a batch with 100 emails/second rate limit.
export async function deliverBatch(jobs: QREmailJob[]): Promise<{ sent: number; failed: number }> {
  const batchSize = 100
  let sent = 0
  let failed = 0

  for (let i = 0; i < jobs.length; i += batchSize) {
    const batch = jobs.slice(i, i + batchSize)
    const results = await Promise.allSettled(batch.map(sendQREmail))
    results.forEach(r => r.status === 'fulfilled' ? sent++ : failed++)

    // Hold 1 second between batches to enforce 100/s cap.
    if (i + batchSize < jobs.length) {
      await new Promise(r => setTimeout(r, 1000))
    }
  }
  return { sent, failed }
}

function buildEmailHTML(job: QREmailJob): string {
  return `
    <div style="font-family:system-ui;max-width:480px;margin:auto">
      <h2 style="color:#1e293b">${job.tenantName}</h2>
      <p>Your attendance QR code for the <strong>${job.academicYear}</strong> academic year is attached.</p>
      <img src="cid:qr-code" alt="Attendance QR Code" style="width:240px;height:240px;display:block;margin:16px 0" />
      <p style="color:#64748b;font-size:13px">
        Present this QR code to the Coordinator at the start of each lecture.
        Do not share it — it is bound to your registered device.
      </p>
      <p style="color:#94a3b8;font-size:12px">Student ID: ${job.studentId}</p>
    </div>
  `
}
