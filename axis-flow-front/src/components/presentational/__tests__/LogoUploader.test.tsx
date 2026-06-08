import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { LogoUploader } from '../LogoUploader'

function makeFile(name: string, type: string, sizeBytes: number): File {
  const blob = new Blob([new ArrayBuffer(sizeBytes)], { type })
  return new File([blob], name, { type })
}

function uploadFile(file: File) {
  const input = document.querySelector('input[type="file"]') as HTMLInputElement
  Object.defineProperty(input, 'files', {
    value: [file],
    writable: false,
    configurable: true,
  })
  fireEvent.change(input)
}

describe('LogoUploader', () => {
  it('rejects file larger than 2MB', () => {
    const onFileSelect = vi.fn()
    render(<LogoUploader onFileSelect={onFileSelect} />)

    const bigFile = makeFile('logo.png', 'image/png', 3 * 1024 * 1024)
    uploadFile(bigFile)

    expect(screen.getByText(/Maximum allowed size is 2 MB/i)).toBeInTheDocument()
    expect(onFileSelect).not.toHaveBeenCalled()
  })

  it('rejects non-image file', () => {
    const onFileSelect = vi.fn()
    render(<LogoUploader onFileSelect={onFileSelect} />)

    const badFile = makeFile('document.pdf', 'application/pdf', 1024)
    uploadFile(badFile)

    expect(screen.getByText(/Invalid file type/i)).toBeInTheDocument()
    expect(onFileSelect).not.toHaveBeenCalled()
  })

  it('accepts valid PNG file', () => {
    const onFileSelect = vi.fn()
    render(<LogoUploader onFileSelect={onFileSelect} />)

    const validFile = makeFile('logo.png', 'image/png', 100 * 1024)
    uploadFile(validFile)

    expect(onFileSelect).toHaveBeenCalledWith(validFile)
    expect(screen.queryByText(/Invalid file type/i)).not.toBeInTheDocument()
    expect(screen.queryByText(/Maximum allowed size/i)).not.toBeInTheDocument()
  })
})
