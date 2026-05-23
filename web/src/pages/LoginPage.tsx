import { useState, useEffect } from 'react'
import { useNavigate, Link } from 'react-router-dom'
import { useAuthStore } from '../stores/auth'
import { login as apiLogin } from '../api/client'

export default function LoginPage() {
  const navigate = useNavigate()
  const login = useAuthStore((state) => state.login)
  const isAuthenticated = useAuthStore((state) => state.isAuthenticated)

  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)

  // Redirect if already authenticated
  useEffect(() => {
    if (isAuthenticated) {
      navigate('/')
    }
  }, [isAuthenticated, navigate])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)

    // Basic validation
    if (!username.trim() || !password.trim()) {
      setError('Please provide both username and password')
      return
    }

    try {
      const authResponse = await apiLogin({ username_or_email: username.trim(), password: password.trim() })
      login(authResponse)
      navigate('/')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Login failed')
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-charcoal-darkest p-4">
      <div className="w-full max-w-md">
        {/* Header */}
        <div className="mb-8 text-center">
          <h1 className="font-mono text-[10px] uppercase tracking-widest text-neutral-300 mb-2">
            System Access
          </h1>
          <div className="border-b border-gray-border mx-auto w-16" />
        </div>

        {/* Login Form */}
        <form onSubmit={handleSubmit} className="border-2 border-gray-border bg-charcoal-dark p-6">
          {/* Username Field */}
          <div className="mb-4">
            <label htmlFor="username" className="block font-mono text-[10px] uppercase tracking-widest text-neutral-400 mb-2">
              Username
            </label>
            <input
              type="text"
              id="username"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              className="w-full border-2 border-gray-border bg-charcoal-darkest text-white px-3 py-2 font-mono text-sm focus:border-amber-primary focus:ring-0 outline-none transition-colors"
              placeholder="Username or email"
              autoComplete="username"
            />
          </div>

          {/* Password Field */}
          <div className="mb-6">
            <label htmlFor="password" className="block font-mono text-[10px] uppercase tracking-widest text-neutral-400 mb-2">
              Password
            </label>
            <input
              type="password"
              id="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className="w-full border-2 border-gray-border bg-charcoal-darkest text-white px-3 py-2 font-mono text-sm focus:border-amber-primary focus:ring-0 outline-none transition-colors"
              placeholder="Enter password"
              autoComplete="current-password"
            />
          </div>

          {/* Error Message */}
          {error && (
            <div className="mb-4 p-3 border border-red-500/50 bg-red-900/20">
              <p className="font-mono text-[10px] uppercase tracking-widest text-red-400">
                {error}
              </p>
            </div>
          )}

          {/* Sign In Button */}
          <button
            type="submit"
            className="glow-button w-full"
          >
            SIGN IN
          </button>
        </form>

        {/* New User Link */}
        <div className="mt-6 text-center">
          <p className="font-mono text-[10px] text-neutral-500">
            New to Loom?{' '}
            <Link
              to="/onboarding"
              className="text-amber-primary hover:text-amber-muted transition-colors uppercase tracking-widest"
            >
              Get Started
            </Link>
          </p>
        </div>
      </div>
    </div>
  )
}
