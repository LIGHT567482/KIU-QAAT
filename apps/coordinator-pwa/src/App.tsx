import { useAuthStore } from './store/auth'
import Login from './pages/Login'
import SessionPage from './pages/SessionPage'

export default function App() {
  const { isAuthenticated, isExpired } = useAuthStore()

  if (!isAuthenticated || isExpired()) {
    return <Login />
  }
  return <SessionPage />
}
