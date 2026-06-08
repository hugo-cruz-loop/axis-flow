import { useState } from 'react'
import { useParams, useNavigate, useSearchParams } from 'react-router-dom'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { ArrowLeft, Star, Plus, ChevronDown, ChevronRight } from 'lucide-react'
import { DashboardLayout } from '@/layouts/DashboardLayout'
import { QualityGateBanner } from '@/components/presentational/QualityGateBanner'
import {
  useCliente,
  useFactura,
  usePresupuesto,
  useCalendario,
  useLocalidades,
  useEvaluaciones,
  usePatchEstatus,
  useCreateFactura,
  useCreatePresupuesto,
  useCreateCalendario,
  useCreateLocalidad,
  useCreateEvaluacion,
} from '@/hooks/api/useClientes'
import { ClienteStatus } from '@/types/clientes'
import { cn } from '@/lib/utils'

type TabKey = 'fiscal' | 'presupuesto' | 'calendario' | 'localidades' | 'evaluaciones'

const TABS: { key: TabKey; label: string }[] = [
  { key: 'fiscal', label: 'Datos Fiscales' },
  { key: 'presupuesto', label: 'Presupuesto' },
  { key: 'calendario', label: 'Calendario' },
  { key: 'localidades', label: 'Localidades' },
  { key: 'evaluaciones', label: 'Evaluaciones' },
]

// Schemas
const facturaSchema = z.object({
  rfc: z.string().min(12).max(13),
  razon_social: z.string().min(2),
  domicilio_fiscal: z.string().min(5),
})

const optionalNumber = z.preprocess(
  (v) => (v === '' || v === undefined || v === null ? undefined : Number(v)),
  z.number().optional(),
)

const presupuestoSchema = z.object({
  personal_requerido: optionalNumber,
  material_estimado: z.string().optional(),
  costo_mensual: optionalNumber,
})

const calendarioSchema = z.object({
  lunes: z.boolean(),
  martes: z.boolean(),
  miercoles: z.boolean(),
  jueves: z.boolean(),
  viernes: z.boolean(),
  sabado: z.boolean(),
  domingo: z.boolean(),
})

const localidadSchema = z.object({
  nombre: z.string().min(2),
  direccion: z.string().min(5),
  supervisor_id: z.string().uuid('UUID required'),
  tipo_localidad_id: z.preprocess((v) => Number(v), z.number().min(1)),
})

const evaluacionSchema = z.object({
  puntuacion: z.preprocess((v) => Number(v), z.number().min(1).max(5)),
  comentarios: z.string().optional(),
})

type FacturaValues = z.infer<typeof facturaSchema>
type PresupuestoValues = z.infer<typeof presupuestoSchema>
type CalendarioValues = z.infer<typeof calendarioSchema>
type LocalidadValues = z.infer<typeof localidadSchema>
type EvaluacionValues = z.infer<typeof evaluacionSchema>

const WEEKDAYS = ['lunes', 'martes', 'miercoles', 'jueves', 'viernes', 'sabado', 'domingo'] as const

function EstatusBadge({ estatus }: { estatus: number }) {
  const isActivo = estatus === ClienteStatus.ACTIVO
  return (
    <span
      className={cn(
        'inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium',
        isActivo ? 'bg-green-100 text-green-700' : 'bg-amber-100 text-amber-700',
      )}
    >
      {isActivo ? 'Activo' : 'Incompleto'}
    </span>
  )
}

function StarRating({ value }: { value: number }) {
  return (
    <div className="flex gap-0.5">
      {[1, 2, 3, 4, 5].map((s) => (
        <Star
          key={s}
          className={cn('h-4 w-4', s <= value ? 'fill-yellow-400 text-yellow-400' : 'text-gray-300')}
        />
      ))}
    </div>
  )
}

