import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { EvaluationSchema, type EvaluationInput } from '../../schemas/validation'
import { useSubmitEvaluation } from '../../api/queries'

interface ScorecardFormProps {
  postulacionId: string
  onSuccess: () => void
}

const RATING_FIELDS: { name: keyof EvaluationInput; label: string }[] = [
  { name: 'puntualidad', label: 'Punctuality' },
  { name: 'cortesia', label: 'Courtesy' },
  { name: 'soft_skills', label: 'Soft skills' },
]

export function ScorecardForm({ postulacionId, onSuccess }: ScorecardFormProps) {
  const { mutate: submit, isPending } = useSubmitEvaluation()

  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<EvaluationInput>({
    resolver: zodResolver(EvaluationSchema),
    defaultValues: { postulacion_id: postulacionId },
  })

  const onSubmit = (data: EvaluationInput) => {
    submit(data, { onSuccess })
  }

  return (
    <form onSubmit={handleSubmit(onSubmit)} className="space-y-4" noValidate>
      <input type="hidden" {...register('postulacion_id')} />

      {RATING_FIELDS.map(({ name, label }) => (
        <div key={name}>
          <label htmlFor={name} className="mb-1 block text-sm font-medium text-gray-700">
            {label} (1–5)
          </label>
          <input
            id={name}
            type="number"
            min={1}
            max={5}
            {...register(name, { valueAsNumber: true })}
            className="w-24 rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
          />
          {errors[name] && (
            <p className="mt-1 text-xs text-red-600">{errors[name]?.message}</p>
          )}
        </div>
      ))}

      <div>
        <label htmlFor="comentarios" className="mb-1 block text-sm font-medium text-gray-700">
          Comments (optional)
        </label>
        <textarea
          id="comentarios"
          rows={3}
          {...register('comentarios')}
          className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
        />
        {errors.comentarios && (
          <p className="mt-1 text-xs text-red-600">{errors.comentarios.message}</p>
        )}
      </div>

      <button
        type="submit"
        disabled={isPending}
        className="w-full rounded-md bg-blue-600 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50"
      >
        {isPending ? 'Saving…' : 'Submit evaluation'}
      </button>
    </form>
  )
}
