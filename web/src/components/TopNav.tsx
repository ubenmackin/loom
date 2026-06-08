import { useState, useRef, useEffect } from 'react'
import { Layers, Sun, Moon, User, LogOut, Users } from 'lucide-react'
import { NavLink, useNavigate } from 'react-router-dom'
import { useTheme } from '../hooks/useTheme'
import { useAuthStore } from '../stores/auth'

function navLinkClass(isActive: boolean): string {
  return `flex items-center justify-center px-5 h-[52px] text-sm font-medium rounded-none transition-colors border-b-2 ${
    isActive
      ? 'bg-purple-active/10 text-purple-active border-purple-active'
      : 'text-slate-500 hover:text-white hover:bg-gray-100 dark:hover:bg-charcoal-darkest border-transparent'
  }`
}

export default function TopNav() {
  const { isDark, toggle } = useTheme()
  const user = useAuthStore((s) => s.user)
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated)
  const isAdmin = useAuthStore((s) => s.user?.role === 'admin')
  const logout = useAuthStore((s) => s.logout)
  const navigate = useNavigate()
  const [dropdownOpen, setDropdownOpen] = useState(false)
  const dropdownRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target as Node)) {
        setDropdownOpen(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [])

  const handleLogout = () => {
    logout()
    setDropdownOpen(false)
    navigate('/login')
  }

  const handleProfileClick = () => {
    setDropdownOpen(false)
    // placeholder - would navigate to profile page if it existed
  }

  return (
    <header className="h-[52px] bg-white dark:bg-charcoal-dark border-b border-gray-200 dark:border-gray-border flex items-center justify-between px-4 sticky top-0 z-40">
      {/* Left — Brand */}
      <div className="flex items-center gap-2">
        <Layers className="h-6 w-6 text-amber-primary" />
        <span className="text-xl font-bold text-loom-600 dark:text-purple-active">
          LOOM
        </span>
        <span className="hidden sm:inline text-gray-400 dark:text-amber-muted">|</span>
        <span className="hidden sm:inline text-sm font-normal text-gray-500 dark:text-amber-muted whitespace-nowrap">
          JIT Kanban
        </span>
      </div>

      {/* Center — Nav */}
      <nav className="hidden md:flex items-center gap-1" aria-label="Main navigation">
        <NavLink to="/" end className={({ isActive }) => navLinkClass(isActive)}>
          <span className="uppercase">Board</span>
        </NavLink>
        <NavLink to="/activity" className={({ isActive }) => navLinkClass(isActive)}>
          <span className="uppercase">Activity</span>
        </NavLink>
        <NavLink to="/dispatcher" className={({ isActive }) => navLinkClass(isActive)}>
          <span className="uppercase">Dispatcher</span>
        </NavLink>
        <NavLink to="/gateway" className={({ isActive }) => navLinkClass(isActive)}>
          <span className="uppercase">Gateway</span>
        </NavLink>
        <NavLink to="/agents" className={({ isActive }) => navLinkClass(isActive)}>
          <span className="uppercase">Agents</span>
        </NavLink>
      </nav>

      {/* Right — Actions */}
      <div className="flex items-center gap-2">
        {isAuthenticated && user ? (
          <>
            {/* User dropdown */}
            <div className="relative" ref={dropdownRef}>
              <button
                onClick={() => setDropdownOpen(!dropdownOpen)}
                className="flex items-center gap-2 px-3 py-2 text-sm font-mono text-slate-700 dark:text-neutral-300 bg-gray-50 dark:bg-charcoal-darkest border border-gray-200 dark:border-gray-border hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors"
                aria-label="User menu"
                aria-expanded={dropdownOpen}
              >
                <User size={16} />
                <span className="hidden sm:inline">
                  {user.display_name || user.username}
                </span>
              </button>

              {dropdownOpen && (
                <div className="absolute right-0 mt-1 w-48 bg-white dark:bg-charcoal-dark border border-gray-border rounded-none shadow-lg z-50">
                  <button
                    onClick={handleProfileClick}
                    className="w-full px-4 py-2 text-left text-sm font-mono text-slate-700 dark:text-neutral-300 hover:bg-gray-100 dark:hover:bg-charcoal-darkest border-b border-gray-200 dark:border-gray-border transition-colors"
                  >
                    Profile
                  </button>
                  {isAdmin && (
                    <>
                      <NavLink
                        to="/projects"
                        onClick={() => setDropdownOpen(false)}
                        className="flex items-center gap-2 px-4 py-2 text-sm font-mono text-slate-700 dark:text-neutral-300 hover:bg-gray-100 dark:hover:bg-charcoal-darkest border-b border-gray-200 dark:border-gray-border transition-colors"
                      >
                        <Layers size={14} />
                        <span>Projects</span>
                      </NavLink>
                      <NavLink
                        to="/users"
                        onClick={() => setDropdownOpen(false)}
                        className="flex items-center gap-2 px-4 py-2 text-sm font-mono text-slate-700 dark:text-neutral-300 hover:bg-gray-100 dark:hover:bg-charcoal-darkest border-b border-gray-200 dark:border-gray-border transition-colors"
                      >
                        <Users size={14} />
                        <span>Users</span>
                      </NavLink>
                      <NavLink
                        to="/profiles"
                        onClick={() => setDropdownOpen(false)}
                        className="flex items-center gap-2 px-4 py-2 text-sm font-mono text-slate-700 dark:text-neutral-300 hover:bg-gray-100 dark:hover:bg-charcoal-darkest border-b border-gray-200 dark:border-gray-border transition-colors"
                      >
                        <Layers size={14} />
                        <span>Profiles</span>
                      </NavLink>
                    </>
                  )}
                  <button
                    onClick={handleLogout}
                    className="w-full flex items-center gap-2 px-4 py-2 text-left text-sm font-mono text-red-600 dark:text-red-400 hover:bg-gray-100 dark:hover:bg-charcoal-darkest transition-colors"
                  >
                    <LogOut size={14} />
                    <span>Logout</span>
                  </button>
                </div>
              )}
            </div>

            {/* Theme toggle */}
            <button
              onClick={toggle}
              className="p-2 rounded-none text-neutral-600 dark:text-neutral-400 hover:bg-gray-100 dark:hover:bg-charcoal-darkest transition-colors"
              aria-label={isDark ? 'Switch to light mode' : 'Switch to dark mode'}
            >
              {isDark ? <Sun size={16} /> : <Moon size={16} />}
            </button>
          </>
        ) : (
          <>
            {/* Sign In link */}
            <NavLink
              to="/login"
              className="px-4 py-2 text-sm font-mono text-slate-700 dark:text-neutral-300 bg-gray-50 dark:bg-charcoal-darkest border border-gray-200 dark:border-gray-border hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors"
            >
              Sign In
            </NavLink>

            {/* Theme toggle */}
            <button
              onClick={toggle}
              className="p-2 rounded-none text-neutral-600 dark:text-neutral-400 hover:bg-gray-100 dark:hover:bg-charcoal-darkest transition-colors"
              aria-label={isDark ? 'Switch to light mode' : 'Switch to dark mode'}
            >
              {isDark ? <Sun size={16} /> : <Moon size={16} />}
            </button>
          </>
        )}
      </div>
    </header>
  )
}
