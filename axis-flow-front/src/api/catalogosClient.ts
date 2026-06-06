import axiosInstance from './axiosInstance'
import type {
  Country,
  State,
  City,
  Bank,
  TaxRegime,
  PaymentForm,
  PaymentCondition,
  WorkflowStatus,
  ComplaintType,
  CatalogService,
  SubscriptionPlan,
  DatePeriodicity,
  HrAbsenceType,
  JobCategory,
  JobType,
} from '@/types/catalogos'

// Geography
export const listCountries = () => axiosInstance.get<Country[]>('/v1/pais').then(r => r.data)
export const createCountry = (data: Partial<Country>) => axiosInstance.post<Country>('/v1/pais', data).then(r => r.data)
export const updateCountry = (id: number, data: Partial<Country>) => axiosInstance.put<Country>(`/v1/pais/${id}`, data).then(r => r.data)
export const deleteCountry = (id: number) => axiosInstance.delete(`/v1/pais/${id}`)

export const listStates = (countryId: number) =>
  axiosInstance.get<State[]>('/v1/estado', { params: { country_id: countryId } }).then(r => r.data)
export const createState = (data: Partial<State>) => axiosInstance.post<State>('/v1/estado', data).then(r => r.data)
export const deleteState = (id: number) => axiosInstance.delete(`/v1/estado/${id}`)

export const listCities = () => axiosInstance.get<City[]>('/v1/ciudad').then(r => r.data)
export const listCitiesByState = (estadoId: number) => axiosInstance.get<City[]>(`/v1/ciudad/byedo/${estadoId}`).then(r => r.data)
export const createCity = (data: Partial<City>) => axiosInstance.post<City>('/v1/ciudad', data).then(r => r.data)
export const deleteCity = (id: number) => axiosInstance.delete(`/v1/ciudad/${id}`)

// Financial
export const listBanks = () => axiosInstance.get<Bank[]>('/v1/bancos').then(r => r.data)
export const createBank = (data: Partial<Bank>) => axiosInstance.post<Bank>('/v1/bancos', data).then(r => r.data)
export const deleteBank = (id: number) => axiosInstance.delete(`/v1/bancos/${id}`)

export const listTaxRegimes = () => axiosInstance.get<TaxRegime[]>('/v1/regimen_fiscal').then(r => r.data)
export const listPaymentForms = () => axiosInstance.get<PaymentForm[]>('/v1/forma_pago').then(r => r.data)
export const listPaymentConditions = () => axiosInstance.get<PaymentCondition[]>('/v1/condiciones_pago').then(r => r.data)

// Operational
export const listWorkflowStatuses = () => axiosInstance.get<WorkflowStatus[]>('/v1/status').then(r => r.data)
export const createWorkflowStatus = (data: Partial<WorkflowStatus>) => axiosInstance.post<WorkflowStatus>('/v1/status', data).then(r => r.data)
export const filterStatusByRole = (rol: number) => axiosInstance.get<WorkflowStatus[]>(`/v1/status/filterStatus/${rol}`).then(r => r.data)
export const listComplaintTypes = () => axiosInstance.get<ComplaintType[]>('/v1/tipo_queja').then(r => r.data)
export const listServices = () => axiosInstance.get<CatalogService[]>('/v1/cataServicio').then(r => r.data)
export const listSubscriptionPlans = () => axiosInstance.get<SubscriptionPlan[]>('/v1/planes').then(r => r.data)
export const listDatePeriodicities = () => axiosInstance.get<DatePeriodicity[]>('/v1/periodicidadFecha').then(r => r.data)

// HR/Jobs
export const listJobCategories = () => axiosInstance.get<JobCategory[]>('/v1/catalogoCategoriaBT').then(r => r.data)
export const createJobCategory = (data: { nombre: string }) => axiosInstance.post<JobCategory>('/v1/catalogoCategoriaBT', data).then(r => r.data)
export const listJobTypes = () => axiosInstance.get<JobType[]>('/v1/catalogoTipoBT').then(r => r.data)
export const createJobType = (data: { nombre: string }) => axiosInstance.post<JobType>('/v1/catalogoTipoBT', data).then(r => r.data)
export const listHrAbsenceTypes = () => axiosInstance.get<HrAbsenceType[]>('/v1/cataTipo_inasistencia').then(r => r.data)
