import { useEffect, useState } from 'react'
import { ReportService } from '@/api/report-service'
import type { ReportFormat, ReportRunResponse } from '@/schemas/report-schemas'

interface ReportPreviewerProps {
  result: ReportRunResponse['data'] | null
  format: ReportFormat
}

function useExpiryCountdown(expiresAt: string | undefined): number {
  const [seconds, setSeconds] = useState<number>(() => {
    if (!expiresAt) return 0
    return Math.max(0, Math.floor((new Date(expiresAt).getTime() - Date.now()) / 1000))
  })

  useEffect(() => {
    if (!expiresAt) return
    const id = setInterval(() => {
      setSeconds(Math.max(0, Math.floor((new Date(expiresAt).getTime() - Date.now()) / 1000)))
    }, 1000)
    return () => clearInterval(id)
  }, [expiresAt])

  return seconds
}

export function ReportPreviewer({ result, format }: ReportPreviewerProps) {
  const secondsLeft = useExpiryCountdown(result?.expiresAt)

  if (!result) {
    return <p>No report generated yet</p>
  }

  const absoluteUrl = ReportService.getDownloadUrl(result.download_url)

  return (
    <div>
      <p>
        Link expires in: <strong>{secondsLeft}s</strong>
      </p>

      {format === 'pdf' ? (
        <iframe
          src={absoluteUrl}
          title="PDF Report Preview"
          width="100%"
          height="600"
          style={{ border: 'none' }}
        />
      ) : (
        <a
          href={absoluteUrl}
          download="report.xlsx"
          className="inline-flex items-center px-5 py-2.5 text-sm font-semibold text-white bg-emerald-600 rounded-lg hover:bg-emerald-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-emerald-500"
          aria-label="Download Excel report"
        >
          Download Excel Report
        </a>
      )}
    </div>
  )
}
