import { BarChart3 } from 'lucide-react'
import { DashboardLayout } from '@/layouts/DashboardLayout'

export function AnalyticsPage() {
  return (
    <DashboardLayout>
      <div className="flex min-h-[60vh] items-center justify-center">
        <div className="rounded-xl border border-slate-200 bg-white px-12 py-16 text-center shadow-sm">
          <div className="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-full bg-indigo-100">
            <BarChart3 className="h-7 w-7 text-indigo-600" />
          </div>
          <h2 className="text-xl font-semibold text-slate-900">Analytics</h2>
          <p className="mt-2 text-sm text-slate-500">
            Usage charts and reports are coming soon.
          </p>
        </div>
      </div>
    </DashboardLayout>
  )
}
