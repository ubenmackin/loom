import { useEffect } from 'react'
import { Outlet, useNavigate } from 'react-router-dom'
import { useAuthStore } from '../stores/auth'

interface ProtectedRouteProps {
  requireAdmin?: boolean
}

const ProtectedRoute: React.FC<ProtectedRouteProps> = ({ requireAdmin = false }) => {
  const navigate = useNavigate()
  const { isAuthenticated, isAdmin } = useAuthStore()

  useEffect(() => {
    if (!isAuthenticated) {
      navigate('/login')
    } else if (requireAdmin && !isAdmin) {
      navigate('/')
    }
  }, [isAuthenticated, isAdmin, requireAdmin, navigate])

  if (!isAuthenticated || (requireAdmin && !isAdmin)) {
    return null
  }

  return <Outlet />
}

export default ProtectedRoute
