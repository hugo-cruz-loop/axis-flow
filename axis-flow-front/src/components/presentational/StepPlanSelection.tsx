import { Check } from 'lucide-react'
import type { SubscriptionPlan } from '@/types/catalogos'
import { cn } from '@/lib/utils'

interface StepPlanSelectionProps {
  plans: SubscriptionPlan[]
  selectedPlanId: number | null
  onSelect: (planId: number) => void
  isLoading?: boolean
}

export function StepPlanSelection({
  plans,
  selectedPlanId,
  onSelect,
  isLoading = false,
}: StepPlanSelectionProps) {
  if (isLoading) {
    return (
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        {Array.from({ length: 4 }).map((_, i) => (
          <div
            key={i}
            className="h-36 animate-pulse rounded-xl border border-slate-200 bg-slate-100"
          />
        ))}
      </div>
    )
  }

  return (
    <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
      {plans.map((plan) => {
        const selected = plan.id === selectedPlanId
        return (
          <button
            key={plan.id}
            type="button"
            onClick={() => onSelect(plan.id)}
            className={cn(
              'relative flex flex-col items-start rounded-xl border-2 p-5 text-left transition-all',
              selected
                ? 'border-indigo-600 bg-indigo-50'
                : 'border-slate-200 bg-white hover:border-indigo-300',
            )}
          >
            {selected && (
              <span className="absolute right-3 top-3 flex h-5 w-5 items-center justify-center rounded-full bg-indigo-600">
                <Check className="h-3 w-3 text-white" />
              </span>
            )}
            <span className="text-sm font-semibold text-slate-500 uppercase tracking-wider">
              {plan.code}
            </span>
            <span className="mt-1 text-lg font-bold text-slate-900">{plan.name}</span>
            <span className="mt-2 text-2xl font-extrabold text-indigo-600">
              {new Intl.NumberFormat('en-US', {
                style: 'currency',
                currency: 'MXN',
              }).format(plan.amount)}
            </span>
            <button
              type="button"
              onClick={() => onSelect(plan.id)}
              className={cn(
                'mt-4 rounded-lg px-4 py-1.5 text-sm font-medium transition-colors',
                selected
                  ? 'bg-indigo-600 text-white'
                  : 'bg-slate-100 text-slate-700 hover:bg-indigo-600 hover:text-white',
              )}
            >
              {selected ? 'Selected' : 'Select'}
            </button>
          </button>
        )
      })}
    </div>
  )
}
