interface EmptyStateProps {
  message?: string
}

export function EmptyState({ message = 'No items found.' }: EmptyStateProps) {
  return (
    <div
      data-testid="empty-state"
      className="flex flex-col items-center justify-center py-12 text-gray-500"
    >
      <p className="text-sm">{message}</p>
    </div>
  )
}
