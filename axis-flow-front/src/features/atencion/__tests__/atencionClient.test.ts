import { describe, it, expect } from 'vitest'
import { readFileSync } from 'fs'
import { resolve } from 'path'

describe('atencionClient security', () => {
  it('does not reference localStorage in atencionClient.ts', () => {
    const source = readFileSync(
      resolve(__dirname, '../../../api/atencionClient.ts'),
      'utf-8',
    )
    expect(source).not.toMatch(/localStorage/)
  })

  it('uses Zustand getState() for token access', () => {
    const source = readFileSync(
      resolve(__dirname, '../../../api/atencionClient.ts'),
      'utf-8',
    )
    expect(source).toMatch(/useAuthStore\.getState\(\)/)
  })
})
