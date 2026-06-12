import { useState } from 'react'
import { ReportExportControlCenter } from '@/components/reports/ReportExportControlCenter'
import { ReportPreviewer } from '@/components/reports/ReportPreviewer'
import { useGenerateReport } from '@/hooks/use-reports'
import type { ReportFormat, ReportRunPayload, ReportRunResponse } from '@/schemas/report-schemas'

interface ReportDashboardScreenProps {
  reportType: string
}

export function ReportDashboardScreen({ reportType }: ReportDashboardScreenProps) {
  const [result, setResult] = useState<ReportRunResponse['data'] | null>(null)
  const [format, setFormat] = useState<ReportFormat>('pdf')

  const { mutate, isPending } = useGenerateReport(reportType)

  const handleGenerate = (payload: ReportRunPayload) => {
    setFormat(payload.outputFormat)
    mutate(payload, { onSuccess: setResult })
  }

  return (
    <main>
      <h1>Report Dashboard</h1>
      <ReportExportControlCenter
        reportType={reportType}
        onGenerate={handleGenerate}
        isPending={isPending}
      />
      <ReportPreviewer result={result} format={format} />
    </main>
  )
}
