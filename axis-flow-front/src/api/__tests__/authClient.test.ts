import { describe, it, expect, vi, beforeEach } from 'vitest'
import axios from 'axios'

vi.mock('@/store/authStore', () => ({
  useAuthStore: { getState: () => ({ accessToken: 'test-token' }) },
}))

vi.mock('axios', async () => {
  const actual = await vi.importActual<typeof import('axios')>('axios')
  return {
    ...actual,
    default: {
      create: vi.fn(() => mockAxiosInstance),
    },
  }
})

const mockPost = vi.fn()
const mockGet = vi.fn()
const mockAxiosInstance = {
  post: mockPost,
  get: mockGet,
  interceptors: {
    request: { use: vi.fn() },
    response: { use: vi.fn() },
  },
  defaults: { headers: { common: {} } },
}

// Re-import after mocking
let login: typeof import('../authClient').login
let me: typeof import('../authClient').me
let meWithToken: typeof import('../authClient').meWithToken

beforeEach(async () => {
  vi.clearAllMocks()
  vi.resetModules()
  const mod = await import('../authClient')
  login = mod.login
  me = mod.me
  meWithToken = mod.meWithToken
})

describe('authClient.login', () => {
  it('returns LoginResponse on 200', async () => {
    const mockResponse = {
      data: {
        access_token: 'token-abc',
        refresh_token: 'refresh-xyz',
        user_id: '123',
        role: 'CLIENTE',
      },
    }
    mockPost.mockResolvedValueOnce(mockResponse)

    const result = await login('user@example.com', 'password123')

    expect(result.access_token).toBe('token-abc')
    expect(result.user_id).toBe('123')
  })

  it('throws on 401', async () => {
    const error = Object.assign(new Error('Unauthorized'), {
      response: { status: 401 },
    })
    mockPost.mockRejectedValueOnce(error)

    await expect(login('user@example.com', 'wrongpassword')).rejects.toThrow(
      'Unauthorized',
    )
  })
})

describe('authClient.me', () => {
  it('uses interceptor-managed Authorization header when no token is passed', async () => {
    const mockUser = {
      id: '1',
      email: 'user@example.com',
      first_name: 'John',
      last_name: 'Doe',
      role: 'CLIENTE',
      permissions: ['roles:update'],
      status: 'ACTIVE',
    }
    mockGet.mockResolvedValueOnce({ data: mockUser })

    const result = await me()

    expect(mockGet).toHaveBeenCalledWith('/auth/me/')
    expect(result.email).toBe('user@example.com')
  })

  it('sends explicit Authorization header when token is passed', async () => {
    const mockUser = {
      id: '1',
      email: 'user@example.com',
      first_name: 'John',
      last_name: 'Doe',
      role: 'CLIENTE',
      permissions: ['roles:update'],
      status: 'ACTIVE',
    }
    mockGet.mockResolvedValueOnce({ data: mockUser })

    const result = await meWithToken('login-token')

    expect(mockGet).toHaveBeenCalledWith('/auth/me/', {
      headers: { Authorization: 'Bearer login-token' },
    })
    expect(result.email).toBe('user@example.com')
  })

})

// Suppress unused import warning
void axios
