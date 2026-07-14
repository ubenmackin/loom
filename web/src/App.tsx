import { useState, useEffect, useCallback } from 'react'
import { Routes, Route } from 'react-router-dom'
import Layout from './components/Layout'
import ProtectedRoute from './components/ProtectedRoute'
import Board from './components/Board'
import ActivityPage from './pages/ActivityPage'
import AgentsPage from './pages/AgentsPage'
import OperationsPage from './pages/OperationsPage'
import LoginPage from './pages/LoginPage'
import OnboardingPage from './pages/OnboardingPage'
import ProfilePage from './pages/ProfilePage'
import ProfilesPage from './pages/ProfilesPage'
import ProjectsPage from './pages/ProjectsPage'
import UsersPage from './pages/UsersPage'
import { useWebSocket } from './hooks/useWebSocket'
import StoryFailedBanner from './components/StoryFailedBanner'
import type { FailedStoryInfo } from './components/StoryFailedBanner'

export default function App() {
  const { lastEvent } = useWebSocket() // activate real-time invalidation
  const [failedStories, setFailedStories] = useState<FailedStoryInfo[]>([])

  // Listen for story_failed WebSocket events
  useEffect(() => {
    if (!lastEvent || lastEvent.type !== 'story_failed') return
    const data = lastEvent.data as { story_id?: string; title?: string; failure_count?: number }
    const storyId = data.story_id
    const title = data.title
    const failureCount = data.failure_count
    if (!storyId) return
    setFailedStories((prev) => {
      // Avoid duplicates for the same story_id
      if (prev.some((f) => f.storyId === storyId)) return prev
      return [
        ...prev,
        {
          storyId,
          title: title ?? `Story ${storyId}`,
          failureCount: failureCount ?? 0,
        },
      ]
    })
  }, [lastEvent])

  const handleDismissBanner = useCallback((storyId: string) => {
    setFailedStories((prev) => prev.filter((f) => f.storyId !== storyId))
  }, [])

  return (
    <>
      {/* Global notification banners */}
      {failedStories.length > 0 && (
        <div className="fixed top-4 right-4 z-[100] flex flex-col gap-2 max-w-sm w-full">
          {failedStories.map((story) => (
            <StoryFailedBanner
              key={story.storyId}
              story={story}
              onDismiss={handleDismissBanner}
            />
          ))}
        </div>
      )}

      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/onboarding" element={<OnboardingPage />} />
        <Route element={<ProtectedRoute />}>
          <Route element={<Layout />}>
            <Route index element={<Board />} />
            <Route path="activity" element={<ActivityPage />} />
            <Route path="agents" element={<AgentsPage />} />
            <Route path="operations" element={<OperationsPage />} />
            <Route path="profile" element={<ProfilePage />} />
            <Route element={<ProtectedRoute requireAdmin />}>
              <Route path="projects" element={<ProjectsPage />} />
              <Route path="users" element={<UsersPage />} />
              <Route path="profiles" element={<ProfilesPage />} />
            </Route>
          </Route>
        </Route>
      </Routes>
    </>
  )
}
