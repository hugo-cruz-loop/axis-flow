import { useEffect, useRef, useState, useCallback } from 'react'
import { connectAttendanceWS, getAccessToken } from '@/api/empleadosClient'
import { useAsistencias } from '@/hooks/api/useEmpleados'
import { BiometricAlertCard } from '@/components/presentational/BiometricAlertCard'
import type { AsistenciaVerificadaEvent } from '@/types/empleados'
import { ObservacionStatus, EstatusRango } from '@/types/empleados'
import { cn } from '@/lib/utils'

interface AttendanceMonitorProps {
  empresaId: number
  empleadoId?: number
}

const MAX_RETRIES = 3
const BASE_DELAY_MS = 1000

export function AttendanceMonitor({ empresaId, empleadoId }: AttendanceMonitorProps) {
  const { data: asistencias = [], isLoading } = useAsistencias(empresaId, empleadoId)
  const [alerts, setAlerts] = useState<Array<AsistenciaVerificadaEvent & { alertId: string }>>([])
  const wsRef = useRef<WebSocket | null>(null)
  const retryRef = useRef(0)
  const timeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const dismissAlert = useCallback((alertId: string) => {
    setAlerts((prev) => prev.filter((a) => a.alertId !== alertId))
  }, [])

  const connect = useCallback(() => {
    if (wsRef.current) {
      wsRef.current.onclose = null
      wsRef.current.close()
    }

    const token = getAccessToken()
    if (!token || !empresaId) return

    const ws = connectAttendanceWS(empresaId, token)
    wsRef.current = ws

    ws.onmessage = (ev) => {
      try {
        const event = JSON.parse(ev.data as string) as AsistenciaVerificadaEvent
        if (event.estatus_observacion === ObservacionStatus.RECHAZADA) {
          setAlerts((prev) => [
            { ...event, alertId: `${event.asistencia_id}-${Date.now()}` },
            ...prev,
          ])
        }
      } catch {
        // ignore malformed messages
      }
    }

    ws.onopen = () => {
      retryRef.current = 0
    }

    ws.onclose = () => {
      if (retryRef.current < MAX_RETRIES) {
        const delay = BASE_DELAY_MS * Math.pow(2, retryRef.current)
        retryRef.current++
        timeoutRef.current = setTimeout(connect, delay)
      }
    }

    ws.onerror = () => {
      ws.close()
    }
  }, [empresaId])

  useEffect(() => {
    connect()
    return () => {
      if (timeoutRef.current) clearTimeout(timeoutRef.current)
      if (wsRef.current) {
        wsRef.current.onclose = null
        wsRef.current.close()
      }
    }
  }, [connect])

  return (
    <div className="flex flex-col gap-4">
      {/* Biometric alerts */}
      {alerts.length > 0 && (
        <div className="flex flex-col gap-2">
          <h3 className="text-sm font-semibold text-red-400">
            Biometric Alerts ({alerts.length})
          </h3>
          {alerts.map((alert) => (
            <BiometricAlertCard
              key={alert.alertId}
              event={alert}
              onDismiss={() => dismissAlert(alert.alertId)}
            />
          ))}
        </div>
      )}

      {/* Attendance table */}
      {isLoading ? (
        <p className="text-sm text-slate-400">Loading attendance records...</p>
      ) : asistencias.length === 0 ? (
        <p className="text-sm text-slate-500">No attendance records found.</p>
      ) : (
        <div className="overflow-x-auto rounded-lg border border-slate-700">
          <table className="w-full text-sm">
            <thead className="bg-slate-800 text-xs uppercase text-slate-400">
              <tr>
                <th className="px-4 py-3 text-left">Date</th>
                <th className="px-4 py-3 text-left">Check-in</th>
                <th className="px-4 py-3 text-left">Type</th>
                <th className="px-4 py-3 text-left">Range</th>
                <th className="px-4 py-3 text-left">Verification</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-700">
              {asistencias.map((a) => (
                <tr key={a.id} className="bg-slate-900 hover:bg-slate-800/50">
                  <td className="px-4 py-3 text-slate-300">{a.fecha}</td>
                  <td className="px-4 py-3 text-slate-300">{a.hora_entrada}</td>
                  <td className="px-4 py-3 text-slate-300">{a.tipo_registro}</td>
                  <td className="px-4 py-3">
                    <span
                      className={cn(
                        'rounded-full px-2 py-0.5 text-xs font-medium',
                        a.estatus_rango === EstatusRango.EN_RANGO
                          ? 'bg-green-900/50 text-green-300'
                          : 'bg-yellow-900/50 text-yellow-300',
                      )}
                    >
                      {a.estatus_rango === EstatusRango.EN_RANGO ? 'In Range' : 'Out of Range'}
                    </span>
                  </td>
                  <td className="px-4 py-3">
                    <ObservacionBadge status={a.estatus_observacion_entrada} />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}

function ObservacionBadge({ status }: { status: number }) {
  const config: Record<number, { label: string; className: string }> = {
    [ObservacionStatus.PENDIENTE]: {
      label: 'Pending',
      className: 'bg-slate-700 text-slate-300',
    },
    [ObservacionStatus.VALIDADA]: {
      label: 'Verified',
      className: 'bg-green-900/50 text-green-300',
    },
    [ObservacionStatus.RECHAZADA]: {
      label: 'Rejected',
      className: 'bg-red-900/50 text-red-300',
    },
  }
  const { label, className } = config[status] ?? {
    label: 'Unknown',
    className: 'bg-slate-700 text-slate-400',
  }
  return (
    <span className={cn('rounded-full px-2 py-0.5 text-xs font-medium', className)}>
      {label}
    </span>
  )
}
