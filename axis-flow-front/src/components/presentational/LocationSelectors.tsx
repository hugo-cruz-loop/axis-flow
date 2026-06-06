import type { Country, State, City } from '@/types/catalogos'

interface LocationSelectorsProps {
  countries: Country[] | null | undefined
  states: State[] | null | undefined
  cities: City[] | null | undefined
  selectedCountryId: number | null
  selectedStateId: number | null
  selectedCityId: number | null
  onCountryChange: (id: number | null) => void
  onStateChange: (id: number | null) => void
  onCityChange: (id: number | null) => void
  loadingStates?: boolean
  loadingCities?: boolean
  disabled?: boolean
}

const selectClass =
  'h-9 rounded-lg border border-slate-300 bg-white px-3 text-sm text-slate-700 focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500 disabled:cursor-not-allowed disabled:opacity-50'

export function LocationSelectors({
  countries,
  states,
  cities,
  selectedCountryId,
  selectedStateId,
  selectedCityId,
  onCountryChange,
  onStateChange,
  onCityChange,
  loadingStates = false,
  loadingCities = false,
  disabled = false,
}: LocationSelectorsProps) {
  const safeCountries = countries ?? []
  const safeStates = states ?? []
  const safeCities = cities ?? []

  return (
    <div className="flex flex-wrap items-center gap-2">
      <select
        value={selectedCountryId ?? ''}
        onChange={e => onCountryChange(e.target.value ? Number(e.target.value) : null)}
        disabled={disabled}
        className={selectClass}
        aria-label="Country"
      >
        <option value="">All Countries</option>
        {safeCountries.map(c => (
          <option key={c.id} value={c.id}>{c.name}</option>
        ))}
      </select>

      <select
        value={selectedStateId ?? ''}
        onChange={e => onStateChange(e.target.value ? Number(e.target.value) : null)}
        disabled={disabled || !selectedCountryId || loadingStates}
        className={selectClass}
        aria-label="State"
      >
        <option value="">All States</option>
        {safeStates.map(s => (
          <option key={s.id} value={s.id}>{s.name}</option>
        ))}
      </select>

      <select
        value={selectedCityId ?? ''}
        onChange={e => onCityChange(e.target.value ? Number(e.target.value) : null)}
        disabled={disabled || !selectedStateId || loadingCities}
        className={selectClass}
        aria-label="City"
      >
        <option value="">All Cities</option>
        {safeCities.map(c => (
          <option key={c.id} value={c.id}>{c.name}</option>
        ))}
      </select>
    </div>
  )
}