// Fiscal tab
function FiscalTab({ clienteId }: { clienteId: string }) {
  const { data: factura, isLoading } = useFactura(clienteId)
  const { mutate: createFactura, isPending } = useCreateFactura()
  const [editMode, setEditMode] = useState(!factura)
  const form = useForm<FacturaValues>({
    resolver: zodResolver(facturaSchema),
    defaultValues: { rfc: factura?.rfc ?? '', razon_social: factura?.razon_social ?? '', domicilio_fiscal: factura?.domicilio_fiscal ?? '' },
  })

  if (isLoading) return <p className="text-sm text-gray-500">Cargando...</p>

  const onSubmit = form.handleSubmit((values) => {
    createFactura({ clienteId, req: values }, { onSuccess: () => setEditMode(false) })
  })

  if (factura && !editMode) {
    return (
      <div className="space-y-3">
        <dl className="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <div>
            <dt className="text-xs font-medium text-gray-500 uppercase">RFC</dt>
            <dd className="mt-0.5 text-sm text-gray-900">{factura.rfc}</dd>
          </div>
          <div>
            <dt className="text-xs font-medium text-gray-500 uppercase">Razón Social</dt>
            <dd className="mt-0.5 text-sm text-gray-900">{factura.razon_social}</dd>
          </div>
          <div className="sm:col-span-2">
            <dt className="text-xs font-medium text-gray-500 uppercase">Domicilio Fiscal</dt>
            <dd className="mt-0.5 text-sm text-gray-900">{factura.domicilio_fiscal}</dd>
          </div>
        </dl>
        <button
          onClick={() => setEditMode(true)}
          className="text-sm text-indigo-600 hover:underline"
        >
          Editar
        </button>
      </div>
    )
  }

  return (
    <form onSubmit={onSubmit} className="space-y-4 max-w-md">
      {(['rfc', 'razon_social', 'domicilio_fiscal'] as const).map((field) => (
        <div key={field}>
          <label className="mb-1 block text-sm font-medium text-gray-700 capitalize">
            {field.replace('_', ' ')}
          </label>
          <input
            {...form.register(field)}
            className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none"
          />
          {form.formState.errors[field] && (
            <p className="mt-1 text-xs text-red-600">{form.formState.errors[field]?.message}</p>
          )}
        </div>
      ))}
      <div className="flex gap-2">
        {factura && (
          <button type="button" onClick={() => setEditMode(false)} className="rounded-lg border border-gray-300 px-3 py-2 text-sm text-gray-700 hover:bg-gray-50">
            Cancelar
          </button>
        )}
        <button
          type="submit"
          disabled={isPending}
          className="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700 disabled:opacity-50"
        >
          {isPending ? 'Guardando...' : 'Guardar'}
        </button>
      </div>
    </form>
  )
}

// Presupuesto tab
function PresupuestoTab({ clienteId }: { clienteId: string }) {
  const { data: presupuesto, isLoading } = usePresupuesto(clienteId)
  const { mutate: createPresupuesto, isPending } = useCreatePresupuesto()
  const [editMode, setEditMode] = useState(!presupuesto)
  const form = useForm<PresupuestoValues>({
    resolver: zodResolver(presupuestoSchema) as never,
    defaultValues: {
      personal_requerido: presupuesto?.personal_requerido,
      material_estimado: presupuesto?.material_estimado ?? '',
      costo_mensual: presupuesto?.costo_mensual,
    },
  })

  if (isLoading) return <p className="text-sm text-gray-500">Cargando...</p>

  const onSubmit = form.handleSubmit((values) => {
    createPresupuesto({ clienteId, req: values }, { onSuccess: () => setEditMode(false) })
  })

  if (presupuesto && !editMode) {
    return (
      <div className="space-y-3">
        <dl className="grid grid-cols-1 gap-3 sm:grid-cols-3">
          <div>
            <dt className="text-xs font-medium text-gray-500 uppercase">Personal Requerido</dt>
            <dd className="mt-0.5 text-sm text-gray-900">{presupuesto.personal_requerido ?? '—'}</dd>
          </div>
          <div>
            <dt className="text-xs font-medium text-gray-500 uppercase">Costo Mensual</dt>
            <dd className="mt-0.5 text-sm text-gray-900">
              {presupuesto.costo_mensual != null ? `$${presupuesto.costo_mensual.toLocaleString()}` : '—'}
            </dd>
          </div>
          <div>
            <dt className="text-xs font-medium text-gray-500 uppercase">Material Estimado</dt>
            <dd className="mt-0.5 text-sm text-gray-900">{presupuesto.material_estimado ?? '—'}</dd>
          </div>
        </dl>
        <button onClick={() => setEditMode(true)} className="text-sm text-indigo-600 hover:underline">
          Editar
        </button>
      </div>
    )
  }

  return (
    <form onSubmit={onSubmit} className="space-y-4 max-w-md">
      <div>
        <label className="mb-1 block text-sm font-medium text-gray-700">Personal Requerido</label>
        <input type="number" {...form.register('personal_requerido')} className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none" />
      </div>
      <div>
        <label className="mb-1 block text-sm font-medium text-gray-700">Costo Mensual</label>
        <input type="number" {...form.register('costo_mensual')} className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none" />
      </div>
      <div>
        <label className="mb-1 block text-sm font-medium text-gray-700">Material Estimado</label>
        <input {...form.register('material_estimado')} className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none" />
      </div>
      <div className="flex gap-2">
        {presupuesto && (
          <button type="button" onClick={() => setEditMode(false)} className="rounded-lg border border-gray-300 px-3 py-2 text-sm text-gray-700 hover:bg-gray-50">Cancelar</button>
        )}
        <button type="submit" disabled={isPending} className="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700 disabled:opacity-50">
          {isPending ? 'Guardando...' : 'Guardar'}
        </button>
      </div>
    </form>
  )
}

