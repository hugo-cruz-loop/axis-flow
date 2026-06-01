import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { ArrowLeft, CheckCircle2 } from 'lucide-react'
import * as usersClient from '@/api/usersClient'
import { DashboardLayout } from '@/layouts/DashboardLayout'
import type { UserRole } from '@/types/users'

const USER_ROLES: UserRole[] = [
  'ADMIN_CHECK_ON',
  'CLIENTE',
  'EMPLEADO',
  'SUPERVISOR',
  'OPERACIONES',
  'GESTOR',
  'ADMINISTRADOR',
  'RH',
]

const schema = z.object({
  email: z.string().email('Please enter a valid email address'),
  first_name: z.string().min(1, 'First name is required'),
  last_name: z.string().min(1, 'Last name is required'),
  role_code: z.enum(USER_ROLES as [UserRole, ...UserRole[]]),
  url_front: z.string().optional(),
})

type FormValues = z.infer<typeof schema>

function FieldError({ message }: { message: string }) {
  return (
    <p role="alert" aria-live="polite" className="mt-1 text-xs text-red-600">
      {message}
    </p>
  )
}

export function DashboardCreateUserPage() {
  const navigate = useNavigate()
  const [serverError, setServerError] = useState<string | null>(null)
  const [success, setSuccess] = useState(false)
  const [isSubmitting, setIsSubmitting] = useState(false)

  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<FormValues>({ resolver: zodResolver(schema) })

  const onSubmit = async (values: FormValues) => {
    setServerError(null)
    setIsSubmitting(true)
    try {
      await usersClient.createUser(values)
      setSuccess(true)
      setTimeout(() => navigate('/dashboard/users', { replace: true }), 1500)
    } catch {
      setServerError('Failed to create user. Please try again.')
    } finally {
      setIsSubmitting(false)
    }
  }

  const inputClass =
    'mt-1 block w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm text-slate-900 placeholder-slate-400 focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500'
  const labelClass = 'block text-sm font-medium text-slate-700'

  return (
    <DashboardLayout>
      <div className="mx-auto max-w-lg">
        <button
          onClick={() => navigate('/dashboard/users')}
          className="mb-4 flex items-center gap-1.5 text-sm font-medium text-slate-600 hover:text-slate-900"
        >
          <ArrowLeft className="h-4 w-4" />
          Users
        </button>

        <div className="rounded-xl border border-slate-200 bg-white p-6 shadow-sm">
          <h1 className="mb-6 text-xl font-bold text-slate-900">Create User</h1>

          {success && (
            <div className="mb-4 flex items-center gap-2 rounded-lg bg-green-50 px-4 py-3 text-sm text-green-700">
              <CheckCircle2 className="h-4 w-4 shrink-0" />
              User created successfully. Redirecting…
            </div>
          )}

          {serverError && (
            <div
              role="alert"
              aria-live="polite"
              className="mb-4 rounded-lg bg-red-50 px-4 py-3 text-sm text-red-600"
            >
              {serverError}
            </div>
          )}

          <form onSubmit={handleSubmit(onSubmit)} noValidate className="space-y-4">
            <div className="grid grid-cols-2 gap-4">
              <div>
                <label htmlFor="first_name" className={labelClass}>
                  First Name
                </label>
                <input
                  id="first_name"
                  type="text"
                  {...register('first_name')}
                  className={inputClass}
                  aria-describedby={errors.first_name ? 'fn-error' : undefined}
                />
                {errors.first_name && (
                  <FieldError message={errors.first_name.message!} />
                )}
              </div>
              <div>
                <label htmlFor="last_name" className={labelClass}>
                  Last Name
                </label>
                <input
                  id="last_name"
                  type="text"
                  {...register('last_name')}
                  className={inputClass}
                  aria-describedby={errors.last_name ? 'ln-error' : undefined}
                />
                {errors.last_name && (
                  <FieldError message={errors.last_name.message!} />
                )}
              </div>
            </div>

            <div>
              <label htmlFor="email" className={labelClass}>
                Email
              </label>
              <input
                id="email"
                type="email"
                {...register('email')}
                className={inputClass}
                aria-describedby={errors.email ? 'email-error' : undefined}
              />
              {errors.email && (
                <FieldError message={errors.email.message!} />
              )}
            </div>

            <div>
              <label htmlFor="role_code" className={labelClass}>
                Role
              </label>
              <select
                id="role_code"
                {...register('role_code')}
                className={inputClass}
                aria-describedby={errors.role_code ? 'role-error' : undefined}
              >
                <option value="">Select a role</option>
                {USER_ROLES.map((r) => (
                  <option key={r} value={r}>
                    {r}
                  </option>
                ))}
              </select>
              {errors.role_code && (
                <FieldError message={errors.role_code.message!} />
              )}
            </div>

            <div>
              <label htmlFor="url_front" className={labelClass}>
                Frontend URL{' '}
                <span className="font-normal text-slate-400">(optional)</span>
              </label>
              <input
                id="url_front"
                type="url"
                {...register('url_front')}
                className={inputClass}
                placeholder="https://example.com"
              />
            </div>

            <button
              type="submit"
              disabled={isSubmitting || success}
              className="mt-2 w-full rounded-lg bg-indigo-600 px-4 py-2.5 text-sm font-semibold text-white hover:bg-indigo-500 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-60"
            >
              {isSubmitting ? 'Creating…' : 'Create User'}
            </button>
          </form>
        </div>
      </div>
    </DashboardLayout>
  )
}
