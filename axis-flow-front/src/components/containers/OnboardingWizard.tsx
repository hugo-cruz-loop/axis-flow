import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { listSubscriptionPlans } from '@/api/catalogosClient'
import { registerEmpresa, createCheckoutSession } from '@/api/empresasClient'
import { StepPlanSelection } from '@/components/presentational/StepPlanSelection'

const STORAGE_KEY = 'axis-flow:onboarding-wizard'

interface WizardState {
  selectedPlanId: number | null
  companyData: Partial<CompanyFormValues>
}

const companySchema = z.object({
  nombre: z.string().min(3, 'Min 3 characters').max(150, 'Max 150 characters'),
  direccion: z.string().min(5, 'Min 5 characters').max(255, 'Max 255 characters'),
  telefono: z.string().regex(/^\d{10}$/, 'Must be exactly 10 digits'),
  representante_email: z.string().email('Valid email required'),
  representante_nombre: z.string().min(2, 'Min 2 characters'),
  representante_apellido_paterno: z.string().min(2, 'Min 2 characters'),
  representante_apellido_materno: z.string().optional(),
})

type CompanyFormValues = z.infer<typeof companySchema>

function loadState(): WizardState {
  try {
    const raw = sessionStorage.getItem(STORAGE_KEY)
    if (raw) return JSON.parse(raw) as WizardState
  } catch {
    // ignore
  }
  return { selectedPlanId: null, companyData: {} }
}

function saveState(state: WizardState) {
  try {
    sessionStorage.setItem(STORAGE_KEY, JSON.stringify(state))
  } catch {
    // ignore
  }
}

const STEPS = ['Plan', 'Company', 'Payment']