// Calendario tab
function CalendarioTab({ clienteId }: { clienteId: string }) {
  const { data: calendario, isLoading } = useCalendario(clienteId)
  const { mutate: createCalendario, isPending } = useCreateCalendario()
  const [editMode, setEditMode] = useState(!calendario)
  const form = useForm<CalendarioValues>({
    resolver: zodResolver(calendarioSchema),
    defaultValues: Object.fromEntries(
      WEEKDAYS.map((d) => [d, calendario?.semana_laboral?.[d] ?? false])
    ) as CalendarioValues,
  })

  if (isLoading) return <p className="text-sm text-gray-500">Cargando...</p>

  const onSubmit = form.handleSubmit((values) => {
    createCalendario(
      { clienteId, req: { semana_laboral: values as Record<string, boolean>, dias_inhabiles: calendario?.dias_inhabiles ?? [] } },
      { onSuccess: () => setEditMode(false) }
    )
  })

  if (calendario && !editMode) {
    return (
      <div className="space-y-4">
        <div className="flex flex-wrap gap-2">
          {WEEKDAYS.map((d) => (
            <span
              key={d}
              className={cn(
                'rounded-full px-3 py-1 text-sm font-medium capitalize',
                calendario.semana_laboral[d] ? 'bg-indigo-100 text-indigo-700' : 'bg-gray-100 text-gray-400',
              )}
            >
              {d}
            </span>
          ))}
        </div>
        {calendario.dias_inhabiles.length > 0 && (
          <div>
            <p className="text-xs font-medium text-gray-500 uppercase mb-1">Días Inhábiles</p>
            <ul className="text-sm text-gray-700 space-y-0.5">
              {calendario.dias_inhabiles.map((d) => <li key={d}>{d}</li>)}
            </ul>
          </div>
        )}
        <button onClick={() => setEditMode(true)} className="text-sm text-indigo-600 hover:underline">
          Editar
        </button>
      </div>
    )
  }

  return (
    <form onSubmit={onSubmit} className="space-y-4 max-w-sm">
      <p className="text-sm font-medium text-gray-700">Días laborales</p>
      <div className="flex flex-wrap gap-3">
        {WEEKDAYS.map((d) => (
          <label key={d} className="flex items-center gap-1.5 text-sm capitalize cursor-pointer">
            <input type="checkbox" {...form.register(d)} className="rounded border-gray-300" />
            {d}
          </label>
        ))}
      </div>
      <div className="flex gap-2">
        {calendario && (
          <button type="button" onClick={() => setEditMode(false)} className="rounded-lg border border-gray-300 px-3 py-2 text-sm text-gray-700 hover:bg-gray-50">Cancelar</button>
        )}
        <button type="submit" disabled={isPending} className="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700 disabled:opacity-50">
          {isPending ? 'Guardando...' : 'Guardar'}
        </button>
      </div>
    </form>
  )
}

