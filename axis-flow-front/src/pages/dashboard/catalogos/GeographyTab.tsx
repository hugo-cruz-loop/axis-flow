import { useMutation, useQueryClient } from '@tanstack/react-query'
import { SimpleCatalogTable } from '@/components/catalogos/SimpleCatalogTable'
import { LocationSelectors } from '@/components/presentational/LocationSelectors'
import { useCountries, useCitiesByState } from '@/hooks/api/useCatalogos'
import { useCascadingLocation } from '@/hooks/useCascadingLocation'
import {
  createCountry,
  deleteCountry,
  createState,
  deleteState,
  createCity,
  deleteCity,
} from '@/api/catalogosClient'
import type { State } from '@/types/catalogos'

export function GeographyTab() {
  const queryClient = useQueryClient()

  const { data: countries = [], isLoading: loadingCountries } = useCountries()
  const {
    selectedCountryId,
    selectedStateId,
    filteredStates,
    loadingStates,
    loadingCities,
    handleCountryChange,
    handleStateChange,
    handleCityChange,
  } = useCascadingLocation()

  const { data: citiesByState = [], isLoading: loadingCitiesByState } = useCitiesByState(
    selectedStateId ?? undefined,
  )

  const addCountry = useMutation({
    mutationFn: createCountry,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['catalogos', 'paises'] }),
  })

  const delCountry = useMutation({
    mutationFn: deleteCountry,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['catalogos', 'paises'] }),
  })

  const addState = useMutation({
    mutationFn: (d: { code: string; name: string }) =>
      createState({ code: d.code, name: d.name, country_id: selectedCountryId ?? undefined } as Partial<State>),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['catalogos', 'estados'] }),
  })

  const delState = useMutation({
    mutationFn: deleteState,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['catalogos', 'estados'] }),
  })

  const addCity = useMutation({
    mutationFn: (d: { code: string; name: string }) =>
      createCity({ name: d.name, state_id: selectedStateId ?? undefined }),
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: ['catalogos', 'ciudades', 'estado', selectedStateId] }),
  })

  const delCity = useMutation({
    mutationFn: deleteCity,
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: ['catalogos', 'ciudades', 'estado', selectedStateId] }),
  })

  return (
    <div className="space-y-8">
      <section>
        <h2 className="mb-3 text-base font-semibold text-slate-800">Countries</h2>
        <SimpleCatalogTable
          items={countries}
          isLoading={loadingCountries}
          onAdd={d => addCountry.mutateAsync(d)}
          onDelete={id => delCountry.mutateAsync(id)}
        />
      </section>

      <section>
        <div className="mb-3 flex flex-wrap items-center gap-4">
          <h2 className="text-base font-semibold text-slate-800">States</h2>
          <LocationSelectors
            countries={countries}
            states={[]}
            cities={[]}
            selectedCountryId={selectedCountryId}
            selectedStateId={null}
            selectedCityId={null}
            onCountryChange={handleCountryChange}
            onStateChange={() => null}
            onCityChange={() => null}
            loadingStates={loadingStates}
          />
        </div>
        <SimpleCatalogTable
          items={filteredStates}
          isLoading={loadingStates}
          onAdd={d => addState.mutateAsync(d)}
          onDelete={id => delState.mutateAsync(id)}
          addDisabled={!selectedCountryId}
        />
      </section>

      <section>
        <div className="mb-3 flex flex-wrap items-center gap-4">
          <h2 className="text-base font-semibold text-slate-800">Cities</h2>
          <LocationSelectors
            countries={countries}
            states={filteredStates}
            cities={[]}
            selectedCountryId={selectedCountryId}
            selectedStateId={selectedStateId}
            selectedCityId={null}
            onCountryChange={handleCountryChange}
            onStateChange={handleStateChange}
            onCityChange={handleCityChange}
            loadingStates={loadingStates}
            loadingCities={loadingCities}
          />
        </div>
        <SimpleCatalogTable
          items={citiesByState.map(c => ({ ...c, code: String(c.id) }))}
          isLoading={loadingCitiesByState}
          onAdd={d => addCity.mutateAsync(d)}
          onDelete={id => delCity.mutateAsync(id)}
          addDisabled={!selectedStateId}
        />
      </section>
    </div>
  )
}
