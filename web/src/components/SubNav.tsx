import { useLocation } from 'react-router-dom'
import { useAuthStore } from '../stores/auth'
import ProjectPicker from './ProjectPicker'

export default function SubNav() {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated)
  const location = useLocation()

  if (!isAuthenticated) {
    return null
  }

  const breadcrumb = location.pathname === '/' ? 'Board'
    : location.pathname === '/activity' ? 'Activity'
    : location.pathname === '/agents' ? 'Agents'
    : location.pathname === '/dispatcher' ? 'Dispatcher'
    : location.pathname === '/projects' ? 'Projects'
    : location.pathname === '/users' ? 'Users'
    : ''

  return (
    <div className="flex items-center gap-3 px-4 py-1.5 border-b border-gray-200 dark:border-gray-border bg-gray-50 dark:bg-charcoal-darkest">
      {/* Left — Project Picker */}
      <ProjectPicker />

      {/* Center — Breadcrumbs placeholder (flex-1) */}
      <div className="flex-1 flex items-center">
        {breadcrumb && (
          <span className="font-mono text-[10px] uppercase tracking-widest text-neutral-400 dark:text-neutral-500">
            {breadcrumb}
          </span>
        )}
      </div>
    </div>
  )
}