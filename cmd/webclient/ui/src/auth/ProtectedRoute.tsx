import { type ReactNode } from 'react'
import { Navigate } from 'react-router-dom'
import { useAuth } from './AuthContext'

export type Role = 'player' | 'editor' | 'moderator' | 'admin'

interface ProtectedRouteProps {
  children: ReactNode
  requiredRole?: Role | Role[]
}

export function ProtectedRoute({ children, requiredRole }: ProtectedRouteProps) {
  const { user } = useAuth()

  if (!user) {
    return <Navigate to="/login" replace />
  }

  if (requiredRole) {
    const allowed = Array.isArray(requiredRole) ? requiredRole : [requiredRole]
    if (!allowed.includes(user.role as Role)) {
      return <Navigate to="/game" replace />
    }
  }

  return <>{children}</>
}
