import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuthStore } from '../stores/auth'
import { getOnboardingCheck, signup } from '../api/client'
import FieldLabel from '../components/FieldLabel'

export default function OnboardingPage() {
  const navigate = useNavigate()
  const login = useAuthStore((state) => state.login)

  const [username, setUsername] = useState('')
  const [email, setEmail] = useState('')
  const [displayName, setDisplayName] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    const checkOnboarding = async () => {
      try {
        const result = await getOnboardingCheck()
      // If onboarding is not required, redirect to login
      if (!result.onboarding_required) {
          navigate('/login')
        }
      } catch {
        setError('Failed to check onboarding status')
      }
    }
    checkOnboarding()
  }, [navigate])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')

    if (password !== confirmPassword) {
      setError('Passwords do not match')
      return
    }

    if (!username.trim() || !email.trim() || !displayName.trim() || !password) {
      setError('All fields are required')
      return
    }

    setLoading(true)
    try {
      const signupResult = await signup({
        username: username.trim(),
        email: email.trim(),
        password,
        display_name: displayName.trim(),
      })

      login(signupResult)

      navigate('/')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Signup failed')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen bg-charcoal-dark text-white flex items-center justify-center p-4">
      <div className="w-full max-w-md border border-gray-200 dark:border-gray-border p-6">
        {/* Header */}
        <div className="mb-6">
          <h1 className="font-mono text-[10px] uppercase tracking-widest text-amber-primary mb-2">
            WELCOME TO LOOM
          </h1>
          <p className="font-mono text-[10px] text-neutral-400 uppercase tracking-widest">
            First-Time Setup
          </p>
        </div>

        {/* Form */}
        <form onSubmit={handleSubmit} className="space-y-4">
          {/* Username */}
          <div>
            <FieldLabel htmlFor="username">Username</FieldLabel>
            <input
              id="username"
              type="text"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              required
              autoComplete="username"
              className="w-full border-2 border-gray-border rounded-none bg-charcoal-dark text-white p-2 font-mono focus:border-amber-primary focus:ring-0"
            />
          </div>

          {/* Email */}
          <div>
            <FieldLabel htmlFor="email">Email</FieldLabel>
            <input
              id="email"
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              required
              autoComplete="email"
              className="w-full border-2 border-gray-border rounded-none bg-charcoal-dark text-white p-2 font-mono focus:border-amber-primary focus:ring-0"
            />
          </div>

          {/* Display Name */}
          <div>
            <FieldLabel htmlFor="displayName">Display Name</FieldLabel>
            <input
              id="displayName"
              type="text"
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
              required
              autoComplete="name"
              className="w-full border-2 border-gray-border rounded-none bg-charcoal-dark text-white p-2 font-mono focus:border-amber-primary focus:ring-0"
            />
          </div>

          {/* Password */}
          <div>
            <FieldLabel htmlFor="password">Password</FieldLabel>
            <input
              id="password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              autoComplete="new-password"
              className="w-full border-2 border-gray-border rounded-none bg-charcoal-dark text-white p-2 font-mono focus:border-amber-primary focus:ring-0"
            />
          </div>

          {/* Confirm Password */}
          <div>
            <FieldLabel htmlFor="confirmPassword">Confirm Password</FieldLabel>
            <input
              id="confirmPassword"
              type="password"
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              required
              autoComplete="new-password"
              className="w-full border-2 border-gray-border rounded-none bg-charcoal-dark text-white p-2 font-mono focus:border-amber-primary focus:ring-0"
            />
          </div>

          {/* Error message */}
          {error && (
            <p className="font-mono text-[10px] text-red-500">{error}</p>
          )}

          {/* Submit */}
          <button
            type="submit"
            disabled={loading}
            className="glow-button w-full"
          >
            {loading ? 'Creating...' : 'Create Account'}
          </button>
        </form>
      </div>
    </div>
  )
}
