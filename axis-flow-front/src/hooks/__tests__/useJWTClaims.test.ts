import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook } from '@testing-library/react'
import { parseJWTClaims, useJWTClaims } from '../useJWTClaims'
import { useAuthStore } from '@/store/authStore'

// Build a base64url-encoded JWT-shaped token from a payload object. We do NOT
// sign — the hook only parses the payload, so the signature segment is a
// placeholder. The hook does not verify the signature; that's the backend's
// job. This keeps the test self-contained and fast.
function makeToken(payload: object): string {
  const header = btoa(JSON.stringify({ alg: 'HS256', typ: 'JWT' }))
    .replace(/=/g, '')
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
  const body = btoa(JSON.stringify(payload))
    .replace(/=/g, '')
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
  return `${header}.${body}.signature`
}

const FULL_PAYLOAD = {
  uid: 'user-uuid-1',
  tid: 'tenant-uuid-1',
  empleado_id: 42,
  email: 'alice@example.com',
  role: 'EMPLEADO',
  exp: 1_900_000_000,
  iat: 1_800_000_000,
}

beforeEach(() => {
  // Reset the auth store between tests so a token seeded in one test does
  // not leak into the next.
  useAuthStore.setState({
    accessToken: null,
    user: null,
    isAuthenticated: false,
  })
  vi.clearAllMocks()
})

describe('parseJWTClaims', () => {
  it('returns a full JWTClaims object for a token with every claim', () => {
    const result = parseJWTClaims(makeToken(FULL_PAYLOAD))
    expect(result).toEqual({
      userId: 'user-uuid-1',
      tenantId: 'tenant-uuid-1',
      empleadoId: 42,
      email: 'alice@example.com',
      role: 'EMPLEADO',
      exp: 1_900_000_000,
      iat: 1_800_000_000,
    })
  })

  it('returns empleadoId=0 when the token omits the empleado_id claim (PR-5 backward-compat)', () => {
    // Backward-compat invariant: tokens minted before PR-5 do NOT carry the
    // empleado_id claim. The frontend must treat these as "no linked
    // empleado" rather than throwing or producing NaN.
    const { empleado_id: _omit, ...rest } = FULL_PAYLOAD
    void _omit
    const result = parseJWTClaims(makeToken(rest))
    expect(result).not.toBeNull()
    expect(result?.empleadoId).toBe(0)
    expect(result?.userId).toBe('user-uuid-1')
    expect(result?.tenantId).toBe('tenant-uuid-1')
  })

  it('returns empty strings for email and role when those claims are missing', () => {
    // Defensive: email/role are not in the strict-claim set. If they're
    // absent the hook must not throw — it should expose '' so callers can
    // detect "unknown" without a try/catch.
    const { email: _e, role: _r, ...rest } = FULL_PAYLOAD
    void _e
    void _r
    const result = parseJWTClaims(makeToken(rest))
    expect(result).not.toBeNull()
    expect(result?.email).toBe('')
    expect(result?.role).toBe('')
  })

  it('returns null when the token is not a 3-segment JWT shape', () => {
    expect(parseJWTClaims('not-a-jwt')).toBeNull()
    expect(parseJWTClaims('a.b')).toBeNull() // missing signature
    expect(parseJWTClaims('a.b.c.d')).toBeNull() // too many segments
    expect(parseJWTClaims('')).toBeNull()
  })

  it('returns null when the payload segment is not valid base64', () => {
    // '!!!' is not valid base64; the atob step will throw.
    const result = parseJWTClaims('header.!!!.sig')
    expect(result).toBeNull()
  })

  it('returns null when the payload segment is not valid JSON', () => {
    // "{not json" is valid base64 but JSON.parse will throw.
    const notJson = btoa('{not json').replace(/=/g, '')
    const result = parseJWTClaims(`header.${notJson}.sig`)
    expect(result).toBeNull()
  })

  it('returns null when the required uid claim is missing', () => {
    const { uid: _u, ...rest } = FULL_PAYLOAD
    void _u
    expect(parseJWTClaims(makeToken(rest))).toBeNull()
  })

  it('returns null when the required tid claim is missing', () => {
    const { tid: _t, ...rest } = FULL_PAYLOAD
    void _t
    expect(parseJWTClaims(makeToken(rest))).toBeNull()
  })
})

describe('useJWTClaims', () => {
  it('returns null when the auth store has no accessToken', () => {
    useAuthStore.setState({ accessToken: null, user: null, isAuthenticated: false })
    const { result } = renderHook(() => useJWTClaims())
    expect(result.current).toBeNull()
  })

  it('returns parsed claims when the auth store has a valid accessToken', () => {
    useAuthStore.setState({
      accessToken: makeToken(FULL_PAYLOAD),
      user: null,
      isAuthenticated: true,
    })
    const { result } = renderHook(() => useJWTClaims())
    expect(result.current).toEqual({
      userId: 'user-uuid-1',
      tenantId: 'tenant-uuid-1',
      empleadoId: 42,
      email: 'alice@example.com',
      role: 'EMPLEADO',
      exp: 1_900_000_000,
      iat: 1_800_000_000,
    })
  })
})
