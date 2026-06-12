import axios from 'axios'
import { useAuthStore } from '@/store/authStore'
import type { ReportRunPayload, ReportRunResponse } from '@/schemas/report-schemas'
import { reportRunResponseSchema } from '@/schemas/report-schemas'

// Token is read from Zustand in-memory store (XSS-safe, never persisted to disk).
const reportClient = axios.create({
  baseURL: (import.meta.env.VITE_API_BASE_URL as string | undefined)
    ? `${import.meta.env.VITE_API_BASE_URL as string}/api/v1/report`
    : '/api/v1/report',
  timeout: 30000,
})

reportClient.interceptors.request.use((config) => {
  const token = useAuthStore.getState().accessToken
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

async function runReport(
  type: string,
  payload: ReportRunPayload,
): Promise<ReportRunResponse['data']> {
  const response = await reportClient.post<unknown>(`/run/${type}`, payload)
  const parsed = reportRunResponseSchema.parse(response.data)
  return parsed.data
}

function getDownloadUrl(downloadUrl: string): string {
  if (downloadUrl.startsWith('http://') || downloadUrl.startsWith('https://')) {
    return downloadUrl
  }
  const base =
    (import.meta.env.VITE_API_BASE_URL as string | undefined) ?? window.location.origin
  return `${base}${downloadUrl.startsWith('/') ? '' : '/'}${downloadUrl}`
}

export const ReportService = { runReport, getDownloadUrl }
