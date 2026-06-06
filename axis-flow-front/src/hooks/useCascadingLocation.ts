import { useState } from 'react'
import { useStates, useCitiesByState } from './api/useCatalogos'

export function useCascadingLocation(onCityChange?: (cityId: number | null) => void) {
  const [selectedCountryId, setSelectedCountryId] = useState<number | null>(null)
  const [selectedStateId, setSelectedStateId] = useState<number | null>(null)
  const [selectedCityId, setSelectedCityId] = useState<number | null>(null)

  const { data: states = [], isLoading: loadingStates } = useStates(selectedCountryId ?? undefined)
  const { data: cities = [], isLoading: loadingCities } = useCitiesByState(selectedStateId ?? undefined)

  const filteredStates = states

  const handleCountryChange = (countryId: number | null) => {
    setSelectedCountryId(countryId)
    setSelectedStateId(null)
    setSelectedCityId(null)
    onCityChange?.(null)
  }

  const handleStateChange = (stateId: number | null) => {
    setSelectedStateId(stateId)
    setSelectedCityId(null)
    onCityChange?.(null)
  }

  const handleCityChange = (cityId: number | null) => {
    setSelectedCityId(cityId)
    onCityChange?.(cityId)
  }

  return {
    selectedCountryId,
    selectedStateId,
    selectedCityId,
    filteredStates,
    cities,
    loadingStates,
    loadingCities,
    handleCountryChange,
    handleStateChange,
    handleCityChange,
  }
}
