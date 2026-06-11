import { useState } from 'react'
import { useSystemSettings, useUpdateSystemSetting } from '../api/queries'
import type { SystemSetting } from '../types'

export function SystemSettingsContainer() {
  const settings = useSystemSettings()
  const updateSetting = useUpdateSystemSetting()
  const [editingKey, setEditingKey] = useState<string | null>(null)
  const [draftValue, setDraftValue] = useState('')
  const [status, setStatus] = useState('')

  function startEditing(setting: SystemSetting) {
    setEditingKey(setting.clave_parametro)
    setDraftValue(setting.valor)
    setStatus('')
  }

  function saveSetting(clave: string) {
    updateSetting.mutate(
      { clave, payload: { valor: draftValue } },
      {
        onSuccess: () => {
          setEditingKey(null)
          setStatus('System setting saved. Cache invalidation requested.')
        },
      },
    )
  }

  return (
    <section className="space-y-6" aria-labelledby="system-settings-title">
      <div>
        <h1 id="system-settings-title" className="text-2xl font-bold text-slate-900">
          System Parameters
        </h1>
        <p className="mt-1 text-sm text-slate-500">
          Review and edit key-value parameters stored in Parametrizacion.
        </p>
      </div>
      <div role="status" aria-live="polite" className="min-h-5 text-sm text-slate-600">
        {updateSetting.isPending ? 'Saving setting and invalidating cache…' : status}
      </div>
      <div className="overflow-hidden rounded-xl border border-slate-200 bg-white shadow-sm">
        <table className="min-w-full divide-y divide-slate-200">
          <thead className="bg-slate-50">
            <tr>
              <th scope="col" className="px-4 py-3 text-left text-xs font-semibold uppercase text-slate-500">Key</th>
              <th scope="col" className="px-4 py-3 text-left text-xs font-semibold uppercase text-slate-500">Value</th>
              <th scope="col" className="px-4 py-3 text-left text-xs font-semibold uppercase text-slate-500">Description</th>
              <th scope="col" className="px-4 py-3 text-left text-xs font-semibold uppercase text-slate-500">Actions</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-200">
            {(settings.data ?? []).map((setting) => {
              const isEditing = editingKey === setting.clave_parametro
              return (
                <tr key={setting.clave_parametro}>
                  <th scope="row" className="px-4 py-3 text-left text-sm font-semibold text-slate-900">
                    {setting.clave_parametro}
                  </th>
                  <td className="px-4 py-3 text-sm text-slate-700">
                    {isEditing ? (
                      <input
                        aria-label={`Value for ${setting.clave_parametro}`}
                        value={draftValue}
                        onChange={(event) => setDraftValue(event.target.value)}
                        className="w-full rounded-md border border-slate-300 px-3 py-2 focus:outline-none focus-visible:ring-2 focus-visible:ring-indigo-500"
                      />
                    ) : (
                      setting.valor
                    )}
                  </td>
                  <td className="px-4 py-3 text-sm text-slate-500">{setting.descripcion}</td>
                  <td className="px-4 py-3 text-sm">
                    {isEditing ? (
                      <div className="flex gap-2">
                        <button
                          type="button"
                          aria-label={`Save ${setting.clave_parametro}`}
                          onClick={() => saveSetting(setting.clave_parametro)}
                          className="rounded-md bg-indigo-600 px-3 py-1.5 text-xs font-semibold text-white hover:bg-indigo-700 focus:outline-none focus-visible:ring-2 focus-visible:ring-indigo-500"
                        >
                          Save
                        </button>
                        <button
                          type="button"
                          aria-label={`Cancel ${setting.clave_parametro}`}
                          onClick={() => setEditingKey(null)}
                          className="rounded-md border border-slate-300 px-3 py-1.5 text-xs font-semibold text-slate-700 hover:bg-slate-50 focus:outline-none focus-visible:ring-2 focus-visible:ring-indigo-500"
                        >
                          Cancel
                        </button>
                      </div>
                    ) : (
                      <button
                        type="button"
                        aria-label={`Edit ${setting.clave_parametro}`}
                        onClick={() => startEditing(setting)}
                        className="rounded-md border border-slate-300 px-3 py-1.5 text-xs font-semibold text-slate-700 hover:bg-slate-50 focus:outline-none focus-visible:ring-2 focus-visible:ring-indigo-500"
                      >
                        Edit
                      </button>
                    )}
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>
        {settings.isLoading ? <p className="p-4 text-sm text-slate-500">Loading system settings…</p> : null}
        {!settings.isLoading && (settings.data?.length ?? 0) === 0 ? (
          <p className="p-4 text-sm text-slate-500">No system settings available.</p>
        ) : null}
      </div>
    </section>
  )
}
