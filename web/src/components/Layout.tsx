import { Outlet } from 'react-router-dom'
import TopNav from './TopNav'
import SubNav from './SubNav'

export default function Layout() {
  return (
    <div className="min-h-screen flex flex-col bg-gray-50 dark:bg-charcoal-darkest">
      <TopNav />
      <SubNav />
      <main className="flex-1 p-4 md:p-6 overflow-auto">
        <Outlet />
      </main>
    </div>
  )
}
