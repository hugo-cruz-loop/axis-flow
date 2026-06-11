import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MatrixField } from '../components/presentational/MatrixField'

const rows = ['Inodoros', 'Lavabos', 'Espejos']
const columns = ['Bueno', 'Regular', 'Malo']

describe('MatrixField', () => {
  it('renders an accessible grid with role=grid, rows, and cells', () => {
    render(
      <MatrixField
        rows={rows}
        columns={columns}
        value={{}}
        onChange={() => {}}
        ariaLabel="Limpieza de baños"
      />,
    )
    expect(screen.getByRole('grid', { name: 'Limpieza de baños' })).toBeInTheDocument()
    // The header row is thead/tr; the data rows are tbody/tr. role="row"
    // matches both. The 3 data rows are the ones carrying role="row" on
    // the tr; the header is implicit.
    const grid = screen.getByRole('grid', { name: 'Limpieza de baños' })
    const dataRows = grid.querySelectorAll('tbody tr[role="row"]')
    expect(dataRows).toHaveLength(3)
    expect(screen.getAllByRole('gridcell')).toHaveLength(9) // 3 rows × 3 cols
  })

  it('has an accessible name via aria-label', () => {
    render(
      <MatrixField
        rows={rows}
        columns={columns}
        value={{}}
        onChange={() => {}}
        ariaLabel="Calidad de la limpieza"
      />,
    )
    expect(screen.getByLabelText('Calidad de la limpieza')).toBeInTheDocument()
  })

  it('clicking a cell calls onChange with the right (rowKey, columnKey) tuple', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    render(
      <MatrixField
        rows={rows}
        columns={columns}
        value={{}}
        onChange={onChange}
        ariaLabel="x"
      />,
    )
    // Each cell is a radio labeled with "Rate '{row}' as '{col}'"
    const inodorosBueno = screen.getByLabelText("Rate 'Inodoros' as 'Bueno'")
    await user.click(inodorosBueno)
    expect(onChange).toHaveBeenCalledWith('Inodoros', 'Bueno')
  })

  it('Pressing ArrowRight on a cell moves focus to the next column in the same row', async () => {
    const user = userEvent.setup()
    render(
      <MatrixField
        rows={rows}
        columns={columns}
        value={{}}
        onChange={() => {}}
        ariaLabel="x"
      />,
    )
    const inodorosBueno = screen.getByLabelText("Rate 'Inodoros' as 'Bueno'")
    inodorosBueno.focus()
    await user.keyboard('{ArrowRight}')
    expect(screen.getByLabelText("Rate 'Inodoros' as 'Regular'")).toHaveFocus()
  })

  it('Pressing ArrowDown moves focus to the same column in the next row', async () => {
    const user = userEvent.setup()
    render(
      <MatrixField
        rows={rows}
        columns={columns}
        value={{}}
        onChange={() => {}}
        ariaLabel="x"
      />,
    )
    const lavabosBueno = screen.getByLabelText("Rate 'Lavabos' as 'Bueno'")
    lavabosBueno.focus()
    await user.keyboard('{ArrowDown}')
    expect(screen.getByLabelText("Rate 'Espejos' as 'Bueno'")).toHaveFocus()
  })

  it('disabled=true prevents interaction (all radios disabled)', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    render(
      <MatrixField
        rows={rows}
        columns={columns}
        value={{}}
        onChange={onChange}
        disabled
        ariaLabel="x"
      />,
    )
    const radio = screen.getByLabelText("Rate 'Inodoros' as 'Bueno'")
    expect(radio).toBeDisabled()
    await user.click(radio)
    expect(onChange).not.toHaveBeenCalled()
  })

  it('reflects the current value (radio checked)', () => {
    render(
      <MatrixField
        rows={rows}
        columns={columns}
        value={{ Inodoros: 'Bueno', Lavabos: 'Malo' }}
        onChange={() => {}}
        ariaLabel="x"
      />,
    )
    expect(screen.getByLabelText("Rate 'Inodoros' as 'Bueno'")).toBeChecked()
    expect(screen.getByLabelText("Rate 'Inodoros' as 'Regular'")).not.toBeChecked()
    expect(screen.getByLabelText("Rate 'Lavabos' as 'Malo'")).toBeChecked()
  })
})
