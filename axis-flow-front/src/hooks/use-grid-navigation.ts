import { useEffect, useRef } from 'react'

interface GridNavigationOptions {
  cols: number
}

export const useGridNavigation = ({ cols }: GridNavigationOptions) => {
  const containerRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const container = containerRef.current
    if (!container) return

    const handleKeyDown = (e: KeyboardEvent) => {
      const activeElement = document.activeElement as HTMLElement
      if (!activeElement || !container.contains(activeElement)) return

      // Extract all grid cells (focusable widget wrappers)
      const cells = Array.from(
        container.querySelectorAll('[role="gridcell"]')
      ) as HTMLElement[]

      const index = cells.indexOf(activeElement)
      if (index === -1) return

      const row = Math.floor(index / cols)
      const totalCells = cells.length

      let nextIndex = index

      switch (e.key) {
        case 'ArrowRight':
          nextIndex = index + 1 < totalCells ? index + 1 : index
          e.preventDefault()
          break;
        case 'ArrowLeft':
          nextIndex = index - 1 >= 0 ? index - 1 : index
          e.preventDefault()
          break;
        case 'ArrowDown':
          nextIndex = index + cols < totalCells ? index + cols : index
          e.preventDefault()
          break;
        case 'ArrowUp':
          nextIndex = index - cols >= 0 ? index - cols : index
          e.preventDefault()
          break;
        case 'Home':
          nextIndex = row * cols
          e.preventDefault()
          break;
        case 'End':
          nextIndex = Math.min((row + 1) * cols - 1, totalCells - 1)
          e.preventDefault()
          break;
        default:
          return
      }

      if (cells[nextIndex]) {
        // Manage focus ring
        cells[nextIndex].setAttribute('tabindex', '0')
        cells[nextIndex].focus()

        // Remove tabindex for non-focused cells
        cells.forEach((cell, idx) => {
          if (idx !== nextIndex) {
            cell.setAttribute('tabindex', '-1')
          }
        })
      }
    }

    // Initialize all grid cells on mount/updates
    const cells = Array.from(
      container.querySelectorAll('[role="gridcell"]')
    ) as HTMLElement[]
    cells.forEach((cell, idx) => {
      if (idx === 0) {
        cell.setAttribute('tabindex', '0')
      } else {
        cell.setAttribute('tabindex', '-1')
      }
    })

    container.addEventListener('keydown', handleKeyDown)
    return () => {
      container.removeEventListener('keydown', handleKeyDown)
    }
  }, [cols])

  return containerRef
}
