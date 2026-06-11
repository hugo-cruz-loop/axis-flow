import axios from 'axios'
import { useAuthStore } from '@/store/authStore'
import {
  evaluationsResponseSchema,
  employeesCountResponseSchema,
  activitiesCountResponseSchema,
  absencesCountResponseSchema,
  servicesLocalidadResponseSchema,
  ticketStatusBreakdownResponseSchema,
  totalTrabajosActivosResponseSchema,
  totalAusenciaTrabajadoresResponseSchema,
  type ClientEvaluationItem,
  type ServicesLocalidadItem,
  type TicketStatusBreakdown,
  type AbsenteeismMetrics,
} from '../schemas/dashboard-schemas'

export const dashboardClient = axios.create({
  baseURL:
    (import.meta.env?.VITE_API_BASE_URL as string | undefined) || '',
  timeout: 10000,
})

dashboardClient.interceptors.request.use((config) => {
  const token = useAuthStore.getState().accessToken
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

export const DashboardService = {
  // --- Admin Empresa Endpoints ---
  getEvaluacionesClientes: async (idEmp: string): Promise<ClientEvaluationItem[]> => {
    const response = await dashboardClient.get(`/api/v1/dashboards/admin-empresa/${idEmp}/evaluaciones-clientes`)
    const parsed = evaluationsResponseSchema.parse(response.data)
    return parsed.data.evaluaciones
  },

  getEmpleadosCount: async (idEmp: string) => {
    const response = await dashboardClient.get(`/api/v1/dashboards/admin-empresa/${idEmp}/empleados/count`)
    const parsed = employeesCountResponseSchema.parse(response.data)
    return parsed.data
  },

  getActividadesRealizadas: async (idEmp: string) => {
    const response = await dashboardClient.get(`/api/v1/dashboards/admin-empresa/${idEmp}/actividades/count`)
    const parsed = activitiesCountResponseSchema.parse(response.data)
    return parsed.data
  },

  getTodasAusencias: async (idEmp: string) => {
    const response = await dashboardClient.get(`/api/v1/dashboards/admin-empresa/${idEmp}/ausencias/count`)
    const parsed = absencesCountResponseSchema.parse(response.data)
    return {
      empresa_id: parsed.data.empresa_id,
      total_ausencias: parsed.data.total_ausencias,
      total_historico_inasistencias: parsed.data.total_ausencias,
      year: parsed.data.year,
    }
  },

  // --- Cliente Endpoints ---
  getServiciosLocalidad: async (idCliente: string): Promise<ServicesLocalidadItem[]> => {
    const response = await dashboardClient.get(`/api/v1/dashboards/cliente/${idCliente}/servicios-localidad`)
    const parsed = servicesLocalidadResponseSchema.parse(response.data)
    
    const flatItems: ServicesLocalidadItem[] = []
    parsed.data.localidades.forEach((loc) => {
      loc.servicios.forEach((srv) => {
        flatItems.push({
          localidad_id: loc.localidad_id,
          localidad_nombre: loc.localidad_nombre,
          servicio_id: String(srv.servicio_id),
          servicio_nombre: srv.servicio_nombre,
          empleados_asignados: srv.empleados_asignados,
        })
      })
    })
    return flatItems
  },

  getEstatusAtencionSeguimiento: async (idCliente: string): Promise<TicketStatusBreakdown> => {
    const response = await dashboardClient.get(`/api/v1/dashboards/cliente/${idCliente}/atencion-seguimiento/status`)
    const parsed = ticketStatusBreakdownResponseSchema.parse(response.data)
    return {
      cliente_id: parsed.data.cliente_id,
      pendiente: parsed.data.tickets.pendiente,
      en_proceso: parsed.data.tickets.en_proceso,
      finalizado: parsed.data.tickets.finalizado,
      total_tickets: parsed.data.total_tickets,
      pendientes: parsed.data.tickets.pendiente,
      finalizados: parsed.data.tickets.finalizado,
    }
  },

  // --- RH Endpoints ---
  getTotalTrabajosActivos: async (idEmp: string) => {
    const response = await dashboardClient.get(`/api/v1/dashboards/rh/${idEmp}/bolsa-trabajo/vacantes-activas`)
    const parsed = totalTrabajosActivosResponseSchema.parse(response.data)
    return parsed.data
  },

  getTotalAusenciaTrabajadores: async (idEmp: string): Promise<AbsenteeismMetrics> => {
    const response = await dashboardClient.get(`/api/v1/dashboards/rh/${idEmp}/empleados/absentismo`)
    const parsed = totalAusenciaTrabajadoresResponseSchema.parse(response.data)
    return {
      empresa_id: parsed.data.empresa_id,
      tasa_absentismo: parsed.data.tasa_absentismo,
      dias_laborables_totales: parsed.data.dias_laborables_totales,
      total_inasistencias: parsed.data.total_inasistencias,
      empleados_afectados: parsed.data.empleados_afectados,
      total_ausencias_actuales: parsed.data.total_inasistencias,
    }
  },
}
