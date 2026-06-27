import { useState, useCallback } from 'react'
import { db } from '../db/vault'
import { decrypt, encrypt } from '../crypto/vault-crypto'
import { useAuthStore } from '../store/auth'

const API = import.meta.env.VITE_API_URL ?? (typeof location !== 'undefined' ? `${location.protocol}//${location.hostname}:8443` : 'http://localhost:8443')

export type FetchStatus = 'idle' | 'fetching' | 'fresh' | 'cached' | 'error'

export function useManifest() {
  const token = useAuthStore(s => s.token)
  const userId = useAuthStore(s => s.userId)
  const [status, setStatus] = useState<FetchStatus>('idle')
  const [error, setError] = useState<string | null>(null)

  const fetchManifest = useCallback(async () => {
    setStatus('fetching')
    setError(null)
    const today = new Date().toISOString().split('T')[0]
    const manifestId = `${today}-${userId}`

    // Check local cache first.
    const cached = await db.daily_manifest.get(manifestId)
    if (cached) {
      const expiresAt = new Date(cached.expires_at)
      if (expiresAt > new Date()) {
        setStatus('cached')
        return
      }
    }

    try {
      const res = await fetch(`${API}/api/v1/manifest/daily`, {
        headers: {
          Authorization: `Bearer ${token}`,
          'X-Device-Fingerprint': await getStoredFingerprint(),
        },
      })

      if (!res.ok) {
        throw new Error(`Server returned ${res.status}`)
      }

      const manifest = await res.json()

      // Encrypt before storing — no plaintext roster on disk.
      const encBlob = await encrypt(JSON.stringify(manifest))

      await db.daily_manifest.put({
        manifest_id: manifestId,
        date: today,
        coordinator_id: userId ?? '',
        encrypted_blob: encBlob,
        fetched_at: new Date().toISOString(),
        expires_at: manifest.expires_at,
      })

      // Cache tenant policy for offline use.
      if (manifest.policy) {
        await db.policy_cache.put({
          tenant_id: manifest.policy.tenant_id ?? 'default',
          ...manifest.policy,
          fetched_at: new Date().toISOString(),
        })
      }

      setStatus('fresh')
    } catch (e) {
      const msg = e instanceof Error ? e.message : 'Unknown error'
      setError(msg)
      // Fall back to stale cache if available.
      if (cached) {
        setStatus('cached')
      } else {
        setStatus('error')
      }
    }
  }, [token, userId])

  const getDecryptedManifest = useCallback(async () => {
    const today = new Date().toISOString().split('T')[0]
    const manifestId = `${today}-${userId}`
    const cached = await db.daily_manifest.get(manifestId)
    if (!cached) return null
    const json = await decrypt(cached.encrypted_blob)
    return JSON.parse(json)
  }, [userId])

  return { fetchManifest, getDecryptedManifest, status, error }
}

async function getStoredFingerprint(): Promise<string> {
  const { computeHardwareFingerprint } = await import('../crypto/fingerprint')
  return computeHardwareFingerprint()
}
