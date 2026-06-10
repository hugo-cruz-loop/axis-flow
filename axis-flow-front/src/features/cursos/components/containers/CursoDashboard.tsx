import { useNavigate } from 'react-router-dom'
import { useAuthStore } from '@/store/authStore'
import type { User } from '@/types/users'
import { useCursos, useEnrollments, useEnroll } from '../../hooks/useCursos'
import { CourseTable } from '../presentational/CourseTable'
import { CourseProgressBar } from '../presentational/CourseProgressBar'
import type { Enrollment } from '../../types'

export function CursoDashboard() {
  const user = useAuthStore((s) => s.user)
  // empresa_id comes from JWT claims stored in Zustand — never hardcoded
  const empleadoId = (user as unknown as User & { empleado_id?: number })?.empleado_id ?? 0

  const { data: courses = [], isLoading: loadingCursos } = useCursos()
  const { data: enrollments = [] } = useEnrollments(empleadoId)
  const enroll = useEnroll()
  const navigate = useNavigate()

  const enrollmentByCurso = enrollments.reduce<Record<number, Enrollment>>(
    (acc, e) => ({ ...acc, [e.curso_id]: e }),
    {},
  )

  const handleSelect = (id: number) => {
    navigate(`/cursos/${id}/player`)
  }

  const handleEnroll = (cursoId: number) => {
    enroll.mutate(cursoId)
  }

  return (
    <div className="flex flex-col gap-6 p-6">
      <h1 className="text-2xl font-semibold text-gray-900">Courses</h1>

      <div className="overflow-x-auto rounded-lg border border-gray-200 bg-white">
        {loadingCursos ? (
          <CourseTable courses={[]} isLoading onSelect={() => {}} />
        ) : (
          <div>
            <CourseTable courses={courses} onSelect={handleSelect} />
            {/* Enrollment overlay rows */}
            {courses.map((course) => {
              const enrollment = enrollmentByCurso[course.id]
              if (!enrollment) {
                const isLocked =
                  course.prerequisito_id !== undefined &&
                  course.prerequisito_id !== null &&
                  !enrollmentByCurso[course.prerequisito_id]

                return (
                  <div key={`enroll-${course.id}`} className="flex items-center gap-2 border-t px-4 py-2">
                    <span className="text-sm text-gray-500 flex-1">{course.titulo}</span>
                    <button
                      type="button"
                      disabled={isLocked || enroll.isPending}
                      onClick={() => handleEnroll(course.id)}
                      title={isLocked ? 'Complete prerequisite course first' : undefined}
                      className={[
                        'rounded-md px-3 py-1 text-xs font-medium',
                        isLocked
                          ? 'cursor-not-allowed bg-gray-100 text-gray-400'
                          : 'bg-blue-600 text-white hover:bg-blue-700',
                      ].join(' ')}
                    >
                      Enroll
                    </button>
                  </div>
                )
              }

              return (
                <div key={`progress-${course.id}`} className="flex items-center gap-4 border-t px-4 py-2">
                  <span className="w-48 truncate text-sm text-gray-700">{course.titulo}</span>
                  <div className="flex-1">
                    <CourseProgressBar
                      porcentaje={enrollment.avance_porcentaje}
                      estatus={enrollment.estatus}
                    />
                  </div>
                </div>
              )
            })}
          </div>
        )}
      </div>
    </div>
  )
}
