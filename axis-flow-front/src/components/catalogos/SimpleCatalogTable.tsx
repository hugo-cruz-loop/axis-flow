import { useState } from 'react'
import { Trash2, Plus } from 'lucide-react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'

interface SimpleCatalogTableProps<T extends { id: number; code: string; name: string }> {
  items: T[] | null | undefined
  isLoading: boolean
  onAdd: (data: { code: string; name: string }) => Promise<unknown>
  onDelete: (id: number) => Promise<unknown>
  extraColumns?: Array<{ header: string; accessor: keyof T }>
  addDisabled?: boolean
}

const addSchema = z.object({
  code: z.string().min(1, 'Code is required'),
  name: z.string().min(1, 'Name is required'),
})

type AddFormValues = z.infer<typeof addSchema>

export function SimpleCatalogTable<T extends { id: number; code: string; name: string }>({
  items,
  isLoading,
  onAdd,
  onDelete,
  extraColumns = [],
  addDisabled = false,
}: SimpleCatalogTableProps<T>) {
  const [deletingId, setDeletingId] = useState<number | null>(null)
  const [adding, setAdding] = useState(false)

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<AddFormValues>({ resolver: zodResolver(addSchema) })

  const handleAdd = async (data: AddFormValues) => {
    setAdding(true)
    try {
      await onAdd(data)
      reset()
    } finally {
      setAdding(false)
    }
  }

  const handleDelete = async (id: number) => {
    setDeletingId(id)
    try {
      await onDelete(id)
    } finally {
      setDeletingId(null)
    }
  }

  const safeItems = items ?? []
  const colCount = 2 + extraColumns.length + 1

  return (
    <div className="overflow-x-auto rounded-xl border border-slate-200 bg-white shadow-sm">
      <table className="min-w-full">
        <thead>
          <tr className="border-b border-slate-200 bg-slate-50">
            <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-slate-500">
              Code
            </th>
            <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-slate-500">
              Name
            </th>
            {extraColumns.map(col => (
              <th
                key={String(col.accessor)}
                className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-slate-500"
              >
                {col.header}
              </th>
            ))}
            <th className="w-16 px-4 py-3" />
          </tr>
        </thead>
        <tbody className="divide-y divide-slate-100">
          {isLoading
            ? Array.from({ length: 3 }).map((_, i) => (
                <tr key={i} data-testid="skeleton-row">
                  {Array.from({ length: colCount }).map((__, j) => (
                    <td key={j} className="px-4 py-3">
                      <div className="h-4 animate-pulse rounded bg-slate-100" />
                    </td>
                  ))}
                </tr>
              ))
            : safeItems.map(item => (
                <tr key={item.id} className="hover:bg-slate-50 transition-colors">
                  <td className="px-4 py-3 font-mono text-sm text-slate-700">{item.code}</td>
                  <td className="px-4 py-3 text-sm text-slate-900">{item.name}</td>
                  {extraColumns.map(col => (
                    <td key={String(col.accessor)} className="px-4 py-3 text-sm text-slate-700">
                      {String(item[col.accessor])}
                    </td>
                  ))}
                  <td className="px-4 py-3">
                    {!addDisabled && (
                      <button
                        onClick={() => void handleDelete(item.id)}
                        disabled={deletingId === item.id}
                        aria-label={`Delete ${item.name}`}
                        className="rounded p-1 text-slate-400 hover:bg-red-50 hover:text-red-600 disabled:opacity-40"
                      >
                        <Trash2 className="h-4 w-4" />
                      </button>
                    )}
                  </td>
                </tr>
              ))}

          {/* Add row */}
          {!addDisabled && (
            <tr className="bg-slate-50">
              <td className="px-4 py-2">
                <input
                  {...register('code')}
                  placeholder="Code"
                  className="h-8 w-full rounded border border-slate-300 px-2 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
                />
                {errors.code && (
                  <p className="mt-0.5 text-xs text-red-500">{errors.code.message}</p>
                )}
              </td>
              <td className="px-4 py-2">
                <input
                  {...register('name')}
                  placeholder="Name"
                  className="h-8 w-full rounded border border-slate-300 px-2 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
                />
                {errors.name && (
                  <p className="mt-0.5 text-xs text-red-500">{errors.name.message}</p>
                )}
              </td>
              {extraColumns.map(col => (
                <td key={String(col.accessor)} className="px-4 py-2" />
              ))}
              <td className="px-4 py-2">
                <button
                  onClick={() => void handleSubmit(handleAdd)()}
                  disabled={adding}
                  className="flex items-center gap-1 rounded bg-indigo-600 px-2 py-1 text-xs font-medium text-white hover:bg-indigo-700 disabled:opacity-50"
                >
                  <Plus className="h-3 w-3" />
                  Add
                </button>
              </td>
            </tr>
          )}

          {/* Empty state */}
          {!isLoading && safeItems.length === 0 && (
            <tr>
              <td colSpan={colCount} className="px-4 py-8 text-center text-sm text-slate-400">
                No records
              </td>
            </tr>
          )}
        </tbody>
      </table>
    </div>
  )
}
