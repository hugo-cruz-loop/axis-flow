import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { LocationSelectors } from './LocationSelectors'

describe('LocationSelectors', () => {
  it('renders safely with null catalog collections', () => {
    render(
      <LocationSelectors
        countries={null}
        states={null}
        cities={null}
        selectedCountryId={null}
        selectedStateId={null}
        selectedCityId={null}
        onCountryChange={vi.fn()}
        onStateChange={vi.fn()}
        onCityChange={vi.fn()}
      />,
    )

    expect(screen.getByLabelText('Country')).toBeInTheDocument()
    expect(screen.getByLabelText('State')).toBeDisabled()
    expect(screen.getByLabelText('City')).toBeDisabled()
  })
})
