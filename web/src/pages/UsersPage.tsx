import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { getUsers, postUser, deleteUser } from '../api/client'
import type { User } from '../types'
import { relativeTime } from '../utils/relativeTime'

export default function UsersPage() {
  const queryClient = useQueryClient()

  const {
    data: users,
    isLoading,
    error: queryError,
    refetch,
  } = useQuery<User[]>({
    queryKey: ['users'],
    queryFn: getUsers,
  })

  const deleteMutation = useMutation({
    mutationFn: deleteUser,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users'] })
    },
  })

  const createMutation = useMutation({
    mutationFn: postUser,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users'] })
    },
  })

  // Form state
  const [isModalOpen, setIsModalOpen] = useState(false)
  const [username, setUsername] = useState('')
  const [email, setEmail] = useState('')
  const [displayName, setDisplayName] = useState('')
  const [password, setPassword] = useState('')
  const [role, setRole] = useState<'admin' | 'normal'>('normal')
  const [formError, setFormError] = useState<string | null>(null)

  const handleCreate = (e: React.FormEvent) => {
    e.preventDefault()
    setFormError(null)
    if (!username.trim() || !email.trim() || !password.trim()) {
      setFormError('Username, email, and password are required')
      return
    }
    createMutation.mutate(
      {
        username: username.trim(),
        email: email.trim(),
        display_name: displayName.trim(),
        password,
        role,
      },
      {
        onSuccess: () => {
          resetForm()
          setIsModalOpen(false)
        },
      }
    )
  }

  const resetForm = () => {
    setUsername('')
    setEmail('')
    setDisplayName('')
    setPassword('')
    setRole('normal')
    setFormError(null)
  }

  const handleCancel = () => {
    resetForm()
    setIsModalOpen(false)
  }

  const handleDelete = (userId: string) => {
    if (confirm('Are you sure you want to delete this user?')) {
      deleteMutation.mutate(userId)
    }
  }

  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-gray-200 dark:border-gray-border">
        <span className="text-[10px] uppercase tracking-widest font-bold text-neutral-600 dark:text-neutral-300">
          Users
        </span>
        <button
          onClick={() => setIsModalOpen(true)}
          className="glow-button"
        >
          CREATE USER
        </button>
      </div>

      {/* Table */}
      <div className="flex-1 overflow-auto">
        {isLoading ? (
          <div className="flex items-center justify-center h-64">
            <span className="font-mono text-sm text-neutral-500 dark:text-amber-muted">
              Loading users...
            </span>
          </div>
        ) : queryError ? (
          <div className="flex flex-col items-center justify-center h-64 gap-2">
            <span className="font-mono text-sm text-red-500">
              Error loading users: {queryError.message}
            </span>
            <button
              onClick={() => refetch()}
              className="glow-button text-xs"
            >
              Retry
            </button>
          </div>
        ) : users && users.length > 0 ? (
          <table className="w-full border-collapse">
            <thead>
              <tr className="border-b border-gray-200 dark:border-gray-border">
                <th className="px-4 py-3 text-left font-mono text-[10px] uppercase tracking-widest text-neutral-500 dark:text-neutral-400">
                  Username
                </th>
                <th className="px-4 py-3 text-left font-mono text-[10px] uppercase tracking-widest text-neutral-500 dark:text-neutral-400">
                  Email
                </th>
                <th className="px-4 py-3 text-left font-mono text-[10px] uppercase tracking-widest text-neutral-500 dark:text-neutral-400">
                  Display Name
                </th>
                <th className="px-4 py-3 text-left font-mono text-[10px] uppercase tracking-widest text-neutral-500 dark:text-neutral-400">
                  Role
                </th>
                <th className="px-4 py-3 text-left font-mono text-[10px] uppercase tracking-widest text-neutral-500 dark:text-neutral-400">
                  Created At
                </th>
                <th className="px-4 py-3 text-left font-mono text-[10px] uppercase tracking-widest text-neutral-500 dark:text-neutral-400">
                  Actions
                </th>
              </tr>
            </thead>
            <tbody>
              {users.map((user) => (
                <tr key={user.id} className="border-b border-gray-200 dark:border-gray-border hover:bg-gray-50 dark:hover:bg-charcoal-darkest">
                  <td className="px-4 py-3 font-mono text-sm text-neutral-800 dark:text-light-neutral">
                    {user.username}
                  </td>
                  <td className="px-4 py-3 font-mono text-sm text-neutral-800 dark:text-light-neutral">
                    {user.email}
                  </td>
                  <td className="px-4 py-3 font-mono text-sm text-neutral-800 dark:text-light-neutral">
                    {user.display_name ?? '-'}
                  </td>
                  <td className="px-4 py-3">
                    <span className={`sharp-tag ${user.role === 'admin' ? 'sharp-tag-primary' : 'sharp-tag-amber'}`}>
                      {user.role}
                    </span>
                  </td>
                  <td className="px-4 py-3 font-mono text-[10px] text-neutral-500 dark:text-neutral-400 whitespace-nowrap">
                    {relativeTime(user.created_at)}
                  </td>
                  <td className="px-4 py-3">
                    <button
                      onClick={() => handleDelete(user.id)}
                      disabled={deleteMutation.isPending}
                      className="glow-button-amber text-xs"
                    >
                      DELETE
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        ) : (
          <div className="flex items-center justify-center py-16">
            <span className="font-mono text-[10px] text-neutral-400 dark:text-neutral-600 uppercase tracking-widest">
              No users found
            </span>
          </div>
        )}
      </div>

      {/* Create User Modal */}
      {isModalOpen && (
        <div className="fixed inset-0 bg-black/50 z-50 flex items-center justify-center">
          <div className="bg-white dark:bg-charcoal-dark rounded-none shadow-none border border-gray-200 dark:border-gray-border w-[480px] max-h-[90vh] overflow-y-auto">
            {/* Header */}
            <div className="px-4 py-3 border-b border-gray-200 dark:border-gray-border flex items-center justify-between">
              <h2 className="text-[10px] uppercase tracking-widest text-neutral-800 dark:text-light-neutral font-bold">
                Create User
              </h2>
              <button
                onClick={handleCancel}
                className="p-1 rounded-none text-neutral-400 hover:text-neutral-600 dark:hover:text-neutral-200 transition-colors"
                aria-label="Close"
              >
                &times;
              </button>
            </div>

            {/* Form */}
            <form onSubmit={handleCreate} className="px-4 py-4 space-y-4">
              {/* Username */}
              <div>
                <label className="text-[10px] uppercase tracking-widest dark:text-amber-primary text-neutral-500 block mb-1">
                  Username <span className="text-red-500">*</span>
                </label>
                <input
                  type="text"
                  value={username}
          onChange={(e) => {
                setUsername(e.target.value)
                if (formError) setFormError(null)
              }}
              placeholder="Enter username"
                  className="w-full rounded-none border border-gray-200 dark:border-gray-border bg-charcoal-darkest p-2 text-sm text-neutral-800 dark:text-light-neutral font-mono"
                  autoFocus
                  required
                />
              </div>

              {/* Email */}
              <div>
                <label className="text-[10px] uppercase tracking-widest dark:text-amber-primary text-neutral-500 block mb-1">
                  Email <span className="text-red-500">*</span>
                </label>
                <input
                  type="email"
                  value={email}
          onChange={(e) => {
                setEmail(e.target.value)
                if (formError) setFormError(null)
              }}
              placeholder="Enter email"
                  className="w-full rounded-none border border-gray-200 dark:border-gray-border bg-charcoal-darkest p-2 text-sm text-neutral-800 dark:text-light-neutral font-mono"
                  required
                />
              </div>

              {/* Display Name */}
              <div>
                <label className="text-[10px] uppercase tracking-widest dark:text-amber-primary text-neutral-500 block mb-1">
                  Display Name
                </label>
                <input
                  type="text"
                  value={displayName}
                  onChange={(e) => setDisplayName(e.target.value)}
                  placeholder="Enter display name"
                  className="w-full rounded-none border border-gray-200 dark:border-gray-border bg-charcoal-darkest p-2 text-sm text-neutral-800 dark:text-light-neutral font-mono"
                />
              </div>

              {/* Password */}
              <div>
                <label className="text-[10px] uppercase tracking-widest dark:text-amber-primary text-neutral-500 block mb-1">
                  Password <span className="text-red-500">*</span>
                </label>
                <input
                  type="password"
                  value={password}
          onChange={(e) => {
                setPassword(e.target.value)
                if (formError) setFormError(null)
              }}
              placeholder="Enter password"
                  className="w-full rounded-none border border-gray-200 dark:border-gray-border bg-charcoal-darkest p-2 text-sm text-neutral-800 dark:text-light-neutral font-mono"
                  required
                />
              </div>

              {/* Role */}
              <div>
                <label className="text-[10px] uppercase tracking-widest dark:text-amber-primary text-neutral-500 block mb-1">
                  Role <span className="text-red-500">*</span>
                </label>
                <select
                  value={role}
                  onChange={(e) => setRole(e.target.value as 'admin' | 'normal')}
                  className="w-full rounded-none border border-gray-200 dark:border-gray-border bg-charcoal-darkest p-2 text-sm text-neutral-800 dark:text-light-neutral font-mono"
                >
                  <option value="normal">Normal</option>
                  <option value="admin">Admin</option>
                </select>
              </div>

  {/* Error */}
          {formError && (
            <p className="font-mono text-xs text-red-500">{formError}</p>
          )}

              {/* Actions */}
              <div className="pt-3 border-t border-gray-200 dark:border-gray-border flex items-center justify-end gap-2">
                <button
                  type="button"
                  onClick={handleCancel}
                  className="px-4 py-2 rounded-none border border-gray-300 dark:border-gray-border text-xs font-bold uppercase tracking-wider text-neutral-600 dark:text-neutral-300 hover:bg-neutral-100 dark:hover:bg-neutral-800 transition-colors"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="glow-button"
                  disabled={createMutation.isPending}
                >
                  {createMutation.isPending ? 'CREATING...' : 'CREATE'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}
