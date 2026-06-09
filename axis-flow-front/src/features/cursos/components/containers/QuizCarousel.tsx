import { useState } from 'react'
import { useParams } from 'react-router-dom'
import { useExamen, useResolverExamen } from '../../hooks/useCursos'
import { QuizQuestionCard } from '../presentational/QuizQuestionCard'
import { downloadCertificate } from '../../api/cursosClient'
import type { ResultadoExamen } from '../../types'

export function QuizCarousel() {
  const { cursoId } = useParams<{ cursoId: string }>()
  const id = Number(cursoId)
  const { data: examen, isLoading } = useExamen(id)
  const resolverExamen = useResolverExamen()

  const [currentIndex, setCurrentIndex] = useState(0)
  const [answers, setAnswers] = useState<Record<number, number>>({}) // pregunta_id → opcion_id
  const [resultado, setResultado] = useState<ResultadoExamen | null>(null)

  if (isLoading) {
    return <div className="flex items-center justify-center py-20 text-gray-500">Loading exam…</div>
  }

  if (!examen) {
    return <div className="flex items-center justify-center py-20 text-gray-500">No exam found.</div>
  }

  const preguntas = examen.preguntas
  const currentPregunta = preguntas[currentIndex]
  const isFirst = currentIndex === 0
  const isLast = currentIndex === preguntas.length - 1
  const allAnswered = preguntas.every((p) => answers[p.id] !== undefined)

  const handleSelect = (opcionId: number) => {
    setAnswers((prev) => ({ ...prev, [currentPregunta.id]: opcionId }))
  }

  const handleSubmit = () => {
    const respuestas = preguntas.map((p) => ({
      pregunta_id: p.id,
      opcion_id: answers[p.id],
    }))
    resolverExamen.mutate(
      { examen_id: examen.id, respuestas },
      { onSuccess: (data) => setResultado(data) },
    )
  }

  const handleDownloadCertificate = async () => {
    if (!resultado) return
    const blob = await downloadCertificate(resultado.examen_id)
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `certificate-${resultado.examen_id}.pdf`
    a.click()
    URL.revokeObjectURL(url)
  }

  if (resultado) {
    return (
      <div className="flex flex-col items-center gap-6 p-10">
        <h1 className="text-2xl font-semibold text-gray-900">Exam Result</h1>
        <p className="text-lg text-gray-700">
          Score: <span className="font-bold">{resultado.calificacion}</span>
        </p>
        <p className={resultado.aprobado ? 'text-green-600 font-medium' : 'text-red-600 font-medium'}>
          {resultado.aprobado ? 'Passed!' : 'Not passed. Try again.'}
        </p>
        {resultado.aprobado && (
          <button
            type="button"
            onClick={handleDownloadCertificate}
            className="rounded-md bg-blue-600 px-6 py-2 text-sm font-medium text-white hover:bg-blue-700"
          >
            Download Certificate
          </button>
        )}
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-6 p-6 max-w-2xl mx-auto">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-semibold text-gray-900">{examen.titulo}</h1>
        <span className="text-sm text-gray-500">
          {currentIndex + 1} / {preguntas.length}
        </span>
      </div>

      <div className="rounded-lg border border-gray-200 bg-white p-6">
        <QuizQuestionCard
          pregunta={currentPregunta}
          selectedOpcionId={answers[currentPregunta.id]}
          onSelect={handleSelect}
        />
      </div>

      <div className="flex items-center justify-between">
        <button
          type="button"
          onClick={() => setCurrentIndex((i) => i - 1)}
          disabled={isFirst}
          className="rounded-md border border-gray-300 px-4 py-2 text-sm text-gray-700 hover:bg-gray-50 disabled:opacity-40"
        >
          Previous
        </button>

        {isLast ? (
          <button
            type="button"
            onClick={handleSubmit}
            disabled={!allAnswered || resolverExamen.isPending}
            className="rounded-md bg-blue-600 px-6 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50"
          >
            Submit
          </button>
        ) : (
          <button
            type="button"
            onClick={() => setCurrentIndex((i) => i + 1)}
            className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700"
          >
            Next
          </button>
        )}
      </div>
    </div>
  )
}
