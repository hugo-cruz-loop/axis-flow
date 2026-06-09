import axios from 'axios'
import { useAuthStore } from '@/store/authStore'

export const recruitmentClient = axios.create({
  baseURL: (import.meta.env.VITE_API_BASE_URL as string | undefined) ?? '/api/v1/bolsa-trabajo',
  timeout: 10000,
})

recruitmentClient.interceptors.request.use((config) => {
  // Token from Zustand in-memory store (XSS-safe, never persisted to disk)
  const token = useAuthStore.getState().accessToken
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})
