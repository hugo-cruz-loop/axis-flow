import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'

const schema = z
  .object({
    password: z.string().min(8, 'Password must be at least 8 characters'),
    confirmPassword: z.string(),
  })
  .refine((data) => data.password === data.confirmPassword, {
    message: 'Passwords do not match',
    path: ['confirmPassword'],
  })

export type PasswordFormValues = z.infer<typeof schema>

interface PasswordFormProps {
  onSubmit: (values: PasswordFormValues) => Promise<void>
  isLoading?: boolean
  error?: string | null
  submitLabel?: string
}

export function PasswordForm({
  onSubmit,
  isLoading,
  error,
  submitLabel = 'Set Password',
}: PasswordFormProps) {
  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<PasswordFormValues>({
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
        <label
          htmlFor="password"
          className="block text-sm font-medium text-gray-700"
        >
          New Password
        </label>
        <input
          id="password"
          type="password"
          autoComplete="new-password"
          aria-describedby={errors.password ? 'password-error' : undefined}
          {...register('password')}
          className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
        />
        {errors.password && (
          <p
            id="password-error"
            role="alert"
            aria-live="polite"
            className="mt-1 text-xs text-red-600"
          >
            {errors.password.message}
          </p>
        )}
      </div>

      <div>
        <label
          htmlFor="confirmPassword"
          className="block text-sm font-medium text-gray-700"
        >
          Confirm Password
        </label>
        <input
          id="confirmPassword"
          type="password"
          autoComplete="new-password"
          aria-describedby={
            errors.confirmPassword ? 'confirm-password-error' : undefined
          }
          {...register('confirmPassword')}
          className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
        />
        {errors.confirmPassword && (
          <p
            id="confirm-password-error"
            role="alert"
            aria-live="polite"
            className="mt-1 text-xs text-red-600"
          >
            {errors.confirmPassword.message}
          </p>
        )}
      </div>

      <button
        type="submit"
        disabled={isLoading}
        className="w-full rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50"
      >
        {isLoading ? 'Saving...' : submitLabel}
      </button>
    </form>
  )
}
