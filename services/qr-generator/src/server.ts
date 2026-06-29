import express from 'express'
import { generateBatch, reissueQR, issueForStudent, qrToken, emailLink } from './handlers/generate.js'
import { requireJWT } from './middleware/auth.js'

const app = express()
app.use(express.json())

app.get('/health', (_req, res) => {
  res.json({ status: 'ok', service: 'qr-generator' })
})

// The API gateway validates JWT + RBAC, but qr-generator independently verifies
// the RS256 token (requireJWT) and derives the tenant from the verified claim, so
// it cannot be abused to mint QR codes for an arbitrary tenant if reached
// directly. (update.md H1)
app.post('/api/v1/qr/generate/batch', requireJWT, generateBatch)
app.post('/api/v1/qr/reissue',        requireJWT, reissueQR)
app.post('/api/v1/qr/issue',          requireJWT, issueForStudent)
app.post('/api/v1/qr/token',          requireJWT, qrToken)
app.post('/api/v1/qr/email-link',     requireJWT, emailLink)

const port = process.env.PORT ?? 3002
app.listen(port, () => {
  console.info(`qr-generator listening on :${port}`)
})
