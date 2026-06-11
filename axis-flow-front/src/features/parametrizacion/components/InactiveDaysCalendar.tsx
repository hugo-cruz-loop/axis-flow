import { useMemo, useRef, useState } from 'react'
import type { InactiveDay } from '../types'

interface InactiveDaysCalendarProps {
  inactiveDays: InactiveDay[]
  initialMonth?: Date
  onCreateInactiveDay: (fecha: string) => void
  onDeleteInactiveDay: (id: number, fecha: string) => void
}

const MONTH_FORMAT = new Intl.DateTimeFormat('en-US', { month: 'long', year: 'numeric' })
const DAY_FORMAT = new Intl.DateTimeFormat('en-US', {
  weekday: 'long',
  month: 'long',
  day: 'numeric',
  year: 'numeric',
})
const ANNOUNCE_DAY_FORMAT = new Intl.DateTimeFormat('en-US', {
  month: 'long',
  day: 'numeric',
  year: 'numeric',
})

function toISODate(year: number, month: number, day: number) {
  return `${year}-${String(month + 1).padStart(2, '0')}-${String(day).padStart(2, '0')}`
}

function daysInMonth(month: Date) {
  const year = month.getFullYear()
  const monthIndex = month.getMonth()
  return Array.from({ length: new Date(year, monthIndex + 1, 0).getDate() }, (_, index) =>
    new Date(year, monthIndex, index + 1),
  )
}

export function InactiveDaysCalendar({
  inactiveDays,
  initialMonth,
  onCreateInactiveDay,
  onDeleteInactiveDay,
}: InactiveDaysCalendarProps) {
  const [month, setMonth] = useState(() => initialMonth ?? new Date())
  const [focusedDay, setFocusedDay] = useState(1)
  const [announcement, setAnnouncement] = useState(`Showing ${MONTH_FORMAT.format(initialMonth ?? new Date())}`)
  const dayRefs = useRef(new Map<number, HTMLButtonElement>())
  const inactiveByDate = useMemo(
    () => new Map(inactiveDays.map((day) => [day.fecha, day])),
    [inactiveDays],
  )
  const days = daysInMonth(month)

  function setMonthByOffset(offset: number) {
    setMonth((current) => {
      const next = new Date(current.getFullYear(), current.getMonth() + offset, 1)
      setFocusedDay(1)
      setAnnouncement(`Showing ${MONTH_FORMAT.format(next)}`)
      window.setTimeout(() => dayRefs.current.get(1)?.focus(), 0)
      return next
    })
  }

  function moveFocus(nextDay: number) {
    const bounded = Math.min(Math.max(nextDay, 1), days.length)
    setFocusedDay(bounded)
    window.setTimeout(() => dayRefs.current.get(bounded)?.focus(), 0)
  }

  function toggleDay(day: Date) {
    const isoDate = toISODate(day.getFullYear(), day.getMonth(), day.getDate())
    const existing = inactiveByDate.get(isoDate)
    if (existing) {
      onDeleteInactiveDay(existing.id, isoDate)
      setAnnouncement(`${ANNOUNCE_DAY_FORMAT.format(day)} toggled back to standard Workday`)
      return
    }
    onCreateInactiveDay(isoDate)
    setAnnouncement(`${ANNOUNCE_DAY_FORMAT.format(day)} marked as Inactive Holiday`)
  }

  function handleDayKeyDown(event: React.KeyboardEvent<HTMLButtonElement>, day: Date) {
    const dayNumber = day.getDate()
    if (event.key === 'ArrowRight') {
      event.preventDefault()
      moveFocus(dayNumber + 1)
    } else if (event.key === 'ArrowLeft') {
      event.preventDefault()
      moveFocus(dayNumber - 1)
    } else if (event.key === 'ArrowDown') {
      event.preventDefault()
      moveFocus(dayNumber + 7)
    } else if (event.key === 'ArrowUp') {
      event.preventDefault()
      moveFocus(dayNumber - 7)
    } else if (event.key === 'PageUp') {
      event.preventDefault()
      setMonthByOffset(-1)
    } else if (event.key === 'PageDown') {
      event.preventDefault()
      setMonthByOffset(1)
    } else if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault()
      toggleDay(day)
    }
  }

  return (
    <section className="rounded-xl border border-slate-200 bg-white p-5 shadow-sm" aria-label="Operational calendar">
      <div className="mb-4 flex items-center justify-between">
        <button
          type="button"
          aria-label="Previous month"
          onClick={() => setMonthByOffset(-1)}
          className="rounded-md border border-slate-300 px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50 focus:outline-none focus-visible:ring-2 focus-visible:ring-indigo-500"
        >
          Previous
        </button>
        <h2 className="text-lg font-semibold text-slate-900">{MONTH_FORMAT.format(month)}</h2>
        <button
          type="button"
          aria-label="Next month"
          onClick={() => setMonthByOffset(1)}
          className="rounded-md border border-slate-300 px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50 focus:outline-none focus-visible:ring-2 focus-visible:ring-indigo-500"
        >
          Next
        </button>
      </div>
      <div role="status" aria-live="polite" id="calendar-live-announcer" className="sr-only">
        {announcement}
      </div>
      <div className="grid grid-cols-7 gap-2 text-center text-xs font-semibold uppercase text-slate-500">
        {['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'].map((day) => (
          <span key={day}>{day}</span>
        ))}
      </div>
      <div className="mt-2 grid grid-cols-7 gap-2">
        {days.map((day) => {
          const dayNumber = day.getDate()
          const isoDate = toISODate(day.getFullYear(), day.getMonth(), dayNumber)
          const isInactive = inactiveByDate.has(isoDate)
          const isFocused = focusedDay === dayNumber
          const label = `${DAY_FORMAT.format(day)}, ${isInactive ? 'Inactive Holiday' : 'Workday'}${
            isFocused ? ', Selected' : ''
          }`
          return (
            <button
              key={isoDate}
              ref={(node) => {
                if (node) dayRefs.current.set(dayNumber, node)
                else dayRefs.current.delete(dayNumber)
              }}
              type="button"
              aria-label={label}
              tabIndex={isFocused ? 0 : -1}
              onFocus={() => setFocusedDay(dayNumber)}
              onClick={() => toggleDay(day)}
              onKeyDown={(event) => handleDayKeyDown(event, day)}
              className={`min-h-12 rounded-lg border text-sm font-semibold focus:outline-none focus-visible:ring-2 focus-visible:ring-indigo-500 ${
                isInactive
                  ? 'border-indigo-600 bg-indigo-600 text-white'
                  : 'border-slate-200 bg-white text-slate-700 hover:bg-slate-50'
              } ${isFocused ? 'ring-2 ring-indigo-500 ring-offset-2' : ''}`}
            >
              <span aria-hidden="true">{dayNumber}</span>
            </button>
          )
        })}
      </div>
    </section>
  )
}
