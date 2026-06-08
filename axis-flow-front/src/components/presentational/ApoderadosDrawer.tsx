import { useEffect } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { X } from 'lucide-react'
import type { Apoderado } from '@/types/empresas'

const schema = z.object({
  nombre: z.string().min(2, 'Name must be at least 2 characters'),
  curp: z
    .string()
    .regex(
      /^[A-Z]{4}[0-9]{6}[HM][A-Z]{5}[0-9A-Z]{2}$/,
      'Invalid CURP format (18 characters)',
    ),
  rfc: z
    .string()
    .min(12, 'RFC must be 12-13 characters')
    .max(13, 'RFC must be 12-13 characters')
    .transform((v) => v.toUpperCase()),
  email: z.string().email('Valid email required'),
  telefono: z
    .string()
    .optional()
    .refine((v) => !v || /^\d{10}$/.test(v), 'Phone must be exactly 10 digits'),
})

type FormValues = z.infer<typeof schema>

interface ApoderadosDrawerProps {
  open: boolean
  onClose: () => void
  apoderado?: Apoderado | null
  onSubmit: (data: Omit<Apoderado, 'id' | 'empresa_id'>) => Promise<void>
  isSubmitting?: boolean
}

export function ApoderadosDrawer({
  open,
  onClose,
  apoderado,
  onSubmit,
  isSubmitting = false,
}: ApoderadosDrawerProps) {
  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<FormValues>({
    resolver: zodResolver(schema),
  })

  useEffect(() => {
    if (open) {
      reset(
        apoderado
          ? {
              nombre: apoderado.nombre,
              curp: apoderado.curp,
              rfc: apoderado.rfc,
              email: apoderado.email,
              telefono: apoderado.telefono ?? '',
            }
          : { nombre: '', curp: '', rfc: '', email: '', telefono: '' },
      )
    }
  }, [open, apoderado, reset])

  if (!open) return null

  const handleFormSubmit = async (data: FormValues) => {
    await onSubmit({
      nombre: data.nombre,
      curp: data.curp,
      rfc: data.rfc,
      email: data.email,
      telefono: data.telefono || undefined,
    })
  }

  return (
    <div className="fixed inset-0 z-50 flex justify-end">
      <div
        className="absolute inset-0 bg-slate-900/40 backdrop-blur-sm"
        onClick={onClose}
        aria-hidden="true"
      />
      <aside className="relative flex h-full w-96 flex-col bg-white shadow-xl">
        <div className="flex items-center justify-between border-b border-slate-200 px-6 py-4">
          <h2 className="text-lg font-semibold text-slate-900">
            {apoderado ? 'Edit Apoderado' : 'Add Apoderado'}
          </h2>
          <button
            onClick={onClose}
            className="rounded-lg p-1.5 text-slate-400 hover:bg-slate-100 hover:text-slate-600"
            aria-label="Close"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        <form
          onSubmit={(e) => void handleSubmit(handleFormSubmit)(e)}
          className="flex flex-1 flex-col gap-4 overflow-y-auto p-6"
        >
          <div>
            <label className="block text-sm font-medium text-slate-700">
              Name <span className="text-red-500">*</span>
            </label>
            <input
              {...register('nombre')}
              className="mt-1 h-9 w-full rounded-lg border border-slate-300 px-3 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
            />
            {errors.nombre && (
              <p className="mt-1 text-xs text-red-500">{errors.nombre.message}</p>
            )}
          </div>

          <div>
            <label className="block text-sm font-medium text-slate-700">
              CURP <span className="text-red-500">*</span>
            </label>
            <input
              {...register('curp')}
              className="mt-1 h-9 w-full rounded-lg border border-slate-300 px-3 font-mono text-sm uppercase focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
              maxLength={18}
            />
            {errors.curp && (
              <p className="mt-1 text-xs text-red-500">{errors.curp.message}</p>
            )}
          </div>

          <div>
            <label className="block text-sm font-medium text-slate-700">
              RFC <span className="text-red-500">*</span>
            </label>
            <input
              {...register('rfc')}
              className="mt-1 h-9 w-full rounded-lg border border-slate-300 px-3 font-mono text-sm uppercase focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
              maxLength={13}
            />
            {errors.rfc && (
              <p className="mt-1 text-xs text-red-500">{errors.rfc.message}</p>
            )}
          </div>

          <div>
            <label className="block text-sm font-medium text-slate-700">
              Email <span className="text-red-500">*</span>
            </label>
            <input
              {...register('email')}
              type="email"
              className="mt-1 h-9 w-full rounded-lg border border-slate-300 px-3 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
            />
            {errors.email && (
              <p className="mt-1 text-xs text-red-500">{errors.email.message}</p>
            )}
          </div>

          <div>
            <label className="block text-sm font-medium text-slate-700">Phone</label>
            <input
              {...register('telefono')}
              type="tel"
              className="mt-1 h-9 w-full rounded-lg border border-slate-300 px-3 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
              maxLength={10}
            />
            {errors.telefono && (
              <p className="mt-1 text-xs text-red-500">{errors.telefono.message}</p>
            )}
          </div>

          <div className="mt-auto flex gap-3 pt-4">
            <button
              type="button"
              onClick={onClose}
              className="flex-1 rounded-lg border border-slate-300 px-4 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={isSubmitting}
              className="flex-1 rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700 disabled:opacity-50"
            >
              {isSubmitting ? 'Saving…' : apoderado ? 'Update' : 'Create'}
            </button>
          </div>
        </form>
      </aside>
    </div>
  )
}
