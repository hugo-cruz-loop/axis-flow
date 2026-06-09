import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { useNavigate } from 'react-router-dom'
import { X } from 'lucide-react'
import { useCreateEmpleado } from '@/hooks/api/useEmpleados'
import { cn } from '@/lib/utils'

const STORAGE_KEY = 'axis-flow:employee-wizard-draft'

const step1Schema = z.object({
  nombre: z.string().min(2, 'Min 2 characters').max(100, 'Max 100 characters'),
  apellido_paterno: z.string().min(2, 'Min 2 characters').max(100, 'Max 100 characters'),
  apellido_materno: z.string().optional(),
  id_empleado: z.string().min(1, 'Employee ID is required'),
  email: z.string().email('Must be a valid email'),
})

type Step1Values = z.infer<typeof step1Schema>

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

interface EmployeeWizardProps {
  empresaId: number
  onClose: () => void
}

const STEPS = ['Basic Data', 'Confirmation']

export function EmployeeWizard({ empresaId, onClose }: EmployeeWizardProps) {
  const navigate = useNavigate()
  const [step, setStep] = useState(1)
  const draft = loadDraft()
  const [submitError, setSubmitError] = useState<string | null>(null)

  const { mutate: createEmpleado, isPending } = useCreateEmpleado()

  const form = useForm<Step1Values>({
    resolver: zodResolver(step1Schema),
    defaultValues: {
      nombre: '',
      apellido_paterno: '',
      apellido_materno: '',
      id_empleado: '',
      email: '',
      ...draft.step1,
    },
  })

  const handleStep1Next = form.handleSubmit((values) => {
    saveDraft({ step1: values })
    setStep(2)
  })

  const handleSubmit = () => {
    const values = form.getValues()
    setSubmitError(null)
    createEmpleado(
      {
        empresa_id: empresaId,
        nombre: values.nombre,
        apellido_paterno: values.apellido_paterno,
        apellido_materno: values.apellido_materno || undefined,
        id_empleado: values.id_empleado,
        email: values.email,
      },
      {
        onSuccess: (data) => {
          clearDraft()
          onClose()
          navigate(`/dashboard/empleados/${data.num_empleado}`)
        },
        onError: (err) => {
          const message =
            err && typeof err === 'object' && 'response' in err
              ? ((err as { response?: { data?: { message?: string } } }).response?.data?.message ??
                'Error creating employee')
              : 'Error creating employee'
          setSubmitError(message)
        },
      },
    )
  }

  const values = form.getValues()

  return (
    <div className="flex flex-col gap-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold text-gray-900">New Employee</h2>
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
                i + 1 <= step ? 'bg-indigo-600 text-white' : 'bg-gray-200 text-gray-500',
              )}
            >
              {i + 1}
            </div>
            <span
              className={cn(
                'text-sm',
                i + 1 === step ? 'font-medium text-gray-900' : 'text-gray-400',
              )}
            >
              {label}
            </span>
            {i < STEPS.length - 1 && <span className="text-gray-300">›</span>}
          </div>
        ))}
      </div>

      {/* Step 1 — Basic Data */}
      {step === 1 && (
        <form onSubmit={handleStep1Next} className="flex flex-col gap-4">
          <div>
            <label className="mb-1 block text-sm font-medium text-gray-700">
              First Name <span className="text-red-500">*</span>
            </label>
            <input
              {...form.register('nombre')}
              className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none"
              placeholder="First name"
            />
            {form.formState.errors.nombre && (
              <p className="mt-1 text-xs text-red-600">{form.formState.errors.nombre.message}</p>
            )}
          </div>
          <div>
            <label className="mb-1 block text-sm font-medium text-gray-700">
              Paternal Last Name <span className="text-red-500">*</span>
            </label>
            <input
              {...form.register('apellido_paterno')}
              className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none"
              placeholder="Paternal last name"
            />
            {form.formState.errors.apellido_paterno && (
              <p className="mt-1 text-xs text-red-600">
                {form.formState.errors.apellido_paterno.message}
              </p>
            )}
          </div>
          <div>
            <label className="mb-1 block text-sm font-medium text-gray-700">
              Maternal Last Name
            </label>
            <input
              {...form.register('apellido_materno')}
              className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none"
              placeholder="Maternal last name (optional)"
            />
          </div>
          <div>
            <label className="mb-1 block text-sm font-medium text-gray-700">
              Employee ID <span className="text-red-500">*</span>
            </label>
            <input
              {...form.register('id_empleado')}
              className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none"
              placeholder="e.g. EMP-001"
            />
            {form.formState.errors.id_empleado && (
              <p className="mt-1 text-xs text-red-600">
                {form.formState.errors.id_empleado.message}
              </p>
            )}
          </div>
          <div>
            <label className="mb-1 block text-sm font-medium text-gray-700">
              Email <span className="text-red-500">*</span>
            </label>
            <input
              type="email"
              {...form.register('email')}
              className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none"
              placeholder="employee@example.com"
            />
            {form.formState.errors.email && (
              <p className="mt-1 text-xs text-red-600">{form.formState.errors.email.message}</p>
            )}
          </div>
          <div className="flex justify-end gap-2 pt-2">
            <button
              type="button"
              onClick={onClose}
              className="rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50"
            >
              Cancel
            </button>
            <button
              type="submit"
              className="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700"
            >
              Next
            </button>
          </div>
        </form>
      )}

      {/* Step 2 — Confirmation */}
      {step === 2 && (
        <div className="flex flex-col gap-4">
          <div className="rounded-lg border border-gray-200 bg-gray-50 p-4">
            <h3 className="mb-3 text-sm font-semibold text-gray-700">Review employee data</h3>
            <dl className="grid grid-cols-2 gap-x-4 gap-y-2 text-sm">
              <dt className="text-gray-500">First Name</dt>
              <dd className="font-medium text-gray-900">{values.nombre}</dd>
              <dt className="text-gray-500">Paternal Last Name</dt>
              <dd className="font-medium text-gray-900">{values.apellido_paterno}</dd>
              {values.apellido_materno && (
                <>
                  <dt className="text-gray-500">Maternal Last Name</dt>
                  <dd className="font-medium text-gray-900">{values.apellido_materno}</dd>
                </>
              )}
              <dt className="text-gray-500">Employee ID</dt>
              <dd className="font-medium text-gray-900">{values.id_empleado}</dd>
              <dt className="text-gray-500">Email</dt>
              <dd className="font-medium text-gray-900">{values.email}</dd>
              <dt className="text-gray-500">Company ID</dt>
              <dd className="font-medium text-gray-900">{empresaId}</dd>
            </dl>
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
              Back
            </button>
            <button
              type="button"
              onClick={handleSubmit}
              disabled={isPending}
              className="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700 disabled:cursor-not-allowed disabled:opacity-50"
            >
              {isPending ? 'Creating...' : 'Create Employee'}
            </button>
          </div>
        </div>
      )}
    </div>
  )
}
