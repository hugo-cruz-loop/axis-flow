import { useMemo } from 'react'
import { useAuthStore } from '@/store/authStore'

export interface JWTClaims {
  userId: string
  /**
   * empresa_id (the tenant UUID). The formularios backend uses this to
   * scope every read/write; it is the openapi `tenant` for all formularios
   * endpoints.
   */
  tenantId: string
  /**
   * The current empleado's id, or 0 if the user has no linked empleado
   * (e.g. an admin/operations role). The formularios backend reads this
   * from the JWT context and ignores any body-supplied value.
   */
  empleadoId: number
  email: string
  role: string
  exp: number
  iat: number
}

/**
 * React hook that exposes the current user's JWT claims.
 *
 * Returns `null` when:
 *   - the user is not authenticated (no `accessToken` in the Zustand store)
 *   - the token is malformed (not a 3-segment JWT)
 *   - the payload is missing the required `uid` or `tid` claim
 *
 * Callers should treat `null` as "session invalid" and render an error
 * state — never as "anonymous, proceed".
 */
export function useJWTClaims(): JWTClaims | null {
  const accessToken = useAuthStore((s) => s.accessToken)

  return useMemo(() => {
    if (!accessToken) return null
    return parseJWTClaims(accessToken)
  }, [accessToken])
}

/**
 * Parse a JWT-shaped string and return its claims as a typed object.
 *
 * Exported separately so non-React callsites (axios interceptors, error
 * handlers) can re-use the same parsing logic, and so unit tests can drive
 * the parser directly without setting up a hook + QueryClient wrapper.
 *
 * The function is intentionally total: ANY failure (bad shape, bad
 * base64, bad JSON, missing required claim) yields `null` rather than
 * throwing. Callers must check for `null` before reading the claims.
 */
export function parseJWTClaims(token: string): JWTClaims | null {
  try {
    const parts = token.split('.')
    if (parts.length !== 3) return null
    const payload = JSON.parse(atob(padBase64(parts[1] ?? '')))

    // Validate the strict-claim set. Missing uid or tid → null: the
    // formularios backend needs both to scope queries, so an empty tenant
    // would silently leak across tenants.
    if (typeof payload.uid !== 'string' || typeof payload.tid !== 'string') {
      return null
    }

    return {
      userId: payload.uid,
      tenantId: payload.tid,
      // PR-5 backward-compat: tokens minted before PR-5 do not carry
      // empleado_id. Default to 0 ("no linked empleado").
      empleadoId: typeof payload.empleado_id === 'number' ? payload.empleado_id : 0,
      email: typeof payload.email === 'string' ? payload.email : '',
      role: typeof payload.role === 'string' ? payload.role : '',
      exp: typeof payload.exp === 'number' ? payload.exp : 0,
      iat: typeof payload.iat === 'number' ? payload.iat : 0,
    }
  } catch {
    return null
  }
}

// padBase64 pads a base64url string to a length divisible by 4 so atob
// accepts it, and translates the URL-safe alphabet to the standard
// base64 alphabet. Internal to the parser — not exported.
function padBase64(s: string): string {
  const padLen = (4 - (s.length % 4)) % 4
  return (s + '='.repeat(padLen)).replace(/-/g, '+').replace(/_/g, '/')
}
