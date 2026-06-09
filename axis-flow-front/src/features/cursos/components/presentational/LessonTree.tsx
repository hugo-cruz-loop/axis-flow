import { useState } from 'react'
import type { Unidad } from '../../types'

interface LessonTreeProps {
  unidades: Unidad[]
  completedLeccionIds: Set<number>
  onSelectLeccion: (id: number) => void
}

function CheckIcon() {
  return (
    <svg
      className="h-4 w-4 text-green-500"
      viewBox="0 0 20 20"
      fill="currentColor"
      aria-hidden="true"
    >
      <path
        fillRule="evenodd"
        d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z"
        clipRule="evenodd"
      />
    </svg>
  )
}

export function LessonTree({ unidades, completedLeccionIds, onSelectLeccion }: LessonTreeProps) {
  const [openUnidades, setOpenUnidades] = useState<Set<number>>(
    () => new Set(unidades.map((u) => u.id)),
  )

  const toggleUnidad = (id: number) => {
    setOpenUnidades((prev) => {
      const next = new Set(prev)
      if (next.has(id)) {
        next.delete(id)
      } else {
        next.add(id)
      }
      return next
    })
  }

  return (
    <nav className="flex flex-col gap-1" aria-label="Course lessons">
      {unidades.map((unidad) => {
        const isOpen = openUnidades.has(unidad.id)
        return (
          <div key={unidad.id} className="rounded-md border border-gray-200">
            <button
              type="button"
              className="flex w-full items-center justify-between px-3 py-2 text-left text-sm font-medium text-gray-800 hover:bg-gray-50"
              onClick={() => toggleUnidad(unidad.id)}
              aria-expanded={isOpen}
            >
              <span>{unidad.titulo}</span>
              <span className="text-gray-400">{isOpen ? '▲' : '▼'}</span>
            </button>
            {isOpen && unidad.lecciones && (
              <ul className="divide-y divide-gray-100 border-t border-gray-100">
                {unidad.lecciones.map((leccion) => {
                  const done = completedLeccionIds.has(leccion.id)
                  return (
                    <li key={leccion.id}>
                      <button
                        type="button"
                        className="flex w-full items-center gap-2 px-4 py-2 text-left text-sm text-gray-700 hover:bg-gray-50"
                        onClick={() => onSelectLeccion(leccion.id)}
                      >
                        {done ? <CheckIcon /> : <span className="h-4 w-4" />}
                        <span className={done ? 'text-gray-500 line-through' : ''}>{leccion.titulo}</span>
                      </button>
                    </li>
                  )
                })}
              </ul>
            )}
          </div>
        )
      })}
    </nav>
  )
}
