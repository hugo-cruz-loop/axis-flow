import type { ExamenPregunta } from '../../types'

interface QuizQuestionCardProps {
  pregunta: ExamenPregunta
  selectedOpcionId?: number
  onSelect: (opcionId: number) => void
}

export function QuizQuestionCard({ pregunta, selectedOpcionId, onSelect }: QuizQuestionCardProps) {
  return (
    <div className="flex flex-col gap-4">
      <p className="text-base font-medium text-gray-900">{pregunta.enunciado}</p>
      <ul className="flex flex-col gap-2" role="radiogroup" aria-label={pregunta.enunciado}>
        {pregunta.opciones.map((opcion) => {
          const isSelected = selectedOpcionId === opcion.id
          return (
            <li key={opcion.id}>
              <button
                type="button"
                role="radio"
                aria-checked={isSelected}
                onClick={() => onSelect(opcion.id)}
                className={[
                  'flex w-full items-center gap-3 rounded-lg border px-4 py-3 text-left text-sm transition-colors',
                  isSelected
                    ? 'border-blue-500 bg-blue-50 text-blue-800'
                    : 'border-gray-200 bg-white text-gray-700 hover:bg-gray-50',
                ].join(' ')}
              >
                <span
                  className={[
                    'flex h-4 w-4 flex-shrink-0 items-center justify-center rounded-full border-2',
                    isSelected ? 'border-blue-500 bg-blue-500' : 'border-gray-400 bg-white',
                  ].join(' ')}
                >
                  {isSelected && <span className="h-2 w-2 rounded-full bg-white" />}
                </span>
                {opcion.texto}
              </button>
            </li>
          )
        })}
      </ul>
    </div>
  )
}
