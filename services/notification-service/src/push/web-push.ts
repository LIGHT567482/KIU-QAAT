// PWA Push Notifications via Web Push (RFC 8030).
// Coordinators can opt in to receive push alerts when:
//   - Warden data arrives (needs review)
//   - Session sync is overdue (>48h)
//
// VAPID keys: generate with `npx web-push generate-vapid-keys`

import webpush from 'web-push'

// Web Push is optional. If VAPID keys are not configured, push silently
// degrades to a no-op rather than crashing the whole service at import time
// (which would also take down the unrelated SMTP notification path).
const VAPID_PUBLIC_KEY = process.env.VAPID_PUBLIC_KEY ?? ''
const VAPID_PRIVATE_KEY = process.env.VAPID_PRIVATE_KEY ?? ''
const pushEnabled = VAPID_PUBLIC_KEY !== '' && VAPID_PRIVATE_KEY !== ''

if (pushEnabled) {
  webpush.setVapidDetails(
    `mailto:admin@${process.env.PLATFORM_DOMAIN ?? 'qaat.platform'}`,
    VAPID_PUBLIC_KEY,
    VAPID_PRIVATE_KEY,
  )
} else {
  console.warn(
    '[web-push] VAPID_PUBLIC_KEY/VAPID_PRIVATE_KEY not set — push notifications disabled. ' +
      'Generate keys with `pnpm dlx web-push generate-vapid-keys`.',
  )
}

export interface PushSubscription {
  endpoint: string
  keys: { p256dh: string; auth: string }
}

export async function sendPush(
  subscription: PushSubscription,
  payload: { title: string; body: string; tag: string; url?: string },
): Promise<void> {
  if (!pushEnabled) return // push disabled — no-op
  await webpush.sendNotification(
    subscription,
    JSON.stringify({ notification: payload }),
  )
}

export async function sendSyncOverduePush(subscription: PushSubscription, sessionDate: string): Promise<void> {
  return sendPush(subscription, {
    title: 'QAAT — Sync Overdue',
    body:  `Session from ${sessionDate} has not synced. Open QAAT to upload.`,
    tag:   'sync-overdue',
    url:   '/',
  })
}

export async function sendWardenDataPush(subscription: PushSubscription, unitName: string): Promise<void> {
  return sendPush(subscription, {
    title: 'QAAT — Warden Data Ready',
    body:  `Warden attendance for ${unitName} is ready for review.`,
    tag:   'warden-data',
    url:   '/',
  })
}
