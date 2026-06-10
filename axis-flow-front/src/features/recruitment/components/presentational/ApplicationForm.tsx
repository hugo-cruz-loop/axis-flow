import { useRef, useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { ApplicationSchema, type ApplicationInput } from '../../schemas/validation'
import { useApplyToJob } from '../../api/queries'

interface ApplicationFormProps {
  trabajoId: string
  onSuccess: () => void
}

export function ApplicationForm({ trabajoId, onSuccess }: ApplicationFormProps) {
  const [turnstileToken, setTurnstileToken] = useState<string>('')
  const fileInputRef = useRef<HTMLInputElement>(null)
  const { mutate: apply, isPending } = useApplyToJob()

  const {
    register,
    handleSubmit,
    formState: { errors },
    setValue,
    watch,
  } = useForm<ApplicationInput>({
    resolver: zodResolver(ApplicationSchema),
    defaultValues: { trabajo_id: trabajoId, turnstile_token: '' },
  })

  const cvFiles = watch('cv') as FileList | undefined
  const hasFile = cvFiles?.length === 1

  const onSubmit = (data: ApplicationInput) => {
    const formData = new FormData()
    formData.append('trabajo_id', data.trabajo_id)
    formData.append('nombre_completo', data.nombre_completo)
    formData.append('email', data.email)
    if (data.telefono) formData.append('telefono', data.telefono)
    formData.append('cv', (data.cv as FileList)[0])
    formData.append('turnstile_token', data.turnstile_token)

    apply(formData, { onSuccess })
  }

  // In a real integration, the Turnstile widget calls this callback with the token.
  // Here we expose a setter so tests and future integration can inject the value.
  const handleTurnstileCallback = (token: string) => {
    setTurnstileToken(token)
    setValue('turnstile_token', token, { shouldValidate: true })
  }

  return (
    <form onSubmit={handleSubmit(onSubmit)} className="space-y-4" noValidate>
      <input type="hidden" {...register('trabajo_id')} />
      <input type="hidden" {...register('turnstile_token')} />

      <div>
        <label htmlFor="nombre_completo" className="mb-1 block text-sm font-medium text-gray-700">
          Full name
        </label>
        <input
          id="nombre_completo"
          type="text"
          {...register('nombre_completo')}
          className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
        />
        {errors.nombre_completo && (
          <p className="mt-1 text-xs text-red-600">{errors.nombre_completo.message}</p>
        )}
      </div>

      <div>
        <label htmlFor="email" className="mb-1 block text-sm font-medium text-gray-700">
          Email
        </label>
        <input
          id="email"
          type="email"
          {...register('email')}
          className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
        />
        {errors.email && (
          <p className="mt-1 text-xs text-red-600">{errors.email.message}</p>
        )}
      </div>

      <div>
        <label htmlFor="telefono" className="mb-1 block text-sm font-medium text-gray-700">
          Phone (optional)
        </label>
        <input
          id="telefono"
          type="tel"
          {...register('telefono')}
          className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
        />
        {errors.telefono && (
          <p className="mt-1 text-xs text-red-600">{errors.telefono.message}</p>
        )}
      </div>

      <div>
        <label className="mb-1 block text-sm font-medium text-gray-700">CV (PDF or DOCX, max 5MB)</label>
        <div
          className={`flex cursor-pointer flex-col items-center justify-center rounded-md border-2 border-dashed px-4 py-6 transition-colors ${
            hasFile ? 'border-blue-400 bg-blue-50' : 'border-gray-300 hover:border-blue-400'
          }`}
          onClick={() => fileInputRef.current?.click()}
          onDragOver={(e) => e.preventDefault()}
          onDrop={(e) => {
            e.preventDefault()
            const files = e.dataTransfer.files
            if (files.length) {
              setValue('cv', files, { shouldValidate: true })
            }
          }}
        >
          <input
            ref={fileInputRef}
            type="file"
            accept=".pdf,.docx,application/pdf,application/vnd.openxmlformats-officedocument.wordprocessingml.document"
            className="hidden"
            onChange={(e) => {
              if (e.target.files) {
                setValue('cv', e.target.files, { shouldValidate: true })
              }
            }}
          />
          {hasFile ? (
            <p className="text-sm text-blue-700">{(cvFiles as FileList)[0].name}</p>
          ) : (
            <p className="text-sm text-gray-500">Drag & drop or click to upload</p>
          )}
        </div>
        {errors.cv && (
          <p className="mt-1 text-xs text-red-600">{String(errors.cv.message)}</p>
        )}
      </div>

      {/* Cloudflare Turnstile placeholder */}
      <div id="cf-turnstile" data-testid="cf-turnstile" />
      {errors.turnstile_token && (
        <p className="text-xs text-red-600">{errors.turnstile_token.message}</p>
      )}

      <button
        type="submit"
        disabled={isPending || !turnstileToken}
        className="w-full rounded-md bg-blue-600 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-50"
      >
        {isPending ? 'Submitting…' : 'Submit application'}
      </button>
    </form>
  )
}