// Localidades tab
function LocalidadesTab({ clienteId }: { clienteId: string }) {
  const { data: localidades = [], isLoading } = useLocalidades(clienteId)
  const { mutate: createLocalidad, isPending } = useCreateLocalidad()
  const [showForm, setShowForm] = useState(false)
  const [expanded, setExpanded] = useState<string | null>(null)
  const form = useForm<LocalidadValues>({ resolver: zodResolver(localidadSchema) as never })

  const onSubmit = form.handleSubmit((values) => {
    createLocalidad({ clienteId, req: values }, { onSuccess: () => { setShowForm(false); form.reset() } })
  })

  if (isLoading) return <p className="text-sm text-gray-500">Cargando...</p>

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <p className="text-sm text-gray-500">{localidades.length} localidad(es)</p>
        <button onClick={() => setShowForm((v) => !v)} className="flex items-center gap-1 rounded-lg bg-indigo-600 px-3 py-1.5 text-sm text-white hover:bg-indigo-700">
          <Plus className="h-3.5 w-3.5" />
          Nueva Localidad
        </button>
      </div>

      {showForm && (
        <form onSubmit={onSubmit} className="rounded-lg border border-indigo-200 bg-indigo-50 p-4 space-y-3 max-w-md">
          {(['nombre', 'direccion', 'supervisor_id'] as const).map((field) => (
            <div key={field}>
              <label className="mb-0.5 block text-xs font-medium text-gray-700 capitalize">{field.replace('_id', ' ID').replace('_', ' ')}</label>
              <input {...form.register(field)} className="w-full rounded border border-gray-300 px-2 py-1.5 text-sm focus:border-indigo-500 focus:outline-none" />
              {form.formState.errors[field] && (
                <p className="mt-0.5 text-xs text-red-600">{form.formState.errors[field]?.message}</p>
              )}
            </div>
          ))}
          <div>
            <label className="mb-0.5 block text-xs font-medium text-gray-700">Tipo Localidad ID</label>
            <input type="number" {...form.register('tipo_localidad_id')} className="w-full rounded border border-gray-300 px-2 py-1.5 text-sm focus:border-indigo-500 focus:outline-none" />
          </div>
          <div className="flex gap-2">
            <button type="button" onClick={() => setShowForm(false)} className="rounded border border-gray-300 px-3 py-1.5 text-sm text-gray-700 hover:bg-gray-50">Cancelar</button>
            <button type="submit" disabled={isPending} className="rounded bg-indigo-600 px-3 py-1.5 text-sm text-white hover:bg-indigo-700 disabled:opacity-50">
              {isPending ? 'Guardando...' : 'Guardar'}
            </button>
          </div>
        </form>
      )}

      <ul className="space-y-2">
        {localidades.map((loc) => (
          <li key={loc.id} className="rounded-lg border border-gray-200 bg-white">
            <button
              onClick={() => setExpanded(expanded === loc.id ? null : loc.id)}
              className="flex w-full items-center justify-between px-4 py-3 text-left"
            >
              <span className="text-sm font-medium text-gray-900">{loc.nombre}</span>
              {expanded === loc.id ? <ChevronDown className="h-4 w-4 text-gray-400" /> : <ChevronRight className="h-4 w-4 text-gray-400" />}
            </button>
            {expanded === loc.id && (
              <div className="border-t border-gray-100 px-4 pb-4 pt-3">
                <p className="text-sm text-gray-600">{loc.direccion}</p>
                <p className="mt-1 text-xs text-gray-400">Supervisor: {loc.supervisor_id}</p>
              </div>
            )}
          </li>
        ))}
      </ul>
    </div>
  )
}

