import axiosInstance from './axiosInstance'
import type { LoginResponse, User } from '@/types/users'

export async function login(
  email: string,
  password: string,
): Promise<LoginResponse> {
  const response = await axiosInstance.post<LoginResponse>('/auth/login/', {
    email,
    password,
  })
  return response.data
}

export async function refreshToken(
  token: string,
): Promise<{ access_token: string }> {
  const response = await axiosInstance.post<{ access_token: string }>(
    '/auth/token/refresh/',
    { refresh_token: token },
  )
  return response.data
}

export async function me(): Promise<User> {
  const response = await axiosInstance.get<User>('/auth/me/')
  return response.data
}

export async function meWithToken(accessToken: string): Promise<User> {
  const response = await axiosInstance.get<User>('/auth/me/', {
    headers: { Authorization: `Bearer ${accessToken}` },
  })
  return response.data
}
