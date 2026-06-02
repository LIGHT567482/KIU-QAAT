import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import { initDeviceKey } from '../crypto/vault-crypto'

interface AuthState {
  token: string | null
  jti: string | null
  role: string | null
  userId: string | null
  tenantId: string | null
  expiresAt: number | null   // unix timestamp
  isAuthenticated: boolean

  login: (payload: LoginPayload) => Promise<void>
  logout: () => void
  isExpired: () => boolean
}

interface LoginPayload {
  access_token: string
  jti: string
  role: string
  user_id: string
  tenant_id: string
  expires_in: number
  device_binding_key?: string
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set, get) => ({
      token: null,
      jti: null,
      role: null,
      userId: null,
      tenantId: null,
      expiresAt: null,
      isAuthenticated: false,

      login: async (payload) => {
        if (payload.device_binding_key) {
          await initDeviceKey(payload.device_binding_key)
        }
        set({
          token: payload.access_token,
          jti: payload.jti,
          role: payload.role,
          userId: payload.user_id,
          tenantId: payload.tenant_id,
          expiresAt: Math.floor(Date.now() / 1000) + payload.expires_in,
          isAuthenticated: true,
        })
      },

      logout: () => {
        set({
          token: null, jti: null, role: null, userId: null,
          tenantId: null, expiresAt: null, isAuthenticated: false,
        })
      },

      isExpired: () => {
        const { expiresAt } = get()
        return expiresAt !== null && Math.floor(Date.now() / 1000) >= expiresAt
      },
    }),
    {
      name: 'qaat-auth',
      // Only persist non-sensitive fields. The option is `partialize` — the
      // previous `partialState` was silently ignored by zustand, which caused
      // the WHOLE store (including the JWT `token` and `jti`) to be written to
      // localStorage where any XSS could read it.
      partialize: (state) => ({
        role: state.role,
        userId: state.userId,
        tenantId: state.tenantId,
        expiresAt: state.expiresAt,
      }),
    },
  ),
)
