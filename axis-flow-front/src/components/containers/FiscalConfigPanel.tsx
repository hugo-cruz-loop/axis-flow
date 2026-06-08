import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { Pencil } from 'lucide-react'
import { getFiscal, createFiscal, updateFiscal } from '@/api/empresasClient'
import { LogoUploader } from '@/components/presentational/LogoUploader'
import type { DatosFiscales } from '@/types/empresas'

interface FiscalConfigPanelProps {
  empresaId: number
}

const fiscalSchema = z.object({
  rfc: z
    .string()
    .min(12, 'RFC must be 12-13 characters')
    .max(13, 'RFC must be 12-13 characters')
    .transform((v) => v.toUpperCase()),
  razon_social: z.string().min(1, 'Required'),
  imss_patronal: z.string().optional(),
  repse: z.string().optional(),
})

type FiscalFormValues = z.infer<typeof fiscalSchema>

export function FiscalConfigPanel({ empresaId }: FiscalConfigPanelProps) {
  const queryClient = useQueryClient()
  const [editing, setEditing] = useState(false)

  const { data: fiscal, isLoading, isError, error } = useQuery({
    queryKey: ['empresas', 'fiscal', empresaId],
    queryFn: () => getFiscal(empresaId),
    retry: (failCount, err) => {
      const status = (err as { response?: { status: number } }).response?.status
      if (status === 404) return false
      return failCount < 3
    },
  })

  const isNotFound =
    isError &&
    (error as { response?: { status: number } }).response?.status === 404

  const showForm = editing || isNotFound

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<FiscalFormValues>({
    resolver: zodResolver(fiscalSchema),
    defaultValues: fiscal
      ? {
          rfc: fiscal.rfc,
          razon_social: fiscal.razon_social,
          imss_patronal: fiscal.imss_patronal ?? '',
          repse: fiscal.repse ?? '',
        }
      : undefined,
  })

  const mutation = useMutation({
    mutationFn: (data: Partial<DatosFiscales>) =>
      fiscal ? updateFiscal(empresaId, data) : createFiscal(empresaId, data),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['empresas', 'fiscal', empresaId] })
      setEditing(false)
    },
  })

  const onSubmit = (data: FiscalFormValues) => {
    mutation.mutate(data)
  }

  if (isLoading) {
    return (
      <div className="space-y-3">
        {Array.from({ length: 4 }).map((_, i) => (
          <div key={i} className="h-8 animate-pulse rounded-lg bg-slate-100" />
        ))}
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* Logo */}
      <div>
        <p className="mb-2 text-sm font-medium text-slate-700">Company Logo</p>
        <LogoUploader
          currentLogoUrl={fiscal?.logo_url}
          onFileSelect={() => {
            // File upload to server is TBD in a future PR
          }}
        />
      </div>

      {/* Fiscal data */}
      {!showForm && fiscal ? (
        <div className="rounded-xl border border-slate-200 bg-white p-6 shadow-sm">
          <div className="mb-4 flex items-center justify-between">
            <h3 className="font-semibold text-slate-900">Fiscal Data</h3>
            <button
              onClick={() => {
                reset({
                  rfc: fiscal.rfc,
                  razon_social: fiscal.razon_social,
                  imss_patronal: fiscal.imss_patronal ?? '',
                  repse: fiscal.repse ?? '',
                })
                setEditing(true)
              }}
              className="flex items-center gap-1.5 rounded-lg border border-slate-300 px-3 py-1.5 text-sm text-slate-700 hover:bg-slate-50"
            >
              <Pencil className="h-3.5 w-3.5" />
              Edit
            </button>
          </div>
          <dl className="grid grid-cols-2 gap-4 text-sm">
            <div>
              <dt className="text-slate-500">RFC</dt>
              <dd className="font-mono font-medium text-slate-900">{fiscal.rfc}</dd>
            </div>
            <div>
              <dt className="text-slate-500">Razón Social</dt>
              <dd className="font-medium text-slate-900">{fiscal.razon_social}</dd>
            </div>
            {fiscal.imss_patronal && (
              <div>
                <dt className="text-slate-500">IMSS Patronal</dt>
                <dd className="font-medium text-slate-900">{fiscal.imss_patronal}</dd>
              </div>
            )}
            {fiscal.repse && (
              <div>
                <dt className="text-slate-500">REPSE</dt>
                <dd className="font-medium text-slate-900">{fiscal.repse}</dd>
              </div>
            )}
          </dl>
        </div>
      ) : (
        <form
          onSubmit={(e) => void handleSubmit(onSubmit)(e)}
          className="rounded-xl border border-slate-200 bg-white p-6 shadow-sm"
        >
          <h3 className="mb-4 font-semibold text-slate-900">
            {isNotFound ? 'Add Fiscal Data' : 'Edit Fiscal Data'}
          </h3>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <div>
              <label className="block text-sm font-medium text-slate-700">
                RFC <span className="text-red-500">*</span>
              </label>
              <input
                {...register('rfc')}
                className="mt-1 h-9 w-full rounded-lg border border-slate-300 px-3 font-mono text-sm uppercase focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
                maxLength={13}
              />
              {errors.rfc && (
                <p className="mt-1 text-xs text-red-500">{errors.rfc.message}</p>
              )}
            </div>

            <div>
              <label className="block text-sm font-medium text-slate-700">
                Razón Social <span className="text-red-500">*</span>
              </label>
              <input
                {...register('razon_social')}
                className="mt-1 h-9 w-full rounded-lg border border-slate-300 px-3 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
              />
              {errors.razon_social && (
                <p className="mt-1 text-xs text-red-500">{errors.razon_social.message}</p>
              )}
            </div>

            <div>
              <label className="block text-sm font-medium text-slate-700">
                IMSS Patronal
              </label>
              <input
                {...register('imss_patronal')}
                className="mt-1 h-9 w-full rounded-lg border border-slate-300 px-3 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
              />
            </div>

            <div>
              <label className="block text-sm font-medium text-slate-700">REPSE</label>
              <input
                {...register('repse')}
                className="mt-1 h-9 w-full rounded-lg border border-slate-300 px-3 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
              />
            </div>
          </div>

          {mutation.isError && (
            <p className="mt-3 text-sm text-red-600">
              Failed to save fiscal data. Please try again.
            </p>
          )}

          <div className="mt-6 flex gap-3">
            {editing && (
              <button
                type="button"
                onClick={() => setEditing(false)}
                className="rounded-lg border border-slate-300 px-4 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50"
              >
                Cancel
              </button>
            )}
            <button
              type="submit"
              disabled={mutation.isPending}
              className="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700 disabled:opacity-50"
            >
              {mutation.isPending ? 'Saving…' : 'Save'}
            </button>
          </div>
        </form>
      )}
    </div>
  )
}
