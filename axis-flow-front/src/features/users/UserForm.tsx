import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
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
  role_code: z.enum(USER_ROLES as [UserRole, ...UserRole[]]).refine(
    (val) => USER_ROLES.includes(val as UserRole),
    { message: 'Please select a role' },
  ),
  url_front: z.string().optional(),
})

export type UserFormValues = z.infer<typeof schema>

interface UserFormProps {
  onSubmit: (values: UserFormValues) => Promise<void>
  isLoading?: boolean
  error?: string | null
}

export function UserForm({ onSubmit, isLoading, error }: UserFormProps) {
  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<UserFormValues>({
    resolver: zodResolver(schema),
  })

  return (
    <form onSubmit={handleSubmit(onSubmit)} noValidate className="space-y-4">
      {error && (
        <div
          role="alert"
          aria-live="polite"
          className="rounded bg-red-50 px-4 py-3 text-sm text-red-600"
        >
          {error}
        </div>
      )}

      <div>
        <label htmlFor="email" className="block text-sm font-medium text-gray-700">
          Email
        </label>
        <input
          id="email"
          type="email"
          aria-describedby={errors.email ? 'email-error' : undefined}
          {...register('email')}
          className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
        />
        {errors.email && (
          <p
            id="email-error"
            role="alert"
            aria-live="polite"
            className="mt-1 text-xs text-red-600"
          >
            {errors.email.message}
          </p>
        )}
      </div>

      <div>
        <label
          htmlFor="first_name"
          className="block text-sm font-medium text-gray-700"
        >
          First Name
        </label>
        <input
          id="first_name"
          type="text"
          aria-describedby={errors.first_name ? 'first-name-error' : undefined}
          {...register('first_name')}
          className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
        />
        {errors.first_name && (
          <p
            id="first-name-error"
            role="alert"
            aria-live="polite"
            className="mt-1 text-xs text-red-600"
          >
            {errors.first_name.message}
          </p>
        )}
      </div>

      <div>
        <label
          htmlFor="last_name"
          className="block text-sm font-medium text-gray-700"
        >
          Last Name
        </label>
        <input
          id="last_name"
          type="text"
          aria-describedby={errors.last_name ? 'last-name-error' : undefined}
          {...register('last_name')}
          className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
        />
        {errors.last_name && (
          <p
            id="last-name-error"
            role="alert"
            aria-live="polite"
            className="mt-1 text-xs text-red-600"
          >
            {errors.last_name.message}
          </p>
        )}
      </div>

      <div>
        <label
          htmlFor="role_code"
          className="block text-sm font-medium text-gray-700"
        >
          Role
        </label>
        <select
          id="role_code"
          aria-describedby={errors.role_code ? 'role-error' : undefined}
          {...register('role_code')}
          className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
        >
          <option value="">Select a role</option>
          {USER_ROLES.map((role) => (
            <option key={role} value={role}>
              {role}
            </option>
          ))}
        </select>
        {errors.role_code && (
          <p
            id="role-error"
            role="alert"
            aria-live="polite"
            className="mt-1 text-xs text-red-600"
          >
            {errors.role_code.message}
          </p>
        )}
      </div>

      <button
        type="submit"
        disabled={isLoading}
        className="w-full rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50"
      >
        {isLoading ? 'Creating...' : 'Create User'}
      </button>
    </form>
  )
}
