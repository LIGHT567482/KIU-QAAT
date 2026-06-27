import { createContext, useContext, useState, useCallback, type ReactNode } from 'react'

export type Role = 'VC' | 'DQA_DIRECTOR' | 'QA_OFFICER' | 'COORDINATOR' | 'ADMIN' | 'LECTURER'

interface AuthUser {
  userId: string
  tenantId: string
  role: Role
  token: string
  expiresAt: number
}

interface AuthContextValue {
  user: AuthUser | null
  login: (token: string, user: Omit<AuthUser, 'token'>) => void
  logout: () => void
  isAuthenticated: boolean
}

const AuthContext = createContext<AuthContextValue | null>(null)

const TOKEN_KEY = 'qaat_admin_token'

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<AuthUser | null>(() => {
    try {
      const raw = sessionStorage.getItem(TOKEN_KEY)
      if (!raw) return null
      const parsed: AuthUser = JSON.parse(raw)
      if (Math.floor(Date.now() / 1000) >= parsed.expiresAt) return null
      return parsed
    } catch {
      return null
    }
  })

  const login = useCallback((token: string, info: Omit<AuthUser, 'token'>) => {
    const u: AuthUser = { ...info, token }
    sessionStorage.setItem(TOKEN_KEY, JSON.stringify(u))
    setUser(u)
  }, [])

  const logout = useCallback(() => {
    sessionStorage.removeItem(TOKEN_KEY)
    setUser(null)
  }, [])

  return (
    <AuthContext.Provider value={{ user, login, logout, isAuthenticated: user !== null }}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used inside AuthProvider')
  return ctx
}
