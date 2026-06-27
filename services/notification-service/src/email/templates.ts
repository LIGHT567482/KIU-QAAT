// Email templates for all QAAT notification types.

export interface TenantBranding {
  name: string
  domain: string
  logoUrl?: string
  brandColor: string
}

export function syncOverdueEmail(branding: TenantBranding, coordinatorName: string, sessionDate: string): string {
  return `
<div style="font-family:system-ui;max-width:520px;margin:auto;padding:24px">
  <div style="border-left:4px solid #f59e0b;padding-left:16px;margin-bottom:24px">
    <h2 style="margin:0;color:#92400e">Sync Overdue Alert</h2>
    <p style="color:#64748b;margin:4px 0 0">Session data has not been synced for 48+ hours</p>
  </div>
  <p>Hello ${coordinatorName},</p>
  <p>A session you conducted on <strong>${sessionDate}</strong> has not been synced to the ${branding.name} attendance system.</p>
  <p>Please open the QAAT Coordinator app and ensure you have internet connectivity to complete the sync.</p>
  <p style="color:#64748b;font-size:13px">If the issue persists, contact your QA Officer.</p>
  <hr style="border:none;border-top:1px solid #e2e8f0;margin:24px 0">
  <p style="color:#94a3b8;font-size:12px">${branding.name} · ${branding.domain}</p>
</div>`
}

export function qrReissuedEmail(branding: TenantBranding, studentName: string, reason: string): string {
  return `
<div style="font-family:system-ui;max-width:520px;margin:auto;padding:24px">
  <h2 style="color:#1e293b">Your QR Code Has Been Reissued</h2>
  <p>Dear ${studentName},</p>
  <p>Your attendance QR code has been reissued by the ${branding.name} QA team.</p>
  <p><strong>Reason:</strong> ${reason}</p>
  <p>Your new QR code is attached. Your previous code is now invalid.</p>
  <p style="color:#64748b;font-size:13px">Present this QR code at every lecture for attendance recording.</p>
  <hr style="border:none;border-top:1px solid #e2e8f0;margin:24px 0">
  <p style="color:#94a3b8;font-size:12px">${branding.name} · ${branding.domain}</p>
</div>`
}

export function wardenDataReceivedEmail(branding: TenantBranding, coordinatorName: string, unitName: string): string {
  return `
<div style="font-family:system-ui;max-width:520px;margin:auto;padding:24px">
  <div style="border-left:4px solid #22c55e;padding-left:16px;margin-bottom:24px">
    <h2 style="margin:0;color:#166534">Warden Data Received</h2>
    <p style="color:#64748b;margin:4px 0 0">${unitName}</p>
  </div>
  <p>Hello ${coordinatorName},</p>
  <p>Attendance data from a Warden session for <strong>${unitName}</strong> has been received and is ready for your review.</p>
  <p>Please open the QAAT Coordinator app to review and approve the Warden attendance records.</p>
  <hr style="border:none;border-top:1px solid #e2e8f0;margin:24px 0">
  <p style="color:#94a3b8;font-size:12px">${branding.name} · ${branding.domain}</p>
</div>`
}
