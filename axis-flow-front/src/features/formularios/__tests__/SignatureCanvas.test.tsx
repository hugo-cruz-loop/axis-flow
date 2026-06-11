import { describe, it, expect, vi, beforeAll } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { SignatureCanvas } from '../components/presentational/SignatureCanvas'

// jsdom does not implement Canvas 2D. Install a recording stub on the prototype
// BEFORE importing the component so the stub is in place when the component
// first calls canvas.getContext('2d') in its useEffect.
const calls: { method: string; args: unknown[] }[] = []
let pressed = false

beforeAll(() => {
  HTMLCanvasElement.prototype.getContext = vi.fn(function getContextStub() {
    return {
      scale: (...args: unknown[]) => calls.push({ method: 'scale', args }),
      beginPath: (...args: unknown[]) => calls.push({ method: 'beginPath', args }),
      moveTo: (...args: unknown[]) => calls.push({ method: 'moveTo', args }),
      lineTo: (...args: unknown[]) => calls.push({ method: 'lineTo', args }),
      stroke: (...args: unknown[]) => calls.push({ method: 'stroke', args }),
      clearRect: (...args: unknown[]) => calls.push({ method: 'clearRect', args }),
      setTransform: (...args: unknown[]) => calls.push({ method: 'setTransform', args }),
      save: (...args: unknown[]) => calls.push({ method: 'save', args }),
      restore: (...args: unknown[]) => calls.push({ method: 'restore', args }),
      set lineCap(_: string) {},
      set lineJoin(_: string) {},
      set strokeStyle(_: string) {},
      set lineWidth(_: number) {},
    }
  }) as unknown as typeof HTMLCanvasElement.prototype.getContext

  // jsdom's canvas toDataURL returns '' by default — make it return a
  // recognizable marker so we can assert the component actually serializes
  // the canvas when pointerup fires.
  HTMLCanvasElement.prototype.toDataURL = vi.fn(function toDataURLStub() {
    pressed = true
    return 'data:image/png;base64,FAKE_SIGNATURE'
  })
})

describe('SignatureCanvas', () => {
  it('renders a canvas with role=img and the given aria-label', () => {
    render(
      <SignatureCanvas
        width={300}
        height={120}
        ariaLabel="Customer signature"
        onChange={() => {}}
      />,
    )
    const canvas = screen.getByRole('img', { name: /customer signature/i })
    expect(canvas).toBeInTheDocument()
    expect(canvas.tagName).toBe('CANVAS')
  })

  it('pointerdown → pointermove → pointerup triggers onChange with a non-empty dataUrl', () => {
    const onChange = vi.fn()
    render(
      <SignatureCanvas width={300} height={120} ariaLabel="x" onChange={onChange} />,
    )
    const canvas = screen.getByRole('img', { name: /x/ })

    // Simulate a single stroke.
    fireEvent.pointerDown(canvas, { clientX: 10, clientY: 10 })
    fireEvent.pointerMove(canvas, { clientX: 30, clientY: 30 })
    fireEvent.pointerUp(canvas, { clientX: 30, clientY: 30 })

    expect(onChange).toHaveBeenCalledTimes(1)
    const arg = onChange.mock.calls[0]?.[0] as string
    expect(arg).toMatch(/^data:image\/png;base64,/)
    expect(arg.length).toBeGreaterThan(20)

    // The canvas drawing API was actually invoked: at least one moveTo and one
    // lineTo, plus a stroke.
    const methodCounts = calls.reduce<Record<string, number>>((acc, c) => {
      acc[c.method] = (acc[c.method] ?? 0) + 1
      return acc
    }, {})
    expect(methodCounts.moveTo).toBeGreaterThanOrEqual(1)
    expect(methodCounts.lineTo).toBeGreaterThanOrEqual(1)
    expect(methodCounts.stroke).toBeGreaterThanOrEqual(1)
  })

  it('clear button resets canvas and emits onChange(null)', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    render(
      <SignatureCanvas width={300} height={120} ariaLabel="x" onChange={onChange} />,
    )
    const canvas = screen.getByRole('img', { name: /x/ })
    fireEvent.pointerDown(canvas, { clientX: 5, clientY: 5 })
    fireEvent.pointerUp(canvas, { clientX: 5, clientY: 5 })
    expect(onChange).toHaveBeenLastCalledWith(expect.any(String))
    onChange.mockClear()

    const clearBtn = screen.getByRole('button', { name: /clear/i })
    await user.click(clearBtn)

    expect(onChange).toHaveBeenCalledWith(null)
    const clearRectCount = calls.filter((c) => c.method === 'clearRect').length
    expect(clearRectCount).toBeGreaterThanOrEqual(1)
  })

  it('disabled=true blocks pointer events (no drawing)', () => {
    const onChange = vi.fn()
    const beforeCount = calls.length
    render(
      <SignatureCanvas width={300} height={120} ariaLabel="x" onChange={onChange} disabled />,
    )
    const canvas = screen.getByRole('img', { name: /x/ })
    fireEvent.pointerDown(canvas, { clientX: 10, clientY: 10 })
    fireEvent.pointerMove(canvas, { clientX: 30, clientY: 30 })
    fireEvent.pointerUp(canvas, { clientX: 30, clientY: 30 })

    // No drawing commands should have been recorded between the mount and now.
    const newCalls = calls.slice(beforeCount)
    const drawingCalls = newCalls.filter(
      (c) => c.method === 'moveTo' || c.method === 'lineTo' || c.method === 'stroke',
    )
    expect(drawingCalls).toHaveLength(0)
    expect(onChange).not.toHaveBeenCalled()
  })

  it('Keyboard fallback: pressing Enter on the focused canvas emits a signature dataUrl', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    render(
      <SignatureCanvas width={300} height={120} ariaLabel="x" onChange={onChange} />,
    )
    const canvas = screen.getByRole('img', { name: /x/ })
    canvas.focus()
    await user.keyboard('{Enter}')

    expect(onChange).toHaveBeenCalledTimes(1)
    const arg = onChange.mock.calls[0]?.[0] as string
    expect(arg).toMatch(/^data:image\/png;base64,/)
  })
})
