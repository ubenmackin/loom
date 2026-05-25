import { Routes, Route } from 'react-router-dom'
import Layout from './components/Layout'
import ProtectedRoute from './components/ProtectedRoute'
import Board from './components/Board'
import ActivityPage from './pages/ActivityPage'
import AgentsPage from './pages/AgentsPage'
import DispatcherPage from './pages/DispatcherPage'
import LoginPage from './pages/LoginPage'
import OnboardingPage from './pages/OnboardingPage'
import UsersPage from './pages/UsersPage'
import { useWebSocket } from './hooks/useWebSocket'

export default function App() {
  useWebSocket() // activate real-time invalidation
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/onboarding" element={<OnboardingPage />} />
      <Route element={<ProtectedRoute />}>
        <Route element={<Layout />}>
          <Route index element={<Board />} />
          <Route path="activity" element={<ActivityPage />} />
          <Route path="agents" element={<AgentsPage />} />
          <Route path="dispatcher" element={<DispatcherPage />} />
          <Route element={<ProtectedRoute requireAdmin />}>
            <Route path="users" element={<UsersPage />} />
          </Route>
        </Route>
      </Route>
    </Routes>
  )
}
