import axiosInstance from './axiosInstance'
import type {
  Empresa,
  DatosFiscales,
  Apoderado,
  Servicio,
  OnboardingRequest,
  OnboardingResult,
} from '@/types/empresas'

// Public — use plain axios (no JWT needed)
import axios from 'axios'
const publicApi = axios.create({ baseURL: '/api/v1' })

export const registerEmpresa = (
  req: OnboardingRequest,
): Promise<{ data: OnboardingResult }> =>
  publicApi.post('/empresa/alta', req).then((r) => r.data)

export const createCheckoutSession = (data: {
  plan_id: number
  clave_pago: string
  success_url: string
  cancel_url: string
}): Promise<{ data: { checkout_url: string } }> =>
  publicApi.post('/empresa/checkout', data).then((r) => r.data)

// Protected — use axiosInstance with JWT
export const getEmpresa = (id: number): Promise<Empresa> =>
  axiosInstance.get(`/v1/empresa/${id}`).then((r) => r.data)

export const updateEmpresa = (id: number, data: Partial<Empresa>): Promise<Empresa> =>
  axiosInstance.put(`/v1/empresa/${id}`, data).then((r) => r.data)

export const getFiscal = (id: number): Promise<DatosFiscales> =>
  axiosInstance.get(`/v1/empresa/${id}/fiscal`).then((r) => r.data)

export const createFiscal = (
  id: number,
  data: Partial<DatosFiscales>,
): Promise<DatosFiscales> =>
  axiosInstance.post(`/v1/empresa/${id}/fiscal`, data).then((r) => r.data)

export const updateFiscal = (
  id: number,
  data: Partial<DatosFiscales>,
): Promise<DatosFiscales> =>
  axiosInstance.put(`/v1/empresa/${id}/fiscal`, data).then((r) => r.data)

export const listApoderados = (id: number): Promise<Apoderado[]> =>
  axiosInstance.get(`/v1/empresa/${id}/apoderados`).then((r) => r.data)

export const createApoderado = (
  id: number,
  data: Omit<Apoderado, 'id' | 'empresa_id'>,
): Promise<Apoderado> =>
  axiosInstance.post(`/v1/empresa/${id}/apoderados`, data).then((r) => r.data)

export const updateApoderado = (
  id: number,
  apoderadoId: number,
  data: Partial<Apoderado>,
): Promise<Apoderado> =>
  axiosInstance
    .put(`/v1/empresa/${id}/apoderados/${apoderadoId}`, data)
    .then((r) => r.data)

export const deleteApoderado = (id: number, apoderadoId: number): Promise<void> =>
  axiosInstance.delete(`/v1/empresa/${id}/apoderados/${apoderadoId}`)

export const listServicios = (id: number): Promise<Servicio[]> =>
  axiosInstance.get(`/v1/empresa/${id}/servicios`).then((r) => r.data)

export const createServicio = (
  id: number,
  data: Omit<Servicio, 'id' | 'empresa_id'>,
): Promise<Servicio> =>
  axiosInstance.post(`/v1/empresa/${id}/servicios`, data).then((r) => r.data)

export const updateServicio = (
  id: number,
  servicioId: number,
  data: Partial<Servicio>,
): Promise<Servicio> =>
  axiosInstance
    .put(`/v1/empresa/${id}/servicios/${servicioId}`, data)
    .then((r) => r.data)

export const deleteServicio = (id: number, servicioId: number): Promise<void> =>
  axiosInstance.delete(`/v1/empresa/${id}/servicios/${servicioId}`)
