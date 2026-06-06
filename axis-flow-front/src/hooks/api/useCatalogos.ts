import { useQuery } from '@tanstack/react-query'
import {
  listCountries,
  listStates,
  listCitiesByState,
  listBanks,
  listTaxRegimes,
  listPaymentForms,
  listPaymentConditions,
  listWorkflowStatuses,
  listComplaintTypes,
  listServices,
  listSubscriptionPlans,
  listDatePeriodicities,
  listJobCategories,
  listJobTypes,
  listHrAbsenceTypes,
} from '@/api/catalogosClient'

const CATALOG_QUERY_OPTIONS = {
  staleTime: 10 * 60 * 1000,
  gcTime: 30 * 60 * 1000,
  refetchOnWindowFocus: false,
}

export const useCountries = () =>
  useQuery({ queryKey: ['catalogos', 'paises'], queryFn: listCountries, ...CATALOG_QUERY_OPTIONS })

export const useStates = (countryId?: number) =>
  useQuery({
    queryKey: ['catalogos', 'estados', 'pais', countryId],
    queryFn: () => listStates(countryId!),
    enabled: !!countryId,
    ...CATALOG_QUERY_OPTIONS,
  })

export const useCitiesByState = (estadoId?: number) =>
  useQuery({
    queryKey: ['catalogos', 'ciudades', 'estado', estadoId],
    queryFn: () => listCitiesByState(estadoId!),
    enabled: !!estadoId,
    ...CATALOG_QUERY_OPTIONS,
  })

export const useBanks = () =>
  useQuery({ queryKey: ['catalogos', 'bancos'], queryFn: listBanks, ...CATALOG_QUERY_OPTIONS })

export const useTaxRegimes = () =>
  useQuery({ queryKey: ['catalogos', 'regimenes'], queryFn: listTaxRegimes, ...CATALOG_QUERY_OPTIONS })

export const usePaymentForms = () =>
  useQuery({ queryKey: ['catalogos', 'formasPago'], queryFn: listPaymentForms, ...CATALOG_QUERY_OPTIONS })

export const usePaymentConditions = () =>
  useQuery({ queryKey: ['catalogos', 'condicionesPago'], queryFn: listPaymentConditions, ...CATALOG_QUERY_OPTIONS })

export const useWorkflowStatuses = () =>
  useQuery({ queryKey: ['catalogos', 'statuses'], queryFn: listWorkflowStatuses, ...CATALOG_QUERY_OPTIONS })

export const useComplaintTypes = () =>
  useQuery({ queryKey: ['catalogos', 'tiposQueja'], queryFn: listComplaintTypes, ...CATALOG_QUERY_OPTIONS })

export const useCatalogServices = () =>
  useQuery({ queryKey: ['catalogos', 'servicios'], queryFn: listServices, ...CATALOG_QUERY_OPTIONS })

export const useSubscriptionPlans = () =>
  useQuery({ queryKey: ['catalogos', 'planes'], queryFn: listSubscriptionPlans, ...CATALOG_QUERY_OPTIONS })

export const useDatePeriodicities = () =>
  useQuery({ queryKey: ['catalogos', 'periodicidades'], queryFn: listDatePeriodicities, ...CATALOG_QUERY_OPTIONS })

export const useJobCategories = () =>
  useQuery({ queryKey: ['catalogos', 'categoriasBT'], queryFn: listJobCategories, ...CATALOG_QUERY_OPTIONS })

export const useJobTypes = () =>
  useQuery({ queryKey: ['catalogos', 'tiposBT'], queryFn: listJobTypes, ...CATALOG_QUERY_OPTIONS })

export const useHrAbsenceTypes = () =>
  useQuery({ queryKey: ['catalogos', 'tiposInasistencia'], queryFn: listHrAbsenceTypes, ...CATALOG_QUERY_OPTIONS })
