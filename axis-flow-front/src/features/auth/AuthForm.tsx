import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'

const schema = z.object({
  email: z.string().email('Please enter a valid email address'),
  password: z.string().min(8, 'Password must be at least 8 characters'),
})

export type AuthFormValues = z.infer<typeof schema>

interface AuthFormProps {
  onSubmit: (values: AuthFormValues) => Promise<void>
  isLoading?: boolean
  error?: string | null
}

export function AuthForm({ onSubmit, isLoading, error }: AuthFormProps) {
  const {
    register,
    handleSubmit,
    formState: { errors },
    setFocus,
  } = useForm<AuthFormValues>({
    resolver: zodResolver(schema),
  })

  const handleFormSubmit = async (values: AuthFormValues) => {
    await onSubmit(values)
  }

  const handleInvalid = () => {
    // Focus first error field for accessibility
    if (errors.email) {
      setFocus('email')
    } else if (errors.password) {
      setFocus('password')
    }
  }

  return (
    <form
      onSubmit={handleSubmit(handleFormSubmit, handleInvalid)}
      noValidate
      className="space-y-4"
    >
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
          autoComplete="email"
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
          htmlFor="password"
          className="block text-sm font-medium text-gray-700"
        >
          Password
        </label>
        <input
          id="password"
          type="password"
          autoComplete="current-password"
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

      <button
        type="submit"
        disabled={isLoading}
        className="w-full rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50"
      >
        {isLoading ? 'Signing in...' : 'Sign in'}
      </button>
    </form>
  )
}
