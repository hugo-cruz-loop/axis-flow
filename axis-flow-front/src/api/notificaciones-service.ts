import axios from 'axios'
import { useAuthStore } from '@/store/authStore'
import type { TokenRegistrationInput } from '@/schemas/notificaciones-schemas'

// Token is read from Zustand in-memory store (XSS-safe, never persisted to disk).
const notificacionesClient = axios.create({
  baseURL:
    (import.meta.env.VITE_API_BASE_URL as string | undefined)
      ? `${import.meta.env.VITE_API_BASE_URL as string}/api/v1/notificaciones`
      : '/api/v1/notificaciones',
  timeout: 10000,
})

notificacionesClient.interceptors.request.use((config) => {
  const token = useAuthStore.getState().accessToken
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

notificacionesClient.interceptors.response.use(
  (res) => res,
  (err: unknown) => {
    if (axios.isAxiosError(err) && err.response?.status === 401) {
      useAuthStore.getState().clearAuth()
    }
    return Promise.reject(err)
  },
)

export interface PushNotification {
  id: string
  user_id: string
  noti_id: string
  device_type: string
  created_at: string
}

export interface PagedResponse<T> {
  data: T[]
  total: number
  page: number
  page_size: number
}

export const notificacionesService = {
  triggerRecoveryEmail(email: string): Promise<void> {
    return notificacionesClient
      .post<void>('/recuperar-password', { email })
      .then(() => undefined)
  },

  triggerNewUserEmail(userId: string, email: string, name: string): Promise<void> {
    return notificacionesClient
      .post<void>('/bienvenida', { user_id: userId, email, name })
      .then(() => undefined)
  },

  getPushHistory(
    userId: string,
    page = 1,
    pageSize = 20,
  ): Promise<PagedResponse<PushNotification>> {
    return notificacionesClient
      .get<{ success: boolean; data: PagedResponse<PushNotification> }>(
        `/usuario/${userId}/notificaciones`,
        { params: { page, page_size: pageSize } },
      )
      .then((res) => res.data.data)
  },

  getUnreadCount(userId: string): Promise<number> {
    return notificacionesClient
      .get<{ success: boolean; data: { count: number } }>(`/usuario/${userId}/no-leidas`)
      .then((res) => res.data.data.count)
  },

  markAsRead(notificationId: string): Promise<void> {
    return notificacionesClient
      .patch<void>(`/${notificationId}/estatus`, { estatus: 'leida' })
      .then(() => undefined)
  },

  registerToken(data: TokenRegistrationInput): Promise<void> {
    return notificacionesClient
      .post<void>('/token', data)
      .then(() => undefined)
  },
}
