import { createContext, useContext, useState, useCallback, type ReactNode } from 'react'

export type Role =
  | 'VC' | 'DQA_DIRECTOR' | 'QA_OFFICER' | 'COORDINATOR' | 'ADMIN' | 'LECTURER'
  // Org-scoped oversight. Each is bounded by the department or college/school on
  // the account: HOD/QA_DEPT_REP by department, DEAN/QA_SCHOOL_HANDLER by school.
  | 'HOD' | 'DEAN' | 'QA_SCHOOL_HANDLER' | 'QA_DEPT_REP'

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
    // The pending welcome toast carries the PREVIOUS user's name. Left behind, it greets whoever
    // signs in next by the wrong name. The theme lives in localStorage on purpose — it is a
    // preference of the device, not of the account, and survives sign-out.
    sessionStorage.removeItem('qaat_welcome')
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