// Evaluaciones tab
function EvaluacionesTab({ clienteId }: { clienteId: string }) {
  const { data: evaluaciones = [], isLoading } = useEvaluaciones(clienteId)
  const { mutate: createEvaluacion, isPending } = useCreateEvaluacion()
  const [showForm, setShowForm] = useState(false)
  const form = useForm<EvaluacionValues>({ resolver: zodResolver(evaluacionSchema) as never })

  const onSubmit = form.handleSubmit((values) => {
    createEvaluacion({ clienteId, req: values }, { onSuccess: () => { setShowForm(false); form.reset() } })
  })

  if (isLoading) return <p className="text-sm text-gray-500">Cargando...</p>

  return (
    <div className="space-y-4">
      <div className="flex justify-end">
        <button onClick={() => setShowForm((v) => !v)} className="flex items-center gap-1 rounded-lg bg-indigo-600 px-3 py-1.5 text-sm text-white hover:bg-indigo-700">
          <Plus className="h-3.5 w-3.5" />
          Nueva Evaluación
        </button>
      </div>

      {showForm && (
        <form onSubmit={onSubmit} className="rounded-lg border border-indigo-200 bg-indigo-50 p-4 space-y-3 max-w-sm">
          <div>
            <label className="mb-0.5 block text-xs font-medium text-gray-700">Puntuación (1–5)</label>
            <input type="number" min={1} max={5} {...form.register('puntuacion')} className="w-full rounded border border-gray-300 px-2 py-1.5 text-sm focus:border-indigo-500 focus:outline-none" />
            {form.formState.errors.puntuacion && (
              <p className="mt-0.5 text-xs text-red-600">{form.formState.errors.puntuacion?.message}</p>
            )}
          </div>
          <div>
            <label className="mb-0.5 block text-xs font-medium text-gray-700">Comentarios</label>
            <textarea {...form.register('comentarios')} className="w-full rounded border border-gray-300 px-2 py-1.5 text-sm focus:border-indigo-500 focus:outline-none" rows={3} />
          </div>
          <div className="flex gap-2">
            <button type="button" onClick={() => setShowForm(false)} className="rounded border border-gray-300 px-3 py-1.5 text-sm text-gray-700 hover:bg-gray-50">Cancelar</button>
            <button type="submit" disabled={isPending} className="rounded bg-indigo-600 px-3 py-1.5 text-sm text-white hover:bg-indigo-700 disabled:opacity-50">
              {isPending ? 'Guardando...' : 'Guardar'}
            </button>
          </div>
        </form>
      )}

      {evaluaciones.length === 0 ? (
        <p className="text-sm text-gray-500">Sin evaluaciones registradas.</p>
      ) : (
        <div className="overflow-hidden rounded-lg border border-gray-200 bg-white">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-gray-200 bg-gray-50 text-left text-xs font-semibold uppercase tracking-wider text-gray-500">
                <th className="px-4 py-3">Puntuación</th>
                <th className="px-4 py-3">Comentarios</th>
                <th className="px-4 py-3">Fecha</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {evaluaciones.map((ev) => (
                <tr key={ev.id} className="hover:bg-gray-50">
                  <td className="px-4 py-3"><StarRating value={ev.puntuacion} /></td>
                  <td className="px-4 py-3 text-gray-500">{ev.comentarios ?? '—'}</td>
                  <td className="px-4 py-3 text-gray-500">{new Date(ev.fecha).toLocaleDateString()}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}

// Main page
export function ClientDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()
  const activeTab = (searchParams.get('tab') as TabKey) ?? 'fiscal'

  const clienteId = id ?? ''
  const { data: cliente, isLoading } = useCliente(clienteId)
  const { data: factura } = useFactura(clienteId)
  const { data: presupuesto } = usePresupuesto(clienteId)
  const { data: calendario } = useCalendario(clienteId)
  const { mutate: patchEstatus, isPending: isActivating } = usePatchEstatus()

  if (isLoading) {
    return (
      <DashboardLayout>
        <div className="flex items-center justify-center py-24 text-gray-500">Cargando...</div>
      </DashboardLayout>
    )
  }

  if (!cliente) {
    return (
      <DashboardLayout>
        <div className="flex items-center justify-center py-24 text-gray-500">Cliente no encontrado.</div>
      </DashboardLayout>
    )
  }

  const isIncompleto = cliente.estatus === ClienteStatus.INCOMPLETO
  const missingSections: Array<'factura' | 'presupuesto' | 'calendario'> = []
  if (!factura) missingSections.push('factura')
  if (!presupuesto) missingSections.push('presupuesto')
  if (!calendario) missingSections.push('calendario')

  const handleActivate = () => {
    patchEstatus({ id: clienteId, estatus: ClienteStatus.ACTIVO })
  }

  const setTab = (tab: TabKey) => {
    setSearchParams({ tab })
  }

  return (
    <DashboardLayout>
      <div className="space-y-6">
        {/* Back */}
        <button
          onClick={() => navigate('/dashboard/clientes')}
          className="flex items-center gap-1 text-sm text-gray-500 hover:text-gray-900"
        >
          <ArrowLeft className="h-4 w-4" />
          Clientes
        </button>

        {/* Header */}
        <div className="flex items-center gap-3">
          <div>
            <h1 className="text-2xl font-bold text-gray-900">{cliente.nombre_comercial}</h1>
            <p className="mt-0.5 text-sm text-gray-500">Empresa ID: {cliente.empresa_id}</p>
          </div>
          <EstatusBadge estatus={cliente.estatus} />
        </div>

        {/* Quality gate banner */}
        {isIncompleto && (
          <QualityGateBanner
            missingSections={missingSections}
            onActivate={handleActivate}
            isActivating={isActivating}
          />
        )}

        {/* Tabs */}
        <div className="border-b border-gray-200">
          <nav className="-mb-px flex gap-4">
            {TABS.map((tab) => (
              <button
                key={tab.key}
                onClick={() => setTab(tab.key)}
                className={cn(
                  'border-b-2 pb-3 text-sm font-medium transition-colors',
                  activeTab === tab.key
                    ? 'border-indigo-600 text-indigo-600'
                    : 'border-transparent text-gray-500 hover:border-gray-300 hover:text-gray-700',
                )}
              >
                {tab.label}
              </button>
            ))}
          </nav>
        </div>

        {/* Tab content */}
        <div>
          {activeTab === 'fiscal' && <FiscalTab clienteId={clienteId} />}
          {activeTab === 'presupuesto' && <PresupuestoTab clienteId={clienteId} />}
          {activeTab === 'calendario' && <CalendarioTab clienteId={clienteId} />}
          {activeTab === 'localidades' && <LocalidadesTab clienteId={clienteId} />}
          {activeTab === 'evaluaciones' && <EvaluacionesTab clienteId={clienteId} />}
        </div>
      </div>
    </DashboardLayout>
  )
}
