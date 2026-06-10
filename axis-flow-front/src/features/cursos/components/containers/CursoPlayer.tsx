import { useState } from 'react'
import { useParams } from 'react-router-dom'
import { useAuthStore } from '@/store/authStore'
import type { User } from '@/types/users'
import { useCursoContenido, useEnrollments, useMarkLeccionCompleta } from '../../hooks/useCursos'
import { LessonTree } from '../presentational/LessonTree'
import { HTML5VideoPlayer } from '../presentational/HTML5VideoPlayer'
import { NotesPanel } from '../presentational/NotesPanel'
import type { Leccion } from '../../types'

export function CursoPlayer() {
  const { cursoId } = useParams<{ cursoId: string }>()
  const id = Number(cursoId)
  const user = useAuthStore((s) => s.user)
  const empleadoId = (user as unknown as User & { empleado_id?: number })?.empleado_id ?? 0

  const { data: unidades = [], isLoading } = useCursoContenido(id)
  const { data: enrollments = [] } = useEnrollments(empleadoId)
  const markCompleta = useMarkLeccionCompleta()

  const [selectedLeccionId, setSelectedLeccionId] = useState<number | null>(null)

  const allLecciones = unidades.flatMap((u) => u.lecciones ?? [])
  const selectedLeccion: Leccion | undefined = allLecciones.find((l) => l.id === selectedLeccionId) ??
    allLecciones[0]

  // Build set of completed leccion IDs from enrollment avance
  // Server returns 100% avance and estatus=Completado when all lessons are done
  const enrollment = enrollments.find((e) => e.curso_id === id)
  const completedIds = new Set<number>()
  if (enrollment?.estatus === 1) {
    allLecciones.forEach((l) => completedIds.add(l.id))
  }

  const handleMarkCompleta = () => {
    if (!selectedLeccion) return
    markCompleta.mutate({ leccionId: selectedLeccion.id, cursoId: id })
  }

  const handleVideoEnded = () => {
    handleMarkCompleta()
  }

  if (isLoading) {
    return <div className="flex items-center justify-center py-20 text-gray-500">Loading…</div>
  }

  return (
    <div className="flex h-screen overflow-hidden">
      {/* Sidebar: Lesson tree */}
      <aside className="w-72 flex-shrink-0 overflow-y-auto border-r border-gray-200 bg-white p-4">
        <h2 className="mb-3 text-sm font-semibold text-gray-700 uppercase tracking-wide">
          Course Content
        </h2>
        <LessonTree
          unidades={unidades}
          completedLeccionIds={completedIds}
          onSelectLeccion={setSelectedLeccionId}
        />
      </aside>

      {/* Main area */}
      <main className="flex flex-1 flex-col overflow-y-auto bg-gray-50">
        {selectedLeccion ? (
          <div className="flex flex-col gap-4 p-6">
            <h1 className="text-xl font-semibold text-gray-900">{selectedLeccion.titulo}</h1>

            {/* Content */}
            {selectedLeccion.tipo === 1 && selectedLeccion.contenido_url && (
              <HTML5VideoPlayer url={selectedLeccion.contenido_url} onEnded={handleVideoEnded} />
            )}
            {selectedLeccion.tipo === 2 && selectedLeccion.contenido_url && (
              <iframe
                src={selectedLeccion.contenido_url}
                title={selectedLeccion.titulo}
                className="h-[60vh] w-full rounded-md border border-gray-200"
              />
            )}
            {selectedLeccion.tipo === 3 && selectedLeccion.contenido_url && (
              <iframe
                src={selectedLeccion.contenido_url}
                title={selectedLeccion.titulo}
                className="h-[60vh] w-full rounded-md border border-gray-200"
              />
            )}

            {/* Mark complete button */}
            <button
              type="button"
              onClick={handleMarkCompleta}
              disabled={completedIds.has(selectedLeccion.id) || markCompleta.isPending}
              className="self-start rounded-md bg-green-600 px-4 py-2 text-sm font-medium text-white hover:bg-green-700 disabled:cursor-not-allowed disabled:opacity-50"
            >
              {completedIds.has(selectedLeccion.id) ? 'Completed' : 'Mark as Complete'}
            </button>

            {/* Notes */}
            <NotesPanel leccionId={selectedLeccion.id} />
          </div>
        ) : (
          <div className="flex flex-1 items-center justify-center text-gray-500">
            Select a lesson to start
          </div>
        )}
      </main>
    </div>
  )
}
