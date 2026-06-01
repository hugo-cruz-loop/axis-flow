import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { AuthForm } from '@/features/auth/AuthForm'
import type { AuthFormValues } from '@/features/auth/AuthForm'
import * as authClient from '@/api/authClient'
import { useAuthStore } from '@/store/authStore'

export function LoginPage() {
  const [error, setError] = useState<string | null>(null)
  const [isLoading, setIsLoading] = useState(false)
  const navigate = useNavigate()
  const setAuth = useAuthStore((s) => s.setAuth)

  const handleSubmit = async (values: AuthFormValues) => {
    setError(null)
    setIsLoading(true)
    try {
      const loginResponse = await authClient.login(values.email, values.password)
      const user = await authClient.meWithToken(loginResponse.access_token)
      setAuth(loginResponse.access_token, user)
      navigate('/dashboard', { replace: true })
    } catch {
      // Do not log credentials or tokens
      setError('Invalid email or password. Please try again.')
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-gray-50 px-4">
      <div className="w-full max-w-md rounded-lg bg-white p-8 shadow">
        <h1 className="mb-6 text-2xl font-bold text-gray-900">Sign in</h1>
        <AuthForm onSubmit={handleSubmit} isLoading={isLoading} error={error} />
      </div>
    </div>
  )
}
