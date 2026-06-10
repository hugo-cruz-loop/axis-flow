import axios from 'axios'
import { useAuthStore } from '@/store/authStore'

// Token is read from Zustand in-memory store (XSS-safe, never persisted to disk).
export const atencionClient = axios.create({
  baseURL:
    (import.meta.env.VITE_API_BASE_URL as string | undefined)
      ? `${import.meta.env.VITE_API_BASE_URL as string}/api/v1/atencion`
      : '/api/v1/atencion',
  timeout: 10000,
})

atencionClient.interceptors.request.use((config) => {
  const token = useAuthStore.getState().accessToken
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})
