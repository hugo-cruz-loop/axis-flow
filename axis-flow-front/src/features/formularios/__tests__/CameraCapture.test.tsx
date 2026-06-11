import { describe, it, expect, vi, beforeAll, afterEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { CameraCapture } from '../components/presentational/CameraCapture'

// jsdom's URL.createObjectURL is not implemented; stub it so we can assert
// the thumbnail is rendered with a blob: URL.
beforeAll(() => {
  if (!('createObjectURL' in URL)) {
    Object.defineProperty(URL, 'createObjectURL', {
      configurable: true,
      writable: true,
      value: vi.fn((blob: Blob) => `blob:mock/${(blob as unknown as { size: number }).size}`),
    })
  } else {
    ;(URL.createObjectURL as unknown as ReturnType<typeof vi.fn>) = vi.fn(
      (blob: Blob) => `blob:mock/${(blob as unknown as { size: number }).size}`,
    )
  }
  if (!('revokeObjectURL' in URL)) {
    Object.defineProperty(URL, 'revokeObjectURL', {
      configurable: true,
      writable: true,
      value: vi.fn(),
    })
  }
})

afterEach(() => {
  vi.clearAllMocks()
})

function makeFile(name: string, sizeBytes: number, type: string): File {
  // Build a File backed by a real ArrayBuffer of the requested size so
  // .size and .type are populated correctly.
  const buf = new ArrayBuffer(sizeBytes)
  return new File([buf], name, { type })
}

describe('CameraCapture', () => {
  it('renders a file input with accept=image/* and capture=environment', () => {
    render(<CameraCapture onCapture={() => {}} ariaLabel="Photo evidence" />)
    const input = document.querySelector('input[type="file"]') as HTMLInputElement
    expect(input).not.toBeNull()
    expect(input.accept).toBe('image/*')
    expect(input.getAttribute('capture')).toBe('environment')
  })

  it('has a visible label and aria-label on the input', () => {
    render(<CameraCapture onCapture={() => {}} ariaLabel="Photo evidence" />)
    // Visible button text
    expect(screen.getByText(/capture photo/i)).toBeInTheDocument()
    // The input is labeled via aria-label
    const input = document.querySelector('input[type="file"]') as HTMLInputElement
    expect(input.getAttribute('aria-label')).toBe('Photo evidence')
    // The label is linked to the input via htmlFor
    const label = document.querySelector('label[for]') as HTMLLabelElement
    expect(label.getAttribute('for')).toBe(input.id)
  })

  it('calls onCapture with the selected File when a valid image is chosen', async () => {
    const onCapture = vi.fn()
    render(<CameraCapture onCapture={onCapture} ariaLabel="Photo evidence" />)
    const input = document.querySelector('input[type="file"]') as HTMLInputElement
    const file = makeFile('evidence.jpg', 1024, 'image/jpeg')
    // userEvent.upload is the canonical way to set files on an input.
    await userEvent.upload(input, file)
    expect(onCapture).toHaveBeenCalledTimes(1)
    expect(onCapture.mock.calls[0]?.[0]).toBe(file)
  })

  it('rejects files larger than maxSizeBytes and does NOT call onCapture; shows an error', async () => {
    const onCapture = vi.fn()
    render(
      <CameraCapture
        onCapture={onCapture}
        ariaLabel="Photo evidence"
        maxSizeBytes={2048} // 2 KB
      />,
    )
    const input = document.querySelector('input[type="file"]') as HTMLInputElement
    const big = makeFile('big.jpg', 5 * 1024, 'image/jpeg')
    fireEvent.change(input, { target: { files: [big] } })
    expect(onCapture).not.toHaveBeenCalled()
    expect(screen.getByRole('alert')).toHaveTextContent(/too large|exceeds|5\.\d|kb/i)
  })

  it('rejects files with a non-image MIME type and shows an error', () => {
    const onCapture = vi.fn()
    render(<CameraCapture onCapture={onCapture} ariaLabel="Photo evidence" />)
    const input = document.querySelector('input[type="file"]') as HTMLInputElement
    const txt = makeFile('note.txt', 100, 'text/plain')
    fireEvent.change(input, { target: { files: [txt] } })
    expect(onCapture).not.toHaveBeenCalled()
    expect(screen.getByRole('alert')).toHaveTextContent(/image/i)
  })

  it('renders a thumbnail (img with src starting with blob:) after a successful capture', async () => {
    const onCapture = vi.fn()
    render(<CameraCapture onCapture={onCapture} ariaLabel="Photo evidence" />)
    const input = document.querySelector('input[type="file"]') as HTMLInputElement
    const file = makeFile('evidence.jpg', 1024, 'image/jpeg')
    await userEvent.upload(input, file)
    const imgs = screen.getAllByRole('img')
    const thumbnail = imgs.find((img) => (img as HTMLImageElement).src.startsWith('blob:'))
    expect(thumbnail).toBeDefined()
  })

  it('disabled=true disables the input', () => {
    render(<CameraCapture onCapture={() => {}} ariaLabel="Photo evidence" disabled />)
    const input = document.querySelector('input[type="file"]') as HTMLInputElement
    expect(input).toBeDisabled()
  })

  it('changing the same input twice (re-capture) replaces the thumbnail', async () => {
    const onCapture = vi.fn()
    render(<CameraCapture onCapture={onCapture} ariaLabel="Photo evidence" />)
    const input = document.querySelector('input[type="file"]') as HTMLInputElement

    const first = makeFile('first.jpg', 1024, 'image/jpeg')
    fireEvent.change(input, { target: { files: [first] } })
    expect(onCapture).toHaveBeenLastCalledWith(first)

    const second = makeFile('second.jpg', 2048, 'image/jpeg')
    fireEvent.change(input, { target: { files: [second] } })
    expect(onCapture).toHaveBeenLastCalledWith(second)
    expect(onCapture).toHaveBeenCalledTimes(2)
  })
})
