import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, fireEvent, screen } from '@testing-library/react'
import * as React from 'react'
import { useGridNavigation } from '../use-grid-navigation'

const TestComponent = ({ cols }: { cols: number }) => {
  const containerRef = useGridNavigation({ cols })
  return React.createElement(
    'div',
    { ref: containerRef, 'data-testid': 'grid-container' },
    React.createElement('div', { role: 'gridcell', 'data-testid': 'cell-0' }, 'Cell 0'),
    React.createElement('div', { role: 'gridcell', 'data-testid': 'cell-1' }, 'Cell 1'),
    React.createElement('div', { role: 'gridcell', 'data-testid': 'cell-2' }, 'Cell 2'),
    React.createElement('div', { role: 'gridcell', 'data-testid': 'cell-3' }, 'Cell 3'),
    React.createElement('div', { role: 'gridcell', 'data-testid': 'cell-4' }, 'Cell 4'),
    React.createElement('div', { role: 'gridcell', 'data-testid': 'cell-5' }, 'Cell 5')
  )
}

describe('useGridNavigation hook', () => {
  beforeEach(() => {
    document.body.innerHTML = ''
  })

  it('sets tabindex="0" on the first gridcell and tabindex="-1" on others by default on mount', () => {
    render(React.createElement(TestComponent, { cols: 3 }))

    const cell0 = screen.getByTestId('cell-0')
    const cell1 = screen.getByTestId('cell-1')

    expect(cell0.getAttribute('tabindex')).toBe('0')
    expect(cell1.getAttribute('tabindex')).toBe('-1')
  })

  it('navigates to the right on ArrowRight keydown', () => {
    render(React.createElement(TestComponent, { cols: 3 }))

    const cell0 = screen.getByTestId('cell-0')
    const cell1 = screen.getByTestId('cell-1')

    cell0.focus()

    fireEvent.keyDown(cell0, { key: 'ArrowRight' })

    expect(document.activeElement).toBe(cell1)
    expect(cell1.getAttribute('tabindex')).toBe('0')
    expect(cell0.getAttribute('tabindex')).toBe('-1')
  })

  it('navigates to the left on ArrowLeft keydown', () => {
    render(React.createElement(TestComponent, { cols: 3 }))

    const cell0 = screen.getByTestId('cell-0')
    const cell1 = screen.getByTestId('cell-1')

    cell1.focus()
    cell1.setAttribute('tabindex', '0')
    cell0.setAttribute('tabindex', '-1')

    fireEvent.keyDown(cell1, { key: 'ArrowLeft' })

    expect(document.activeElement).toBe(cell0)
    expect(cell0.getAttribute('tabindex')).toBe('0')
    expect(cell1.getAttribute('tabindex')).toBe('-1')
  })

  it('navigates down to next row on ArrowDown keydown', () => {
    render(React.createElement(TestComponent, { cols: 3 }))

    const cell0 = screen.getByTestId('cell-0')
    const cell3 = screen.getByTestId('cell-3')

    cell0.focus()

    fireEvent.keyDown(cell0, { key: 'ArrowDown' })

    expect(document.activeElement).toBe(cell3)
  })

  it('navigates up to previous row on ArrowUp keydown', () => {
    render(React.createElement(TestComponent, { cols: 3 }))

    const cell0 = screen.getByTestId('cell-0')
    const cell3 = screen.getByTestId('cell-3')

    cell3.focus()
    cell3.setAttribute('tabindex', '0')
    cell0.setAttribute('tabindex', '-1')

    fireEvent.keyDown(cell3, { key: 'ArrowUp' })

    expect(document.activeElement).toBe(cell0)
  })

  it('moves focus to the start of the row on Home keydown', () => {
    render(React.createElement(TestComponent, { cols: 3 }))

    const cell1 = screen.getByTestId('cell-1')
    const cell0 = screen.getByTestId('cell-0')

    cell1.focus()
    cell1.setAttribute('tabindex', '0')
    cell0.setAttribute('tabindex', '-1')

    fireEvent.keyDown(cell1, { key: 'Home' })

    expect(document.activeElement).toBe(cell0)
  })

  it('moves focus to the end of the row on End keydown', () => {
    render(React.createElement(TestComponent, { cols: 3 }))

    const cell1 = screen.getByTestId('cell-1')
    const cell2 = screen.getByTestId('cell-2')

    cell1.focus()
    cell1.setAttribute('tabindex', '0')
    cell2.setAttribute('tabindex', '-1')

    fireEvent.keyDown(cell1, { key: 'End' })

    expect(document.activeElement).toBe(cell2)
  })
})
