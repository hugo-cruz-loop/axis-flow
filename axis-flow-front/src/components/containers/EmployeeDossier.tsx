import { useEffect, useState } from 'react'
import { useForm, useFieldArray } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { useSearchParams } from 'react-router-dom'
import { Plus, Trash2 } from 'lucide-react'
import {
  useUbicacion,
  useAdicionales,
  useDocumentos,
  useUpsertUbicacion,
  useUpsertAdicionales,
  useUploadDocumento,
} from '@/hooks/api/useEmpleados'
import { FileDropzone } from '@/components/presentational/FileDropzone'
import type { TipoDocumento, Documentos } from '@/types/empleados'
import { cn } from '@/lib/utils'

interface EmployeeDossierProps {
  numEmpleado: number
}

type Tab = 'ubicacion' | 'adicionales' | 'documentos'

// — Ubicación schema —

const ubicacionSchema = z.object({
  curp: z.string().length(18, 'CURP must be 18 characters'),
  nss: z.string().min(10, 'NSS must be at least 10 characters'),
  calle: z.string().min(1, 'Required'),
  numero_exterior: z.string().min(1, 'Required'),
  numero_interior: z.string().optional(),
  colonia: z.string().min(1, 'Required'),
  codigo_postal: z.string().length(5, 'Postal code must be 5 digits'),
  ciudad_id: z.number().min(1, 'Required'),
  estado_id: z.number().min(1, 'Required'),
  pais_id: z.number().min(1, 'Required'),
})
type UbicacionValues = z.infer<typeof ubicacionSchema>

// — Adicionales schema —

const beneficiarioSchema = z.object({
  nombre: z.string().min(1, 'Required'),
  parentesco: z.string().min(1, 'Required'),
  porcentaje: z.number().min(1).max(100),
})

const adicionalesSchema = z.object({
  contacto_emergencia_nombre: z.string().min(1, 'Required'),
  contacto_emergencia_telefono: z.string().min(10, 'Min 10 digits'),
  contacto_emergencia_parentesco: z.string().min(1, 'Required'),
  beneficiarios: z.array(beneficiarioSchema),
})
type AdicionalesValues = z.infer<typeof adicionalesSchema>

// — Document slots config —

const DOCUMENT_SLOTS: Array<{ tipo: TipoDocumento; label: string; field: keyof Documentos }> = [
  { tipo: 'ACTA_NACIMIENTO', label: 'Birth Certificate', field: 'acta_url' },
  { tipo: 'INE', label: 'INE / ID', field: 'ine_url' },
  { tipo: 'COMPROBANTE_DOMICILIO', label: 'Proof of Address', field: 'comprobante_domicilio_url' },
  { tipo: 'CURP_PDF', label: 'CURP PDF', field: 'curp_pdf_url' },
  { tipo: 'NSS_PDF', label: 'NSS PDF', field: 'nss_pdf_url' },
  { tipo: 'CONTRATO', label: 'Contract', field: 'contrato_url' },
]

export function EmployeeDossier({ numEmpleado }: EmployeeDossierProps) {
  const [searchParams, setSearchParams] = useSearchParams()
  const activeTab = (searchParams.get('tab') as Tab) ?? 'ubicacion'

  const setTab = (tab: Tab) => {
    setSearchParams({ tab }, { replace: true })
  }

  return (
    <div className="flex flex-col gap-4">
      {/* Tabs */}
      <div className="flex gap-1 border-b border-slate-700">
        {(['ubicacion', 'adicionales', 'documentos'] as Tab[]).map((tab) => (
          <button
            key={tab}
            onClick={() => setTab(tab)}
            className={cn(
              'px-4 py-2 text-sm font-medium capitalize transition-colors',
              activeTab === tab
                ? 'border-b-2 border-indigo-500 text-indigo-400'
                : 'text-slate-400 hover:text-white',
            )}
          >
            {tab === 'ubicacion' ? 'Location' : tab === 'adicionales' ? 'Additional Info' : 'Documents'}
          </button>
        ))}
      </div>

      {activeTab === 'ubicacion' && <UbicacionTab numEmpleado={numEmpleado} />}
      {activeTab === 'adicionales' && <AdicionalesTab numEmpleado={numEmpleado} />}
      {activeTab === 'documentos' && <DocumentosTab numEmpleado={numEmpleado} />}
    </div>
  )
}

// — Ubicación Tab —

