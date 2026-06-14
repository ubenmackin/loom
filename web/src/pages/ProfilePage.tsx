import { useState } from 'react'
import { Navigate } from 'react-router-dom'
import { useAuthStore } from '../stores/auth'
import { updateMyProfile } from '../api/client'

export default function ProfilePage() {
  const user = useAuthStore((s) => s.user)
  const updateUser = useAuthStore((s) => s.updateUser)

  const [displayName, setDisplayName] = useState(user?.display_name || '')
  const [email, setEmail] = useState(user?.email || '')
  const [showPasswordFields, setShowPasswordFields] = useState(false)
  const [currentPassword, setCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [success, setSuccess] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)

  // Redirect if not authenticated (safety guard — ProtectedRoute should prevent this)
  if (!user) {
    return <Navigate to="/login" replace />
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    setSuccess(null)

    const data: {
      display_name?: string
      email?: string
      current_password?: string
      new_password?: string
    } = {}

    if (displayName !== (user.display_name || '')) {
      data.display_name = displayName
    }
    if (email !== user.email) {
      data.email = email
    }

    if (showPasswordFields) {
      if (!currentPassword || !newPassword) {
        setError('Both current and new password are required when changing password')
        return
      }
      if (newPassword.length < 6) {
        setError('New password must be at least 6 characters')
        return
      }
      data.current_password = currentPassword
      data.new_password = newPassword
    }

    if (Object.keys(data).length === 0) {
      setError('No changes to save')
      return
    }

    try {
      setSaving(true)
      const updatedUser = await updateMyProfile(data)
      updateUser(updatedUser)
      setSuccess('Profile updated successfully')
      setCurrentPassword('')
      setNewPassword('')
      setShowPasswordFields(false)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update profile')
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="p-6">
      {error && (
        <div className="mb-4 p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 text-red-700 dark:text-red-400 text-sm">
          {error}
        </div>
      )}

      {success && (
        <div className="mb-4 p-3 bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 text-green-700 dark:text-green-400 text-sm">
          {success}
        </div>
      )}

      <form onSubmit={handleSubmit} className="max-w-lg">
        <div className="space-y-4">
          {/* Username (read-only) */}
          <div>
            <label className="block text-sm font-medium text-slate-700 dark:text-neutral-300 mb-1">
              Username
            </label>
            <input
              type="text"
              value={user.username}
              disabled
              className="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-border bg-gray-100 dark:bg-charcoal-darkest text-slate-500 dark:text-neutral-500 rounded-none cursor-not-allowed"
            />
            <p className="text-xs text-slate-400 dark:text-neutral-500 mt-1">
              Username cannot be changed
            </p>
          </div>

          {/* Display Name */}
          <div>
            <label className="block text-sm font-medium text-slate-700 dark:text-neutral-300 mb-1">
              Display Name
            </label>
            <input
              type="text"
              value={displayName}
              onChange={(e) => {
                setDisplayName(e.target.value)
                if (error) setError(null)
              }}
              className="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-border bg-white dark:bg-charcoal-darkest text-slate-800 dark:text-white rounded-none focus:outline-none focus:ring-1 focus:ring-purple-500"
              placeholder="Your display name"
            />
          </div>

          {/* Email */}
          <div>
            <label className="block text-sm font-medium text-slate-700 dark:text-neutral-300 mb-1">
              Email
            </label>
            <input
              type="email"
              value={email}
              onChange={(e) => {
                setEmail(e.target.value)
                if (error) setError(null)
              }}
              className="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-border bg-white dark:bg-charcoal-darkest text-slate-800 dark:text-white rounded-none focus:outline-none focus:ring-1 focus:ring-purple-500"
              placeholder="your@email.com"
            />
          </div>

          {/* Change Password Toggle */}
          <div className="border-t border-gray-200 dark:border-gray-border pt-4">
            <button
              type="button"
              onClick={() => setShowPasswordFields(!showPasswordFields)}
              className="text-sm font-medium text-purple-600 dark:text-purple-400 hover:text-purple-700 dark:hover:text-purple-300 transition-colors"
            >
              {showPasswordFields ? 'Cancel password change' : 'Change Password'}
            </button>

            {showPasswordFields && (
              <div className="mt-4 space-y-4">
                <div>
                  <label className="block text-sm font-medium text-slate-700 dark:text-neutral-300 mb-1">
                    Current Password
                  </label>
                  <input
                    type="password"
                    value={currentPassword}
                    onChange={(e) => {
                      setCurrentPassword(e.target.value)
                      if (error) setError(null)
                    }}
                    className="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-border bg-white dark:bg-charcoal-darkest text-slate-800 dark:text-white rounded-none focus:outline-none focus:ring-1 focus:ring-purple-500"
                    placeholder="Enter current password"
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-slate-700 dark:text-neutral-300 mb-1">
                    New Password
                  </label>
                  <input
                    type="password"
                    value={newPassword}
                    onChange={(e) => {
                      setNewPassword(e.target.value)
                      if (error) setError(null)
                    }}
                    className="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-border bg-white dark:bg-charcoal-darkest text-slate-800 dark:text-white rounded-none focus:outline-none focus:ring-1 focus:ring-purple-500"
                    placeholder="Enter new password (min 6 characters)"
                  />
                </div>
              </div>
            )}
          </div>
        </div>

        <div className="mt-6">
          <button
            type="submit"
            disabled={saving}
            className="px-6 py-2 text-sm font-medium text-white bg-purple-600 hover:bg-purple-700 disabled:opacity-50 disabled:cursor-not-allowed rounded-none transition-colors"
          >
            {saving ? 'Saving...' : 'Save Changes'}
          </button>
        </div>
      </form>
    </div>
  )
}
