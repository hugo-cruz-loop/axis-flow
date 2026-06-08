import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { UserForm } from '@/features/users/UserForm'
import type { UserFormValues } from '@/features/users/UserForm'
import * as usersClient from '@/api/usersClient'

export function CreateUserPage() {
  const [error, setError] = useState<string | null>(null)
  const [isLoading, setIsLoading] = useState(false)
  const navigate = useNavigate()

  const handleSubmit = async (values: UserFormValues) => {
    setError(null)
    setIsLoading(true)
    try {
      await usersClient.createUser(values)
      navigate('/admin/users', { replace: true })
    } catch {
      setError('Failed to create user. Please try again.')
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-gray-50 px-4">
      <div className="w-full max-w-md rounded-lg bg-white p-8 shadow">
        <h1 className="mb-6 text-2xl font-bold text-gray-900">Create User</h1>
        <UserForm onSubmit={handleSubmit} isLoading={isLoading} error={error} />
      </div>
    </div>
  )
}
