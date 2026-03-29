import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from 'react'
import { jwtDecode } from 'jwt-decode'

const TOKEN_KEY = 'mud_token'

interface JwtClaims {
  account_id: number
  role: string
  exp: number
}

interface AuthUser {
  accountId: number
  role: string
  exp: number
}

interface AuthContextValue {
  user: AuthUser | null
  token: string | null
  login: (token: string) => void
  logout: () => void
}

function decodeToken(token: string): AuthUser | null {
  try {
    const claims = jwtDecode<JwtClaims>(token)
    if (claims.exp * 1000 < Date.now()) {
      return null
    }
    return { accountId: claims.account_id, role: claims.role, exp: claims.exp }
  } catch {
    return null
  }
}

const AuthContext = createContext<AuthContextValue | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [token, setToken] = useState<string | null>(() => localStorage.getItem(TOKEN_KEY))
  const [user, setUser] = useState<AuthUser | null>(() => {
    const stored = localStorage.getItem(TOKEN_KEY)
    return stored ? decodeToken(stored) : null
  })

  useEffect(() => {
    if (token) {
      const decoded = decodeToken(token)
      if (!decoded) {
        localStorage.removeItem(TOKEN_KEY)
        setToken(null)
        setUser(null)
      } else {
        setUser(decoded)
      }
    }
  }, [token])

  const login = useCallback((newToken: string) => {
    localStorage.setItem(TOKEN_KEY, newToken)
    setToken(newToken)
    setUser(decodeToken(newToken))
  }, [])

  const logout = useCallback(() => {
    localStorage.removeItem(TOKEN_KEY)
    setToken(null)
    setUser(null)
  }, [])

  const value = useMemo<AuthContextValue>(
    () => ({ user, token, login, logout }),
    [user, token, login, logout],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext)
  if (!ctx) {
    throw new Error('useAuth must be used within an AuthProvider')
  }
  return ctx
}
