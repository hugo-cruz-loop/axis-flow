import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { IncidenciaSchema } from '../../schemas/validation'
import type { IncidenciaInput } from '../../schemas/validation'

interface IncidentFormProps {
  onSubmit: (data: IncidenciaInput) => void
  isSubmitting?: boolean
}

export function IncidentForm({ onSubmit, isSubmitting = false }: IncidentFormProps) {
  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<IncidenciaInput>({
    resolver: zodResolver(IncidenciaSchema),
  })

  return (
    <form onSubmit={handleSubmit(onSubmit)} className="space-y-4" noValidate>
      <div>
        <label htmlFor="empleado_id" className="block text-sm font-medium text-gray-700">
          Employee ID
        </label>
        <input
          id="empleado_id"
          type="number"
          {...register('empleado_id', { valueAsNumber: true })}
          className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
        />
        {errors.empleado_id && (
          <p className="mt-1 text-xs text-red-600">{errors.empleado_id.message}</p>
        )}
      </div>

      <div>
        <label htmlFor="tipo_incidencia_id" className="block text-sm font-medium text-gray-700">
          Incident Type ID
        </label>
        <input
          id="tipo_incidencia_id"
          type="text"
          {...register('tipo_incidencia_id')}
          placeholder="UUID"
          className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
        />
        {errors.tipo_incidencia_id && (
          <p className="mt-1 text-xs text-red-600">{errors.tipo_incidencia_id.message}</p>
        )}
      </div>

      <div>
        <label htmlFor="descripcion" className="block text-sm font-medium text-gray-700">
          Description
        </label>
        <textarea
          id="descripcion"
          rows={3}
          {...register('descripcion')}
          className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
        />
        {errors.descripcion && (
          <p className="mt-1 text-xs text-red-600">{errors.descripcion.message}</p>
        )}
      </div>

      <div>
        <label htmlFor="fecha_incidencia" className="block text-sm font-medium text-gray-700">
          Incident Date
        </label>
        <input
          id="fecha_incidencia"
          type="datetime-local"
          {...register('fecha_incidencia')}
          className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
        />
        {errors.fecha_incidencia && (
          <p className="mt-1 text-xs text-red-600">{errors.fecha_incidencia.message}</p>
        )}
      </div>

      <div>
        <label htmlFor="sancion_sugerida" className="block text-sm font-medium text-gray-700">
          Suggested Sanction (optional)
        </label>
        <input
          id="sancion_sugerida"
          type="text"
          {...register('sancion_sugerida')}
          className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
        />
        {errors.sancion_sugerida && (
          <p className="mt-1 text-xs text-red-600">{errors.sancion_sugerida.message}</p>
        )}
      </div>

      <div>
        <label htmlFor="evidencia_url" className="block text-sm font-medium text-gray-700">
          Evidence URL (optional)
        </label>
        <input
          id="evidencia_url"
          type="url"
          {...register('evidencia_url')}
          placeholder="https://..."
          className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
        />
        {errors.evidencia_url && (
          <p className="mt-1 text-xs text-red-600">{errors.evidencia_url.message}</p>
        )}
      </div>

      <button
        type="submit"
        disabled={isSubmitting}
        className="w-full rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-50"
      >
        {isSubmitting ? 'Submitting…' : 'Submit Incident'}
      </button>
    </form>
  )
}
