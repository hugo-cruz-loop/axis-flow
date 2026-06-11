import { useState } from 'react'
import { EvaluationFrequenciesContainer } from '../containers/EvaluationFrequenciesContainer'
import { InactiveDaysContainer } from '../containers/InactiveDaysContainer'
import { SystemSettingsContainer } from '../containers/SystemSettingsContainer'

interface AdminSettingsPageProps {
  empresaId?: number
}

const TABS = [
  { id: 'evaluations', label: 'Evaluation Frequencies' },
  { id: 'calendar', label: 'Operational Calendar' },
  { id: 'system', label: 'System Parameters' },
] as const

type SettingsTab = (typeof TABS)[number]['id']

export function AdminSettingsPage({ empresaId = 1 }: AdminSettingsPageProps) {
  const [activeTab, setActiveTab] = useState<SettingsTab>('evaluations')

  return (
    <main className="mx-auto max-w-6xl space-y-6 px-4 py-8">
      <div>
        <p className="text-sm font-semibold uppercase tracking-wide text-indigo-600">Admin Settings</p>
        <h1 className="mt-1 text-3xl font-bold tracking-tight text-slate-950">Parametrizacion</h1>
        <p className="mt-2 text-sm text-slate-500">
          Manage evaluation frequencies, inactive days, and global system parameters.
        </p>
      </div>
      <div role="tablist" aria-label="Admin settings sections" className="flex flex-wrap gap-2 rounded-xl bg-slate-100 p-2">
        {TABS.map((tab) => (
          <button
            key={tab.id}
            type="button"
            role="tab"
            aria-selected={activeTab === tab.id}
            onClick={() => setActiveTab(tab.id)}
            className={`rounded-lg px-4 py-2 text-sm font-semibold focus:outline-none focus-visible:ring-2 focus-visible:ring-indigo-500 ${
              activeTab === tab.id ? 'bg-white text-slate-950 shadow-sm' : 'text-slate-600 hover:bg-white/70'
            }`}
          >
            {tab.label}
          </button>
        ))}
      </div>
      {activeTab === 'evaluations' ? <EvaluationFrequenciesContainer empresaId={empresaId} /> : null}
      {activeTab === 'calendar' ? <InactiveDaysContainer empresaId={empresaId} /> : null}
      {activeTab === 'system' ? <SystemSettingsContainer /> : null}
    </main>
  )
}
