import { Layers, Sun, Moon } from 'lucide-react'
import { NavLink } from 'react-router-dom'
import { useTheme } from '../hooks/useTheme'

function navLinkClass(isActive: boolean): string {
  return `flex items-center justify-center px-5 h-[52px] text-sm font-medium rounded-none transition-colors border-b-2 ${
    isActive
      ? 'bg-purple-active/10 text-purple-active border-purple-active'
      : 'text-slate-500 hover:text-white hover:bg-gray-100 dark:hover:bg-charcoal-darkest border-transparent'
  }`
}

export default function TopNav() {
  const { isDark, toggle } = useTheme()

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
        <NavLink to="/agents" className={({ isActive }) => navLinkClass(isActive)}>
          <span className="uppercase">Agents</span>
        </NavLink>
      </nav>

      {/* Right — Theme toggle */}
      <button
        onClick={toggle}
        className="p-2 rounded-none text-neutral-600 dark:text-neutral-400 hover:bg-gray-100 dark:hover:bg-charcoal-darkest transition-colors"
        aria-label={isDark ? 'Switch to light mode' : 'Switch to dark mode'}
      >
        {isDark ? <Sun size={16} /> : <Moon size={16} />}
      </button>
    </header>
  )
}
