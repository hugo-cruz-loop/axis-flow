import type { EnrollEstatus } from '../../types'

interface CourseProgressBarProps {
  porcentaje: number
  estatus: EnrollEstatus
}

export function CourseProgressBar({ porcentaje, estatus }: CourseProgressBarProps) {
  const completed = estatus === 1
  const clamped = Math.min(100, Math.max(0, porcentaje))

  return (
    <div className="flex flex-col gap-1">
      <div className="flex items-center justify-between text-xs text-gray-600">
        <span>{clamped}%</span>
        {completed && (
          <span className="inline-flex items-center rounded-full bg-green-100 px-2 py-0.5 text-xs font-medium text-green-800">
            Completed
          </span>
        )}
      </div>
      <div className="h-2 w-full overflow-hidden rounded-full bg-gray-200">
        <div
          className="h-full rounded-full bg-green-500 transition-all duration-300"
          style={{ width: `${clamped}%` }}
          role="progressbar"
          aria-valuenow={clamped}
          aria-valuemin={0}
          aria-valuemax={100}
        />
      </div>
    </div>
  )
}
