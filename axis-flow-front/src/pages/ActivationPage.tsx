import { useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { PasswordForm } from '@/features/auth/PasswordForm'
import type { PasswordFormValues } from '@/features/auth/PasswordForm'
import * as usersClient from '@/api/usersClient'

export function ActivationPage() {
  const [searchParams] = useSearchParams()
  const token = searchParams.get('token') ?? ''
  const [error, setError] = useState<string | null>(null)
  const [isLoading, setIsLoading] = useState(false)
  const navigate = useNavigate()

  const handleSubmit = async (values: PasswordFormValues) => {
    setError(null)
    setIsLoading(true)
    try {
      await usersClient.resetPassword(token, values.password)
      navigate('/login', { replace: true })
    } catch {
      setError('Failed to activate account. The link may have expired.')
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-gray-50 px-4">
      <div className="w-full max-w-md rounded-lg bg-white p-8 shadow">
        <h1 className="mb-6 text-2xl font-bold text-gray-900">
          Activate your account
        </h1>
        <p className="mb-4 text-sm text-gray-600">
          Please set a password to activate your account.
        </p>
        <PasswordForm
          onSubmit={handleSubmit}
          isLoading={isLoading}
          error={error}
          submitLabel="Activate Account"
        />
      </div>
    </div>
  )
}
