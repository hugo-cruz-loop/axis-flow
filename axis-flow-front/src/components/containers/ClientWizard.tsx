import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { useNavigate } from 'react-router-dom'
import { X } from 'lucide-react'
import { useCreateCliente } from '@/hooks/api/useClientes'
import { cn } from '@/lib/utils'

const STORAGE_KEY = 'axis-flow:client-wizard-draft'

// Step 1 schema
const step1Schema = z.object({
  nombre_comercial: z.string().min(2, 'Min 2 characters').max(150, 'Max 150 characters'),
  razon_social: z.string().max(150).optional(),
  fecha_inicio_contrato: z.string().optional(),
})

// Step 2 schema
const step2Schema = z.object({
  representante_id: z.string().uuid('Must be a valid UUID'),
})

type Step1Values = z.infer<typeof step1Schema>
type Step2Values = z.infer<typeof step2Schema>

interface WizardDraft {
  step1: Partial<Step1Values>
}

function loadDraft(): WizardDraft {
  try {
    const raw = sessionStorage.getItem(STORAGE_KEY)
    if (raw) return JSON.parse(raw) as WizardDraft
  } catch {
    // ignore
  }
  return { step1: {} }
}

function saveDraft(draft: WizardDraft) {
  try {
    sessionStorage.setItem(STORAGE_KEY, JSON.stringify(draft))
  } catch {
    // ignore
  }
}

function clearDraft() {
  try {
    sessionStorage.removeItem(STORAGE_KEY)
  } catch {
    // ignore
  }
}

interface ClientWizardProps {
  empresaId: number
  onClose: () => void
}

const STEPS = ['Datos del Cliente', 'Representante']

export function ClientWizard({ empresaId, onClose }: ClientWizardProps) {
  const navigate = useNavigate()
  const [step, setStep] = useState(1)
  const draft = loadDraft()
  const [submitError, setSubmitError] = useState<string | null>(null)

  const { mutate: createCliente, isPending } = useCreateCliente()

  const form1 = useForm<Step1Values>({
    resolver: zodResolver(step1Schema),
    defaultValues: { nombre_comercial: '', razon_social: '', fecha_inicio_contrato: '', ...draft.step1 },
  })

  const form2 = useForm<Step2Values>({
    resolver: zodResolver(step2Schema),
    defaultValues: { representante_id: '' },
  })

  const handleStep1Next = form1.handleSubmit((values) => {
    saveDraft({ step1: values })
    setStep(2)
  })

  const handleSubmit = form2.handleSubmit((values) => {
    const step1Values = form1.getValues()
    setSubmitError(null)
    createCliente(
      {
        empresa_id: empresaId,
        representante_id: values.representante_id,
        nombre_comercial: step1Values.nombre_comercial,
        razon_social: step1Values.razon_social || undefined,
        fecha_inicio_contrato: step1Values.fecha_inicio_contrato || undefined,
      },
      {
        onSuccess: (data) => {
          clearDraft()
          onClose()
          navigate(`/dashboard/clientes/${data.id}`)
        },
        onError: (err) => {
          const message =
            err && typeof err === 'object' && 'response' in err
              ? ((err as { response?: { data?: { message?: string } } }).response?.data?.message ?? 'Error creating client')
              : 'Error creating client'
          setSubmitError(message)
        },
      },
    )
  })

  return (
    <div className="flex flex-col gap-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold text-gray-900">Nuevo Cliente</h2>
        <button onClick={onClose} className="rounded p-1 text-gray-400 hover:text-gray-600">
          <X className="h-5 w-5" />
        </button>
      </div>

      {/* Step indicator */}
      <div className="flex gap-2">
        {STEPS.map((label, i) => (
          <div key={label} className="flex items-center gap-2">
            <div
              className={cn(
                'flex h-6 w-6 items-center justify-center rounded-full text-xs font-semibold',
                i + 1 <= step
                  ? 'bg-indigo-600 text-white'
                  : 'bg-gray-200 text-gray-500',
              )}
            >
              {i + 1}
            </div>
            <span className={cn('text-sm', i + 1 === step ? 'font-medium text-gray-900' : 'text-gray-400')}>
              {label}
            </span>
            {i < STEPS.length - 1 && <span className="text-gray-300">›</span>}
          </div>
        ))}
      </div>

      {/* Step 1 */}
      {step === 1 && (
        <form onSubmit={handleStep1Next} className="flex flex-col gap-4">
          <div>
            <label className="mb-1 block text-sm font-medium text-gray-700">
              Nombre Comercial <span className="text-red-500">*</span>
            </label>
            <input
              {...form1.register('nombre_comercial')}
              className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none"
              placeholder="Nombre comercial del cliente"
            />
            {form1.formState.errors.nombre_comercial && (
              <p className="mt-1 text-xs text-red-600">{form1.formState.errors.nombre_comercial.message}</p>
            )}
          </div>
          <div>
            <label className="mb-1 block text-sm font-medium text-gray-700">Razón Social</label>
            <input
              {...form1.register('razon_social')}
              className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none"
              placeholder="Razón social (opcional)"
            />
          </div>
          <div>
            <label className="mb-1 block text-sm font-medium text-gray-700">Fecha Inicio de Contrato</label>
            <input
              type="date"
              {...form1.register('fecha_inicio_contrato')}
              className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none"
            />
          </div>
          <div className="flex justify-end gap-2 pt-2">
            <button
              type="button"
              onClick={onClose}
              className="rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50"
            >
              Cancelar
            </button>
            <button
              type="submit"
              className="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700"
            >
              Siguiente
            </button>
          </div>
        </form>
      )}

      {/* Step 2 */}
      {step === 2 && (
        <form onSubmit={handleSubmit} className="flex flex-col gap-4">
          <div>
            <label className="mb-1 block text-sm font-medium text-gray-700">
              ID del Representante (UUID) <span className="text-red-500">*</span>
            </label>
            <input
              {...form2.register('representante_id')}
              className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none"
              placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
            />
            {form2.formState.errors.representante_id && (
              <p className="mt-1 text-xs text-red-600">{form2.formState.errors.representante_id.message}</p>
            )}
          </div>
          {submitError && (
            <p className="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-600">{submitError}</p>
          )}
          <div className="flex justify-end gap-2 pt-2">
            <button
              type="button"
              onClick={() => setStep(1)}
              className="rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50"
            >
              Atrás
            </button>
            <button
              type="submit"
              disabled={isPending}
              className="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700 disabled:cursor-not-allowed disabled:opacity-50"
            >
              {isPending ? 'Creando...' : 'Crear Cliente'}
            </button>
          </div>
        </form>
      )}
    </div>
  )
}
