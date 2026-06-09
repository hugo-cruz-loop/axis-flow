import { X } from 'lucide-react'
import { ObservacionStatus } from '@/types/empleados'
import type { AsistenciaVerificadaEvent } from '@/types/empleados'
import { cn } from '@/lib/utils'

interface BiometricAlertCardProps {
  event: AsistenciaVerificadaEvent
  onDismiss: () => void
}

export function BiometricAlertCard({ event, onDismiss }: BiometricAlertCardProps) {
  const isRechazada = event.estatus_observacion === ObservacionStatus.RECHAZADA
  const similarity = Math.round(event.similitud_facial * 100) / 100

  return (
    <div
      role="alert"
      className={cn(
        'flex items-start gap-3 rounded-lg border p-3',
        isRechazada
          ? 'border-red-700 bg-red-900/30 text-red-200'
          : 'border-green-700 bg-green-900/30 text-green-200',
      )}
    >
      {event.foto_entrada_url && (
        <img
          src={event.foto_entrada_url}
          alt="Attendance photo"
          className="h-12 w-12 shrink-0 rounded-md object-cover"
        />
      )}
      <div className="min-w-0 flex-1">
        <p className="text-sm font-semibold">
          {isRechazada ? 'Facial verification rejected' : 'Facial verification approved'}
        </p>
        <p className="mt-0.5 text-xs opacity-80">
          Employee #{event.empleado_id} &mdash; similarity: {similarity}%
        </p>
        <p className="text-xs opacity-60">
          {new Date(event.timestamp).toLocaleTimeString()}
        </p>
      </div>
      <button
        onClick={onDismiss}
        aria-label="Dismiss alert"
        className="shrink-0 rounded p-1 opacity-70 hover:opacity-100"
      >
        <X className="h-4 w-4" />
      </button>
    </div>
  )
}
