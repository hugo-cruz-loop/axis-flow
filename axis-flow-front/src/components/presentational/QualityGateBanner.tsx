import { CheckCircle, AlertTriangle } from 'lucide-react'
import { cn } from '@/lib/utils'

interface QualityGateBannerProps {
  missingSections: Array<'factura' | 'presupuesto' | 'calendario'>
  onActivate: () => void
  isActivating: boolean
}

const SECTION_LABELS: Record<string, string> = {
  factura: 'Datos Fiscales',
  presupuesto: 'Presupuesto',
  calendario: 'Calendario Laboral',
}

const ALL_SECTIONS = ['factura', 'presupuesto', 'calendario'] as const

export function QualityGateBanner({ missingSections, onActivate, isActivating }: QualityGateBannerProps) {
  const isComplete = missingSections.length === 0

  return (
    <div
      className={cn(
        'rounded-lg border p-4',
        isComplete
          ? 'border-green-200 bg-green-50'
          : 'border-amber-200 bg-amber-50',
      )}
    >
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <div className="flex items-center gap-2 mb-2">
            {isComplete ? (
              <CheckCircle className="h-5 w-5 text-green-600" />
            ) : (
              <AlertTriangle className="h-5 w-5 text-amber-600" />
            )}
            <p className={cn('text-sm font-semibold', isComplete ? 'text-green-800' : 'text-amber-800')}>
              {isComplete ? 'Cliente listo para activar' : 'Complete los datos requeridos para activar el cliente'}
            </p>
          </div>
          <ul className="flex flex-wrap gap-3">
            {ALL_SECTIONS.map((section) => {
              const missing = missingSections.includes(section)
              return (
                <li key={section} className="flex items-center gap-1 text-sm">
                  <CheckCircle
                    className={cn('h-4 w-4', missing ? 'text-gray-300' : 'text-green-500')}
                  />
                  <span className={cn(missing ? 'text-red-600' : 'text-green-700')}>
                    {SECTION_LABELS[section]}
                  </span>
                </li>
              )
            })}
          </ul>
        </div>
        <button
          onClick={onActivate}
          disabled={missingSections.length > 0 || isActivating}
          className={cn(
            'shrink-0 rounded-lg px-4 py-2 text-sm font-semibold transition-colors',
            isComplete && !isActivating
              ? 'bg-indigo-600 text-white hover:bg-indigo-700'
              : 'cursor-not-allowed bg-gray-200 text-gray-400',
          )}
        >
          {isActivating ? 'Activando...' : 'Activar Cliente'}
        </button>
      </div>
    </div>
  )
}
