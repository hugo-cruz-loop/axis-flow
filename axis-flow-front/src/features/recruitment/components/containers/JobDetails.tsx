import type { Trabajo } from '../../types'
import { ApplicationForm } from '../presentational/ApplicationForm'

interface JobDetailsProps {
  trabajo: Trabajo
  onClose: () => void
}

export function JobDetails({ trabajo, onClose }: JobDetailsProps) {
  return (
    <>
      {/* Backdrop */}
      <div
        className="fixed inset-0 z-40 bg-black/40"
        onClick={onClose}
        aria-hidden="true"
      />

      {/* Side sheet */}
      <aside
        className="fixed inset-y-0 right-0 z-50 flex w-full max-w-md flex-col bg-white shadow-xl"
        role="complementary"
        aria-label={`Application for ${trabajo.titulo}`}
      >
        <div className="flex items-center justify-between border-b border-gray-200 px-4 py-3">
          <h2 className="text-base font-semibold text-gray-900">{trabajo.titulo}</h2>
          <button
            type="button"
            onClick={onClose}
            aria-label="Close"
            className="rounded-md p-1 text-gray-400 hover:text-gray-600"
          >
            ✕
          </button>
        </div>

        <div className="flex-1 overflow-y-auto p-4">
          <ApplicationForm
            trabajoId={trabajo.id}
            onSuccess={onClose}
          />
        </div>
      </aside>
    </>
  )
}
