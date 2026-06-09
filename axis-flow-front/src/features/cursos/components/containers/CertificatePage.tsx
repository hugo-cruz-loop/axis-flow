import { useParams } from 'react-router-dom'
import { useAuthStore } from '@/store/authStore'
import type { User } from '@/types/users'
import { useResultados } from '../../hooks/useCursos'
import { downloadCertificate } from '../../api/cursosClient'

export function CertificatePage() {
  const { examenId } = useParams<{ examenId: string }>()
  const id = Number(examenId)
  const user = useAuthStore((s) => s.user)
  const empleadoId = (user as unknown as User & { empleado_id?: number })?.empleado_id ?? 0

  const { data: resultados = [], isLoading } = useResultados(id, empleadoId)

  const passing = resultados.find((r) => r.aprobado)

  const handleDownload = async () => {
    if (!passing) return
    const blob = await downloadCertificate(passing.examen_id)
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `certificate-${passing.examen_id}.pdf`
    a.click()
    URL.revokeObjectURL(url)
  }

  if (isLoading) {
    return <div className="flex items-center justify-center py-20 text-gray-500">Loading…</div>
  }

  if (!passing) {
    return (
      <div className="flex items-center justify-center py-20 text-gray-500">
        No passing result found for this exam.
      </div>
    )
  }

  return (
    <div className="flex flex-col items-center gap-8 p-10">
      {/* Certificate preview */}
      <div className="w-full max-w-lg rounded-xl border-2 border-blue-200 bg-white p-10 shadow-md text-center">
        <p className="text-xs uppercase tracking-widest text-gray-400 mb-2">Certificate of Completion</p>
        <p className="text-sm text-gray-600 mb-1">This certifies that</p>
        <p className="text-xl font-bold text-gray-900 mb-1">
          {user?.first_name} {user?.last_name}
        </p>
        <p className="text-sm text-gray-600 mb-4">
          successfully completed the exam with a score of{' '}
          <span className="font-semibold">{passing.calificacion}</span>
        </p>
        <p className="text-xs text-gray-400">
          Date: {new Date(passing.created_at).toLocaleDateString()}
        </p>
      </div>

      <button
        type="button"
        onClick={handleDownload}
        className="rounded-md bg-blue-600 px-8 py-3 text-sm font-medium text-white hover:bg-blue-700"
      >
        Download PDF Certificate
      </button>
    </div>
  )
}
