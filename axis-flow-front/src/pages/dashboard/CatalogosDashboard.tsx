import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { DashboardLayout } from '@/layouts/DashboardLayout'
import { SimpleCatalogTable } from '@/components/catalogos/SimpleCatalogTable'
import {
  useBanks,
  useWorkflowStatuses,
  useCatalogServices,
  useJobCategories,
  useJobTypes,
} from '@/hooks/api/useCatalogos'
import {
  createBank,
  deleteBank,
  createWorkflowStatus,
  createJobCategory,
  createJobType,
} from '@/api/catalogosClient'
import { GeographyTab } from './catalogos/GeographyTab'

type TabKey = 'geography' | 'financial' | 'operational' | 'hr'

interface Tab {
  key: TabKey
  label: string
}

const TABS: Tab[] = [
  { key: 'geography', label: 'Geography' },
  { key: 'financial', label: 'Financial' },
  { key: 'operational', label: 'Operational' },
  { key: 'hr', label: 'HR / Jobs' },
]

function useSimpleMutation<TData>(
  mutationFn: (data: { code: string; name: string }) => Promise<TData>,
  queryKey: string[],
  onError?: (err: unknown) => void,
) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn,
    onSuccess: () => queryClient.invalidateQueries({ queryKey }),
    onError,
  })
}

function useDeleteMutation(mutationFn: (id: number) => Promise<unknown>, queryKey: string[]) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn,
    onSuccess: () => queryClient.invalidateQueries({ queryKey }),
  })
}

function getErrorMessage(err: unknown): string {
  if (err && typeof err === 'object' && 'response' in err) {
    const res = (err as { response?: { status?: number; data?: { code?: string } } }).response
    if (res?.status === 409) {
      if (res.data?.code === 'HAS_DEPENDENTS') return 'Cannot delete — has dependents'
      return 'Duplicate code or name'
    }
  }
  return 'An error occurred'
}

export function CatalogosDashboard() {
  const [activeTab, setActiveTab] = useState<TabKey>('geography')
  const [errorMsg, setErrorMsg] = useState<string | null>(null)

  const handleError = (err: unknown) => setErrorMsg(getErrorMessage(err))

  // Financial
  const { data: banks = [], isLoading: loadingBanks } = useBanks()
  const addBank = useSimpleMutation(createBank, ['catalogos', 'bancos'], handleError)
  const delBank = useDeleteMutation(deleteBank, ['catalogos', 'bancos'])

  // Operational
  const { data: statuses = [], isLoading: loadingStatuses } = useWorkflowStatuses()
  const addStatus = useSimpleMutation(
    (d) => createWorkflowStatus({ code: d.code, name: d.name }),
    ['catalogos', 'statuses'],
    handleError,
  )
  const { data: services = [], isLoading: loadingServices } = useCatalogServices()

  // HR
  const { data: jobCats = [], isLoading: loadingJobCats } = useJobCategories()
  const addJobCat = useSimpleMutation(
    (d) => createJobCategory({ nombre: d.name.toLowerCase() }),
    ['catalogos', 'categoriasBT'],
    handleError,
  )
  const { data: jobTypes = [], isLoading: loadingJobTypes } = useJobTypes()
  const addJobType = useSimpleMutation(
    (d) => createJobType({ nombre: d.name.toLowerCase() }),
    ['catalogos', 'tiposBT'],
    handleError,
  )

  return (
    <DashboardLayout>
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-slate-900">Catalogs</h1>
        <p className="mt-1 text-sm text-slate-500">Manage system reference data</p>
      </div>

      {errorMsg && (
        <div className="mb-4 flex items-center justify-between rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
          <span>{errorMsg}</span>
          <button onClick={() => setErrorMsg(null)} className="ml-4 text-red-500 hover:text-red-700">✕</button>
        </div>
      )}

      {/* Tab bar */}
      <div className="mb-6 flex gap-1 border-b border-slate-200">
        {TABS.map(tab => (
          <button
            key={tab.key}
            onClick={() => setActiveTab(tab.key)}
            className={`px-4 py-2 text-sm font-medium transition-colors ${
              activeTab === tab.key
                ? 'border-b-2 border-indigo-600 text-indigo-600'
                : 'text-slate-500 hover:text-slate-700'
            }`}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {/* Tab content */}
      {activeTab === 'geography' && <GeographyTab />}

      {activeTab === 'financial' && (
        <div className="space-y-6">
          <section>
            <h2 className="mb-3 text-base font-semibold text-slate-800">Banks</h2>
            <SimpleCatalogTable
              items={banks}
              isLoading={loadingBanks}
              onAdd={d => addBank.mutateAsync(d)}
              onDelete={id => delBank.mutateAsync(id)}
            />
          </section>
        </div>
      )}

      {activeTab === 'operational' && (
        <div className="space-y-6">
          <section>
            <h2 className="mb-3 text-base font-semibold text-slate-800">Workflow Statuses</h2>
            <SimpleCatalogTable
              items={statuses}
              isLoading={loadingStatuses}
              onAdd={d => addStatus.mutateAsync(d)}
              onDelete={() => Promise.resolve()}
            />
          </section>
          <section>
            <h2 className="mb-3 text-base font-semibold text-slate-800">Services</h2>
            <SimpleCatalogTable
              items={services}
              isLoading={loadingServices}
              onAdd={() => Promise.resolve()}
              onDelete={() => Promise.resolve()}
              addDisabled
            />
          </section>
        </div>
      )}

      {activeTab === 'hr' && (
        <div className="space-y-6">
          <section>
            <h2 className="mb-3 text-base font-semibold text-slate-800">Job Categories</h2>
            <SimpleCatalogTable
              items={jobCats}
              isLoading={loadingJobCats}
              onAdd={d => addJobCat.mutateAsync(d)}
              onDelete={() => Promise.resolve()}
            />
          </section>
          <section>
            <h2 className="mb-3 text-base font-semibold text-slate-800">Job Types</h2>
            <SimpleCatalogTable
              items={jobTypes}
              isLoading={loadingJobTypes}
              onAdd={d => addJobType.mutateAsync(d)}
              onDelete={() => Promise.resolve()}
            />
          </section>
        </div>
      )}
    </DashboardLayout>
  )
}
