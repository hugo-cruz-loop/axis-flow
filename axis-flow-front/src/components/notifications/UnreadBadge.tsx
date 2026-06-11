export interface UnreadBadgeProps {
  count: number
  className?: string
}

export function UnreadBadge({ count, className = '' }: UnreadBadgeProps) {
  if (count <= 0) return null

  const label = count > 99 ? '99+' : String(count)

  return (
    <span
      aria-label={`${count} notificaciones sin leer`}
      className={`inline-flex items-center justify-center rounded-full bg-red-500 text-white text-xs font-bold min-w-[1.25rem] h-5 px-1 ${className}`}
    >
      {label}
    </span>
  )
}
