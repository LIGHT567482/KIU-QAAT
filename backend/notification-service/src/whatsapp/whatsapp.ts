// Pluggable WhatsApp sender. It is a no-op that just logs until you provide provider credentials
// via environment variables — so the notification flow works end-to-end now (email + app) and
// WhatsApp "lights up" the moment creds are set, with no code change.
//
//   Twilio: WHATSAPP_PROVIDER=twilio, TWILIO_ACCOUNT_SID, TWILIO_AUTH_TOKEN,
//           TWILIO_WHATSAPP_FROM (e.g. "whatsapp:+14155238886")
//   Meta:   WHATSAPP_PROVIDER=meta, META_PHONE_NUMBER_ID, META_ACCESS_TOKEN

export async function sendWhatsApp(toPhone: string, message: string): Promise<boolean> {
  const provider = (process.env.WHATSAPP_PROVIDER ?? '').toLowerCase()
  const phone = (toPhone ?? '').replace(/[^\d+]/g, '')
  if (!phone) return false
  try {
    if (provider === 'twilio' && process.env.TWILIO_ACCOUNT_SID) {
      const sid = process.env.TWILIO_ACCOUNT_SID
      const token = process.env.TWILIO_AUTH_TOKEN ?? ''
      const from = process.env.TWILIO_WHATSAPP_FROM ?? 'whatsapp:+14155238886'
      const res = await fetch(`https://api.twilio.com/2010-04-01/Accounts/${sid}/Messages.json`, {
        method: 'POST',
        headers: {
          Authorization: 'Basic ' + Buffer.from(`${sid}:${token}`).toString('base64'),
          'Content-Type': 'application/x-www-form-urlencoded',
        },
        body: new URLSearchParams({ From: from, To: `whatsapp:${phone}`, Body: message }),
      })
      return res.ok
    }
    if (provider === 'meta' && process.env.META_ACCESS_TOKEN) {
      const pnid = process.env.META_PHONE_NUMBER_ID
      const token = process.env.META_ACCESS_TOKEN
      const res = await fetch(`https://graph.facebook.com/v20.0/${pnid}/messages`, {
        method: 'POST',
        headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
        body: JSON.stringify({
          messaging_product: 'whatsapp',
          to: phone.replace('+', ''),
          type: 'text',
          text: { body: message },
        }),
      })
      return res.ok
    }
  } catch (e) {
    console.warn('whatsapp send failed', e)
    return false
  }
  // Not configured (or unknown provider) — log so the end-to-end flow works without creds.
  console.log(`[whatsapp:stub] to=${phone} msg=${message.slice(0, 80)}`)
  return false
}