function UbicacionTab({ numEmpleado }: { numEmpleado: number }) {
  const { data, isLoading } = useUbicacion(numEmpleado)
  const { mutate: upsert, isPending } = useUpsertUbicacion()
  const [submitError, setSubmitError] = useState<string | null>(null)
  const [success, setSuccess] = useState(false)

  const form = useForm<UbicacionValues>({
    resolver: zodResolver(ubicacionSchema),
    defaultValues: {
      curp: '', nss: '', calle: '', numero_exterior: '',
      numero_interior: '', colonia: '', codigo_postal: '',
      ciudad_id: 0, estado_id: 0, pais_id: 0,
    },
  })

  useEffect(() => {
    if (data) form.reset({ ...data })
  }, [data, form])

  const onSubmit = form.handleSubmit((values) => {
    setSubmitError(null)
    setSuccess(false)
    upsert(
      { numEmpleado, req: values },
      {
        onSuccess: () => setSuccess(true),
        onError: (err) => {
          const message =
            err && typeof err === 'object' && 'response' in err
              ? ((err as { response?: { data?: { message?: string } } }).response?.data?.message ?? 'Error saving')
              : 'Error saving'
          setSubmitError(message)
        },
      },
    )
  })

  if (isLoading) return <p className="text-sm text-slate-400">Loading...</p>

  return (
    <form onSubmit={onSubmit} className="grid grid-cols-1 gap-4 sm:grid-cols-2">
      <Field label="CURP" error={form.formState.errors.curp?.message}>
        <input {...form.register('curp')} className={inputClass} placeholder="18-character CURP" />
      </Field>
      <Field label="NSS" error={form.formState.errors.nss?.message}>
        <input {...form.register('nss')} className={inputClass} placeholder="Social security number" />
      </Field>
      <Field label="Street" error={form.formState.errors.calle?.message}>
        <input {...form.register('calle')} className={inputClass} />
      </Field>
      <Field label="Exterior Number" error={form.formState.errors.numero_exterior?.message}>
        <input {...form.register('numero_exterior')} className={inputClass} />
      </Field>
      <Field label="Interior Number">
        <input {...form.register('numero_interior')} className={inputClass} placeholder="Optional" />
      </Field>
      <Field label="Neighborhood" error={form.formState.errors.colonia?.message}>
        <input {...form.register('colonia')} className={inputClass} />
      </Field>
      <Field label="Postal Code" error={form.formState.errors.codigo_postal?.message}>
        <input {...form.register('codigo_postal')} className={inputClass} placeholder="5 digits" />
      </Field>
      <Field label="City ID" error={form.formState.errors.ciudad_id?.message}>
        <input type="number" {...form.register('ciudad_id', { valueAsNumber: true })} className={inputClass} />
      </Field>
      <Field label="State ID" error={form.formState.errors.estado_id?.message}>
        <input type="number" {...form.register('estado_id', { valueAsNumber: true })} className={inputClass} />
      </Field>
      <Field label="Country ID" error={form.formState.errors.pais_id?.message}>
        <input type="number" {...form.register('pais_id', { valueAsNumber: true })} className={inputClass} />
      </Field>
      {submitError && (
        <p className="col-span-full rounded bg-red-900/30 px-3 py-2 text-sm text-red-400">
          {submitError}
        </p>
      )}
      {success && (
        <p className="col-span-full rounded bg-green-900/30 px-3 py-2 text-sm text-green-400">
          Saved successfully
        </p>
      )}
      <div className="col-span-full flex justify-end">
        <button
          type="submit"
          disabled={isPending}
          className="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700 disabled:opacity-50"
        >
          {isPending ? 'Saving...' : 'Save'}
        </button>
      </div>
    </form>
  )
}

// — Adicionales Tab —

