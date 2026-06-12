import { useMutation } from '@tanstack/react-query'
import { ReportService } from '@/api/report-service'
import type { ReportRunPayload } from '@/schemas/report-schemas'

export const useGenerateReport = (type: string) => {
  return useMutation({
    mutationFn: (payload: ReportRunPayload) => ReportService.runReport(type, payload),
  })
}