export function OnboardingWizard() {
  const [step, setStep] = useState(1)
  const [wizardState, setWizardState] = useState<WizardState>(loadState)
  const [submitting, setSubmitting] = useState(false)
  const [submitError, setSubmitError] = useState<string | null>(null)

  const { data: plans, isLoading: plansLoading } = useQuery({
    queryKey: ['planes'],
    queryFn: listSubscriptionPlans,
    staleTime: 5 * 60 * 1000,
  })

  const {
    register,
    handleSubmit,
    formState: { errors },
    getValues,
  } = useForm<CompanyFormValues>({
    resolver: zodResolver(companySchema),
    defaultValues: {
      nombre: wizardState.companyData.nombre ?? '',
      direccion: wizardState.companyData.direccion ?? '',
      telefono: wizardState.companyData.telefono ?? '',
      representante_email: wizardState.companyData.representante_email ?? '',
      representante_nombre: wizardState.companyData.representante_nombre ?? '',
      representante_apellido_paterno:
        wizardState.companyData.representante_apellido_paterno ?? '',
      representante_apellido_materno:
        wizardState.companyData.representante_apellido_materno ?? '',
    },
  })

  const selectedPlan = plans?.find((p) => p.id === wizardState.selectedPlanId)

  const updateState = (patch: Partial<WizardState>) => {
    const next = { ...wizardState, ...patch }
    setWizardState(next)
    saveState(next)
  }

  const handlePlanSelect = (planId: number) => {
    updateState({ selectedPlanId: planId })
  }

  const handleCompanySubmit = (data: CompanyFormValues) => {
    updateState({ companyData: data })
    setStep(3)
  }

  const handlePayment = async () => {
    const data = getValues()
    if (!wizardState.selectedPlanId) return
    setSubmitting(true)
    setSubmitError(null)
    try {
      const regResult = await registerEmpresa({
        nombre: data.nombre,
        direccion: data.direccion,
        telefono: data.telefono,
        plan_id: wizardState.selectedPlanId,
        representante: {
          email: data.representante_email,
          nombre: data.representante_nombre,
          apellido_paterno: data.representante_apellido_paterno,
          apellido_materno: data.representante_apellido_materno || undefined,
        },
      })
      const checkoutResult = await createCheckoutSession({
        plan_id: wizardState.selectedPlanId,
        clave_pago: regResult.data.clave_pago,
        success_url: `${window.location.origin}/onboarding/success`,
        cancel_url: `${window.location.origin}/onboarding/cancel`,
      })
      sessionStorage.removeItem(STORAGE_KEY)
      window.location.href = checkoutResult.data.checkout_url
    } catch {
      setSubmitError('There was an error processing your request. Please try again.')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="mx-auto max-w-2xl">
      {/* Step indicator */}
      <div className="mb-8 flex items-center justify-center gap-2">
        {STEPS.map((label, idx) => {
          const n = idx + 1
          const active = step === n
          const done = step > n
          return (
            <div key={label} className="flex items-center gap-2">
              <div className="flex flex-col items-center">
                <div
                  className={`flex h-8 w-8 items-center justify-center rounded-full text-sm font-bold transition-colors ${
                    done
                      ? 'bg-indigo-600 text-white'
                      : active
                        ? 'bg-indigo-600 text-white ring-4 ring-indigo-200'
                        : 'bg-slate-200 text-slate-500'
                  }`}
                >
                  {n}
                </div>
                <span className="mt-1 text-xs text-slate-500">{label}</span>
              </div>
              {idx < STEPS.length - 1 && (
                <div
                  className={`mb-4 h-0.5 w-12 transition-colors ${done ? 'bg-indigo-600' : 'bg-slate-200'}`}
                />
              )}
            </div>
          )
        })}
      </div>

      <div className="rounded-xl border border-slate-200 bg-white p-8 shadow-sm">
        {/* Step 1 — Plan Selection */}
        {step === 1 && (
          <div>
            <h2 className="mb-6 text-xl font-bold text-slate-900">Choose a Plan</h2>
            <StepPlanSelection
              plans={plans ?? []}
              selectedPlanId={wizardState.selectedPlanId}
              onSelect={handlePlanSelect}
              isLoading={plansLoading}
            />
            <div className="mt-6 flex justify-end">
              <button
                type="button"
                disabled={wizardState.selectedPlanId === null}
                onClick={() => setStep(2)}
                className="rounded-lg bg-indigo-600 px-6 py-2 text-sm font-medium text-white hover:bg-indigo-700 disabled:cursor-not-allowed disabled:opacity-40"
              >
                Next
              </button>
            </div>
          </div>
        )}

        {/* Step 2 — Company Data */}
        {step === 2 && (
          <form onSubmit={(e) => void handleSubmit(handleCompanySubmit)(e)}>
            <h2 className="mb-6 text-xl font-bold text-slate-900">Company Information</h2>
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <div className="sm:col-span-2">
                <label className="block text-sm font-medium text-slate-700">
                  Company Name <span className="text-red-500">*</span>
                </label>
                <input
                  {...register('nombre')}
                  className="mt-1 h-9 w-full rounded-lg border border-slate-300 px-3 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
                />
                {errors.nombre && (
                  <p className="mt-1 text-xs text-red-500">{errors.nombre.message}</p>
                )}
              </div>

              <div className="sm:col-span-2">
                <label className="block text-sm font-medium text-slate-700">
                  Address <span className="text-red-500">*</span>
                </label>
                <input
                  {...register('direccion')}
                  className="mt-1 h-9 w-full rounded-lg border border-slate-300 px-3 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
                />
                {errors.direccion && (
                  <p className="mt-1 text-xs text-red-500">{errors.direccion.message}</p>
                )}
              </div>

              <div>
                <label className="block text-sm font-medium text-slate-700">
                  Phone <span className="text-red-500">*</span>
                </label>
                <input
                  {...register('telefono')}
                  type="tel"
                  maxLength={10}
                  className="mt-1 h-9 w-full rounded-lg border border-slate-300 px-3 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
                />
                {errors.telefono && (
                  <p className="mt-1 text-xs text-red-500">{errors.telefono.message}</p>
                )}
              </div>

              <div>
                <label className="block text-sm font-medium text-slate-700">
                  Representative Email <span className="text-red-500">*</span>
                </label>
                <input
                  {...register('representante_email')}
                  type="email"
                  className="mt-1 h-9 w-full rounded-lg border border-slate-300 px-3 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
                />
                {errors.representante_email && (
                  <p className="mt-1 text-xs text-red-500">
                    {errors.representante_email.message}
                  </p>
                )}
              </div>

              <div>
                <label className="block text-sm font-medium text-slate-700">
                  Representative First Name <span className="text-red-500">*</span>
                </label>
                <input
                  {...register('representante_nombre')}
                  className="mt-1 h-9 w-full rounded-lg border border-slate-300 px-3 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
                />
                {errors.representante_nombre && (
                  <p className="mt-1 text-xs text-red-500">
                    {errors.representante_nombre.message}
                  </p>
                )}
              </div>

              <div>
                <label className="block text-sm font-medium text-slate-700">
                  Paternal Surname <span className="text-red-500">*</span>
                </label>
                <input
                  {...register('representante_apellido_paterno')}
                  className="mt-1 h-9 w-full rounded-lg border border-slate-300 px-3 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
                />
                {errors.representante_apellido_paterno && (
                  <p className="mt-1 text-xs text-red-500">
                    {errors.representante_apellido_paterno.message}
                  </p>
                )}
              </div>

              <div>
                <label className="block text-sm font-medium text-slate-700">
                  Maternal Surname
                </label>
                <input
                  {...register('representante_apellido_materno')}
                  className="mt-1 h-9 w-full rounded-lg border border-slate-300 px-3 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
                />
              </div>
            </div>

            <div className="mt-6 flex justify-between">
              <button
                type="button"
                onClick={() => setStep(1)}
                className="rounded-lg border border-slate-300 px-6 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50"
              >
                Back
              </button>
              <button
                type="submit"
                className="rounded-lg bg-indigo-600 px-6 py-2 text-sm font-medium text-white hover:bg-indigo-700"
              >
                Next
              </button>
            </div>
          </form>
        )}

        {/* Step 3 — Payment */}
        {step === 3 && (
          <div>
            <h2 className="mb-6 text-xl font-bold text-slate-900">Review & Pay</h2>
            <div className="mb-6 rounded-xl border border-slate-200 bg-slate-50 p-4">
              <div className="flex justify-between text-sm">
                <span className="text-slate-500">Company</span>
                <span className="font-medium text-slate-900">
                  {wizardState.companyData.nombre}
                </span>
              </div>
              <div className="mt-2 flex justify-between text-sm">
                <span className="text-slate-500">Plan</span>
                <span className="font-medium text-slate-900">
                  {selectedPlan?.name ?? '—'}
                </span>
              </div>
              {selectedPlan && (
                <div className="mt-2 flex justify-between text-sm">
                  <span className="text-slate-500">Amount</span>
                  <span className="font-bold text-indigo-600">
                    {new Intl.NumberFormat('en-US', {
                      style: 'currency',
                      currency: 'MXN',
                    }).format(selectedPlan.amount)}
                  </span>
                </div>
              )}
            </div>

            {submitError && (
              <p className="mb-4 rounded-lg bg-red-50 px-4 py-3 text-sm text-red-600">
                {submitError}
              </p>
            )}

            <div className="flex justify-between">
              <button
                type="button"
                onClick={() => setStep(2)}
                disabled={submitting}
                className="rounded-lg border border-slate-300 px-6 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50 disabled:opacity-50"
              >
                Back
              </button>
              <button
                type="button"
                onClick={() => void handlePayment()}
                disabled={submitting}
                className="flex items-center gap-2 rounded-lg bg-indigo-600 px-6 py-2 text-sm font-medium text-white hover:bg-indigo-700 disabled:opacity-50"
              >
                {submitting && (
                  <svg
                    className="h-4 w-4 animate-spin"
                    viewBox="0 0 24 24"
                    fill="none"
                    aria-hidden="true"
                  >
                    <circle
                      className="opacity-25"
                      cx="12"
                      cy="12"
                      r="10"
                      stroke="currentColor"
                      strokeWidth="4"
                    />
                    <path
                      className="opacity-75"
                      fill="currentColor"
                      d="M4 12a8 8 0 018-8v8H4z"
                    />
                  </svg>
                )}
                Proceed to Payment
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