function AdicionalesTab({ numEmpleado }: { numEmpleado: number }) {
  const { data, isLoading } = useAdicionales(numEmpleado)
  const { mutate: upsert, isPending } = useUpsertAdicionales()
  const [submitError, setSubmitError] = useState<string | null>(null)
  const [success, setSuccess] = useState(false)

  const form = useForm<AdicionalesValues>({
    resolver: zodResolver(adicionalesSchema),
    defaultValues: {
      contacto_emergencia_nombre: '',
      contacto_emergencia_telefono: '',
      contacto_emergencia_parentesco: '',
      beneficiarios: [],
    },
  })

  const { fields, append, remove } = useFieldArray({
    control: form.control,
    name: 'beneficiarios',
  })

  useEffect(() => {
    if (data) form.reset({ ...data })
  }, [data, form])

  const onSubmit = form.handleSubmit((values) => {
    setSubmitError(null)
    setSuccess(false)
    upsert(
      { numEmpleado, req: values },
      {
        onSuccess: () => setSuccess(true),
        onError: (err) => {
          const message =
            err && typeof err === 'object' && 'response' in err
              ? ((err as { response?: { data?: { message?: string } } }).response?.data?.message ?? 'Error saving')
              : 'Error saving'
          setSubmitError(message)
        },
      },
    )
  })

  if (isLoading) return <p className="text-sm text-slate-400">Loading...</p>

  return (
    <form onSubmit={onSubmit} className="flex flex-col gap-4">
      <h3 className="text-sm font-semibold text-slate-300">Emergency Contact</h3>
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <Field label="Name" error={form.formState.errors.contacto_emergencia_nombre?.message}>
          <input {...form.register('contacto_emergencia_nombre')} className={inputClass} />
        </Field>
        <Field label="Phone" error={form.formState.errors.contacto_emergencia_telefono?.message}>
          <input {...form.register('contacto_emergencia_telefono')} className={inputClass} />
        </Field>
        <Field label="Relationship" error={form.formState.errors.contacto_emergencia_parentesco?.message}>
          <input {...form.register('contacto_emergencia_parentesco')} className={inputClass} />
        </Field>
      </div>

      <div className="flex items-center justify-between">
        <h3 className="text-sm font-semibold text-slate-300">Beneficiaries</h3>
        <button
          type="button"
          onClick={() => append({ nombre: '', parentesco: '', porcentaje: 0 })}
          className="flex items-center gap-1 rounded px-2 py-1 text-xs font-medium text-indigo-400 hover:bg-indigo-500/10"
        >
          <Plus className="h-3 w-3" /> Add
        </button>
      </div>

      {fields.length === 0 && (
        <p className="text-xs text-slate-500">No beneficiaries added yet.</p>
      )}

      {fields.map((field, index) => (
        <div key={field.id} className="flex items-end gap-2">
          <Field label="Name" error={form.formState.errors.beneficiarios?.[index]?.nombre?.message}>
            <input
              {...form.register(`beneficiarios.${index}.nombre`)}
              className={inputClass}
              placeholder="Name"
            />
          </Field>
          <Field label="Relationship">
            <input
              {...form.register(`beneficiarios.${index}.parentesco`)}
              className={inputClass}
              placeholder="e.g. Spouse"
            />
          </Field>
          <Field label="% Share">
            <input
              type="number"
              min={1}
              max={100}
              {...form.register(`beneficiarios.${index}.porcentaje`, { valueAsNumber: true })}
              className={cn(inputClass, 'w-20')}
            />
          </Field>
          <button
            type="button"
            onClick={() => remove(index)}
            className="mb-0.5 rounded p-1 text-red-400 hover:bg-red-900/20"
          >
            <Trash2 className="h-4 w-4" />
          </button>
        </div>
      ))}

      {submitError && (
        <p className="rounded bg-red-900/30 px-3 py-2 text-sm text-red-400">{submitError}</p>
      )}
      {success && (
        <p className="rounded bg-green-900/30 px-3 py-2 text-sm text-green-400">
          Saved successfully
        </p>
      )}
      <div className="flex justify-end">
        <button
          type="submit"
          disabled={isPending}
          className="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700 disabled:opacity-50"
        >
          {isPending ? 'Saving...' : 'Save'}
        </button>
      </div>
    </form>
  )
}

// — Documentos Tab —

function DocumentosTab({ numEmpleado }: { numEmpleado: number }) {
  const { data, isLoading } = useDocumentos(numEmpleado)
  const { mutate: upload, isPending } = useUploadDocumento()
  const [uploadingSlot, setUploadingSlot] = useState<TipoDocumento | null>(null)

  const handleFile = (tipo: TipoDocumento, file: File) => {
    setUploadingSlot(tipo)
    upload(
      { numEmpleado, tipo, file },
      { onSettled: () => setUploadingSlot(null) },
    )
  }

  if (isLoading) return <p className="text-sm text-slate-400">Loading...</p>

  return (
    <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
      {DOCUMENT_SLOTS.map(({ tipo, label, field }) => {
        const existingUrl = data?.[field] as string | undefined
        return (
          <div key={tipo} className="flex flex-col gap-2 rounded-lg border border-slate-700 p-3">
            <p className="text-sm font-medium text-slate-200">{label}</p>
            {existingUrl ? (
              <a
                href={existingUrl}
                target="_blank"
                rel="noopener noreferrer"
                className="text-xs text-indigo-400 underline"
              >
                View current file
              </a>
            ) : (
              <p className="text-xs text-slate-500">No file uploaded</p>
            )}
            <FileDropzone
              accept=".pdf,.png,.jpg,.jpeg"
              maxSizeMB={5}
              onFile={(file) => handleFile(tipo, file)}
              label={existingUrl ? 'Replace file' : 'Upload file'}
              disabled={uploadingSlot === tipo || isPending}
            />
          </div>
        )
      })}
    </div>
  )
}

// — Shared helpers —

const inputClass =
  'w-full rounded-lg border border-slate-600 bg-slate-800 px-3 py-2 text-sm text-white placeholder-slate-500 focus:border-indigo-500 focus:outline-none'

function Field({
  label,
  error,
  children,
}: {
  label: string
  error?: string
  children: React.ReactNode
}) {
  return (
    <div className="flex flex-col gap-1">
      <label className="text-xs font-medium text-slate-400">{label}</label>
      {children}
      {error && <p className="text-xs text-red-400">{error}</p>}
    </div>
  )
}

