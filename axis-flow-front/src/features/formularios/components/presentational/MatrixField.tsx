import { useEffect, useRef } from 'react'

interface MatrixFieldProps {
  rows: string[]
  columns: string[]
  value: Record<string, string>
  onChange: (rowKey: string, columnKey: string) => void
  disabled?: boolean
  ariaLabel: string
}

/**
 * WCAG 2.1 AA composite-widget pattern for matrix inputs.
 *
 * - role="grid" wraps the table, role="row" wraps each row, role="gridcell"
 *   wraps each cell.
 * - Each cell is a radio (one selection per row) with an aria-label that
 *   reads "Rate '{row}' as '{col}'".
 * - ArrowRight / ArrowLeft move focus within the same row.
 * - ArrowDown / ArrowUp move focus to the same column in the next/prev row.
 * - Home / End jump to the first / last cell in the current row.
 * - PageDown / PageUp jump to the same column in the first / last row.
 */
export function MatrixField({
  rows,
  columns,
  value,
  onChange,
  disabled = false,
  ariaLabel,
}: MatrixFieldProps) {
  const gridRef = useRef<HTMLDivElement>(null)

  // Focus management: when the grid first mounts, focus the first radio so
  // keyboard users land in the matrix and can immediately arrow around.
  useEffect(() => {
    const grid = gridRef.current
    if (!grid) return
    const first = grid.querySelector<HTMLInputElement>('input[type="radio"]')
    if (first) first.focus()
    // Only on mount — we don't want to steal focus on every re-render.
  }, [])

  function getCell(rowIdx: number, colIdx: number): HTMLInputElement | null {
    const grid = gridRef.current
    if (!grid) return null
    // Cell order: rows are top-to-bottom; within each row, columns are left-to-right.
    // The flat list of inputs in document order matches that.
    const inputs = grid.querySelectorAll<HTMLInputElement>('input[type="radio"]')
    const idx = rowIdx * columns.length + colIdx
    return inputs[idx] ?? null
  }

  function handleKeyDown(
    e: React.KeyboardEvent<HTMLDivElement>,
    rowIdx: number,
    colIdx: number,
  ) {
    let nextRow = rowIdx
    let nextCol = colIdx
    let handled = true
    switch (e.key) {
      case 'ArrowRight':
        nextCol = Math.min(colIdx + 1, columns.length - 1)
        break
      case 'ArrowLeft':
        nextCol = Math.max(colIdx - 1, 0)
        break
      case 'ArrowDown':
        nextRow = Math.min(rowIdx + 1, rows.length - 1)
        break
      case 'ArrowUp':
        nextRow = Math.max(rowIdx - 1, 0)
        break
      case 'Home':
        nextCol = 0
        break
      case 'End':
        nextCol = columns.length - 1
        break
      case 'PageDown':
        nextRow = rows.length - 1
        break
      case 'PageUp':
        nextRow = 0
        break
      default:
        handled = false
    }
    if (handled) {
      e.preventDefault()
      const target = getCell(nextRow, nextCol)
      target?.focus()
    }
  }

  return (
    <div
      ref={gridRef}
      role="grid"
      aria-label={ariaLabel}
      aria-disabled={disabled || undefined}
      onKeyDown={(e) => {
        // Determine the cell that currently has focus to compute (rowIdx, colIdx).
        const grid = gridRef.current
        if (!grid) return
        const active = document.activeElement as HTMLElement | null
        if (!active || !grid.contains(active)) return
        const cell = active.closest('[data-row][data-col]') as HTMLElement | null
        if (!cell) return
        const rowIdx = Number(cell.getAttribute('data-row') ?? -1)
        const colIdx = Number(cell.getAttribute('data-col') ?? -1)
        if (rowIdx < 0 || colIdx < 0) return
        handleKeyDown(e, rowIdx, colIdx)
      }}
      className="w-full overflow-x-auto"
    >
      <table className="min-w-full divide-y divide-slate-200 text-sm">
        <thead>
          <tr className="bg-slate-50">
            <th scope="col" className="px-4 py-3 text-left font-bold text-slate-500 w-1/3">
              Item
            </th>
            {columns.map((col) => (
              <th
                key={col}
                scope="col"
                className="px-4 py-3 text-center font-bold text-slate-500"
              >
                {col}
              </th>
            ))}
          </tr>
        </thead>
        <tbody className="divide-y divide-slate-100 bg-white">
          {rows.map((row, rIdx) => {
            const rowName = `matrix-row-${rIdx}`
            return (
              <tr key={row} role="row" className="hover:bg-slate-50/50">
                <td className="px-4 py-3 font-semibold text-slate-700">{row}</td>
                {columns.map((col, cIdx) => {
                  const isChecked = value[row] === col
                  const inputId = `matrix-${rIdx}-${cIdx}`
                  return (
                    <td
                      key={col}
                      role="gridcell"
                      data-row={rIdx}
                      data-col={cIdx}
                      className="px-4 py-3 text-center"
                    >
                      <input
                        type="radio"
                        id={inputId}
                        name={rowName}
                        checked={isChecked}
                        disabled={disabled}
                        aria-label={`Rate '${row}' as '${col}'`}
                        onChange={() => onChange(row, col)}
                        className="h-4 w-4 text-indigo-600 focus:ring-indigo-500 border-slate-300"
                      />
                    </td>
                  )
                })}
              </tr>
            )
          })}
        </tbody>
      </table>
    </div>
  )
}
