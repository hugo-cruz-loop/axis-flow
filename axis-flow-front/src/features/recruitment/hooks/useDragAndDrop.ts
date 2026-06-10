import { useState } from 'react'

export function useDragAndDrop(onDrop: (candidateId: string, newEstatus: number) => void): {
  draggingId: string | null
  onDragStart: (id: string) => void
  onDragOver: (e: React.DragEvent) => void
  onDrop: (e: React.DragEvent, estatus: number) => void
} {
  const [draggingId, setDraggingId] = useState<string | null>(null)

  const handleDragStart = (id: string) => {
    setDraggingId(id)
  }

  const handleDragOver = (e: React.DragEvent) => {
    e.preventDefault()
  }

  const handleDrop = (e: React.DragEvent, estatus: number) => {
    e.preventDefault()
    if (draggingId !== null) {
      onDrop(draggingId, estatus)
      setDraggingId(null)
    }
  }

  return {
    draggingId,
    onDragStart: handleDragStart,
    onDragOver: handleDragOver,
    onDrop: handleDrop,
  }
}
