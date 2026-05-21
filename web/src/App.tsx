import { Routes, Route } from 'react-router-dom'
import Layout from './components/Layout'
import Board from './components/Board'
import ActivityPage from './pages/ActivityPage'
import AgentsPage from './pages/AgentsPage'

export default function App() {
  return (
    <Routes>
      <Route element={<Layout />}>
        <Route index element={<Board />} />
        <Route path="activity" element={<ActivityPage />} />
        <Route path="agents" element={<AgentsPage />} />
      </Route>
    </Routes>
  )
}
