import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { SLABadge } from '../components/presentational/SLABadge'

describe('SLABadge', () => {
  it('renders "overdue" when fecha_vigencia is in the past', () => {
    const past = new Date(Date.now() - 1000 * 60 * 60 * 24 * 2).toISOString()
    render(<SLABadge fechaVigencia={past} estatus={1} />)
    expect(screen.getByText('overdue')).toBeTruthy()
  })

  it('renders days remaining when deadline is in the future', () => {
    const future = new Date(Date.now() + 1000 * 60 * 60 * 24 * 5).toISOString()
    render(<SLABadge fechaVigencia={future} estatus={1} />)
    expect(screen.getByText(/d left/)).toBeTruthy()
  })

  it('renders nothing when estatus is 3 (Finalizado)', () => {
    const future = new Date(Date.now() + 1000 * 60 * 60 * 24 * 5).toISOString()
    const { container } = render(<SLABadge fechaVigencia={future} estatus={3} />)
    expect(container.firstChild).toBeNull()
  })
})
