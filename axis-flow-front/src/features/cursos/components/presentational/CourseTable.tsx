import type { Curso } from '../../types'

interface CourseTableProps {
  courses: Curso[]
  onSelect: (id: number) => void
  isLoading?: boolean
}

function EstatusBadge({ estatus }: { estatus: Curso['estatus'] }) {
  const isPublic = estatus === 1
  return (
    <span
      className={[
        'inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium',
        isPublic
          ? 'bg-green-100 text-green-800'
          : 'bg-gray-100 text-gray-600',
      ].join(' ')}
    >
      {isPublic ? 'Public' : 'Private'}
    </span>
  )
}

export function CourseTable({ courses, onSelect, isLoading = false }: CourseTableProps) {
  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-12 text-gray-500 text-sm">
        Loading courses…
      </div>
    )
  }

  if (courses.length === 0) {
    return (
      <div className="flex items-center justify-center py-12 text-gray-500 text-sm">
        No courses found.
      </div>
    )
  }

  return (
    <div className="overflow-x-auto">
      <table className="min-w-full divide-y divide-gray-200 text-sm">
        <thead className="bg-gray-50">
          <tr>
            <th className="px-4 py-3 text-left font-medium text-gray-600">Title</th>
            <th className="px-4 py-3 text-left font-medium text-gray-600">Category</th>
            <th className="px-4 py-3 text-left font-medium text-gray-600">Duration</th>
            <th className="px-4 py-3 text-left font-medium text-gray-600">Status</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-gray-100 bg-white">
          {courses.map((course) => (
            <tr
              key={course.id}
              className="cursor-pointer hover:bg-gray-50 transition-colors"
              onClick={() => onSelect(course.id)}
            >
              <td className="px-4 py-3 font-medium text-gray-900">{course.titulo}</td>
              <td className="px-4 py-3 text-gray-600">{course.categoria_id}</td>
              <td className="px-4 py-3 text-gray-600">{course.duracion_minutos} min</td>
              <td className="px-4 py-3">
                <EstatusBadge estatus={course.estatus} />
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
