import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { reportRunPayloadSchema } from '@/schemas/report-schemas'
import type { ReportRunPayload } from '@/schemas/report-schemas'

interface ReportExportControlCenterProps {
  reportType: string
  onGenerate: (data: ReportRunPayload) => void
  isPending: boolean
}

function defaultDateFrom(): string {
  const d = new Date()
  d.setDate(d.getDate() - 30)
  return d.toISOString().slice(0, 10)
}

function defaultDateTo(): string {
  return new Date().toISOString().slice(0, 10)
}

export function ReportExportControlCenter({
  reportType: _reportType,
  onGenerate,
  isPending,
}: ReportExportControlCenterProps) {
  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<ReportRunPayload>({
    resolver: zodResolver(reportRunPayloadSchema),
    defaultValues: {
      filters: {
        date_from: defaultDateFrom(),
        date_to: defaultDateTo(),
      },
      outputFormat: 'pdf',
    },
  })

  return (
    <form onSubmit={handleSubmit(onGenerate)} noValidate>
      <div>
        <label htmlFor="date_from">Date From</label>
        <input id="date_from" type="date" {...register('filters.date_from')} />
        {errors.filters?.date_from && (
          <span role="alert">{errors.filters.date_from.message}</span>
        )}
      </div>

      <div>
        <label htmlFor="date_to">Date To</label>
        <input id="date_to" type="date" {...register('filters.date_to')} />
        {errors.filters?.date_to && (
          <span role="alert">{errors.filters.date_to.message}</span>
        )}
      </div>

      <div>
        <label htmlFor="client_id">Client</label>
        <select
          id="client_id"
          {...register('filters.client_id', {
            setValueAs: (v: string) => (v === '' ? undefined : parseInt(v, 10)),
          })}
        >
          <option value="">All Clients</option>
          <option value="1">Client 1</option>
          <option value="2">Client 2</option>
          <option value="3">Client 3</option>
        </select>
        {errors.filters?.client_id && (
          <span role="alert">{errors.filters.client_id.message}</span>
        )}
      </div>

      <div>
        <label htmlFor="employee_id">Employee ID</label>
        <input
          id="employee_id"
          type="text"
          placeholder="Optional"
          {...register('filters.employee_id')}
        />
        {errors.filters?.employee_id && (
          <span role="alert">{errors.filters.employee_id.message}</span>
        )}
      </div>

      <fieldset>
        <legend>Output Format</legend>
        <label htmlFor="format_pdf">
          <input
            id="format_pdf"
            type="radio"
            value="pdf"
            {...register('outputFormat')}
          />
          PDF (Maroto Engine)
        </label>
        <label htmlFor="format_xlsx">
          <input
            id="format_xlsx"
            type="radio"
            value="xlsx"
            {...register('outputFormat')}
          />
          Excel (Excelize Engine)
        </label>
        {errors.outputFormat && (
          <span role="alert">{errors.outputFormat.message}</span>
        )}
      </fieldset>

      <button type="submit" disabled={isPending} aria-busy={isPending}>
        {isPending ? 'Generating…' : 'Generate and Export'}
      </button>
    </form>
  )
}
