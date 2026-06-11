import { useQuery } from '@tanstack/react-query'
import { DashboardService } from '../api/dashboard-service'

const DASHBOARD_STALE_TIME = 1000 * 60 * 5 // 5 minutes
const DASHBOARD_GC_TIME = 1000 * 60 * 15    // 15 minutes

const COMMON_QUERY_OPTIONS = {
  staleTime: DASHBOARD_STALE_TIME,
  gcTime: DASHBOARD_GC_TIME,
  retry: 2,
  refetchOnWindowFocus: false,
}

// --- ADMIN EMPRESA HOOKS ---

export const useEvaluacionesClientes = (idEmp: string, enabled = true) => {
  return useQuery({
    queryKey: ['dashboard', 'admin', 'evaluaciones', idEmp],
    queryFn: () => DashboardService.getEvaluacionesClientes(idEmp),
    enabled: !!idEmp && enabled,
    ...COMMON_QUERY_OPTIONS,
  })
}

export const useEmpleadosCount = (idEmp: string) => {
  return useQuery({
    queryKey: ['dashboard', 'admin', 'empleados-count', idEmp],
    queryFn: () => DashboardService.getEmpleadosCount(idEmp),
    enabled: !!idEmp,
    ...COMMON_QUERY_OPTIONS,
  })
}

export const useActividadesRealizadas = (idEmp: string) => {
  return useQuery({
    queryKey: ['dashboard', 'admin', 'actividades-finalizadas', idEmp],
    queryFn: () => DashboardService.getActividadesRealizadas(idEmp),
    enabled: !!idEmp,
    ...COMMON_QUERY_OPTIONS,
  })
}

export const useTodasAusencias = (idEmp: string) => {
  return useQuery({
    queryKey: ['dashboard', 'admin', 'ausencias-count', idEmp],
    queryFn: () => DashboardService.getTodasAusencias(idEmp),
    enabled: !!idEmp,
    ...COMMON_QUERY_OPTIONS,
  })
}

// --- CLIENTE HOOKS ---

export const useServiciosLocalidad = (idCliente: string) => {
  return useQuery({
    queryKey: ['dashboard', 'cliente', 'servicios-localidad', idCliente],
    queryFn: () => DashboardService.getServiciosLocalidad(idCliente),
    enabled: !!idCliente,
    ...COMMON_QUERY_OPTIONS,
  })
}

export const useEstatusAtencionSeguimiento = (idCliente: string) => {
  return useQuery({
    queryKey: ['dashboard', 'cliente', 'atencion-seguimiento', idCliente],
    queryFn: () => DashboardService.getEstatusAtencionSeguimiento(idCliente),
    enabled: !!idCliente,
    ...COMMON_QUERY_OPTIONS,
  })
}

// --- RH HOOKS ---

export const useTotalTrabajosActivos = (idEmp: string) => {
  return useQuery({
    queryKey: ['dashboard', 'rh', 'trabajos-activos', idEmp],
    queryFn: () => DashboardService.getTotalTrabajosActivos(idEmp),
    enabled: !!idEmp,
    ...COMMON_QUERY_OPTIONS,
  })
}

export const useTotalAusenciaTrabajadores = (idEmp: string) => {
  return useQuery({
    queryKey: ['dashboard', 'rh', 'ausencias-trabajadores', idEmp],
    queryFn: () => DashboardService.getTotalAusenciaTrabajadores(idEmp),
    enabled: !!idEmp,
    ...COMMON_QUERY_OPTIONS,
  })
}
