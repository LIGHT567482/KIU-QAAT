import { create } from 'zustand'
import { persist, createJSONStorage } from 'zustand/middleware'
import { initDeviceKey } from '../crypto/vault-crypto'

interface AuthState {
  token: string | null
  jti: string | null
  role: string | null
  userId: string | null
  fullName: string | null
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
  full_name?: string
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
      fullName: null,
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
          fullName: payload.full_name ?? null,
          tenantId: payload.tenant_id,
          expiresAt: Math.floor(Date.now() / 1000) + payload.expires_in,
          isAuthenticated: true,
        })
      },

      logout: () => {
        set({
          token: null, jti: null, role: null, userId: null, fullName: null,
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
      // Persist into localStorage so the coordinator stays signed in across app
      // restarts — essential for offline-first: the user logs in once while online,
      // and the token survives phone reboots and app restarts so the PWA opens
      // directly to the dashboard without requiring the server.
      storage: createJSONStorage(() => localStorage),
      partialize: (state) => ({
        token: state.token,
        jti: state.jti,
        role: state.role,
        userId: state.userId,
        fullName: state.fullName,
        tenantId: state.tenantId,
        expiresAt: state.expiresAt,
        isAuthenticated: state.isAuthenticated,
      }),
    },
  ),
)
