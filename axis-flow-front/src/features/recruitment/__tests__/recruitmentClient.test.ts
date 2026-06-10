import { describe, it, expect } from 'vitest'
import { readFileSync } from 'fs'
import { resolve } from 'path'

// Task 6.11 — assert zero localStorage references in recruitmentClient.ts
describe('recruitmentClient security', () => {
  it('does not reference localStorage in recruitmentClient.ts', () => {
    const source = readFileSync(
      resolve(__dirname, '../../../api/recruitmentClient.ts'),
      'utf-8',
    )
    expect(source).not.toMatch(/localStorage/)
  })

  it('uses Zustand getState() for token access', () => {
    const source = readFileSync(
      resolve(__dirname, '../../../api/recruitmentClient.ts'),
      'utf-8',
    )
    expect(source).toMatch(/useAuthStore\.getState\(\)/)
  })
})
