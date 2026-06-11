import { describe, it, expect } from 'vitest'
import {
  evaluationsResponseSchema,
  employeesCountResponseSchema,
  activitiesCountResponseSchema,
  absencesCountResponseSchema,
  servicesLocalidadResponseSchema,
  ticketStatusBreakdownResponseSchema,
  totalTrabajosActivosResponseSchema,
  totalAusenciaTrabajadoresResponseSchema
} from '../dashboard-schemas'

describe('Dashboard Zod Schemas', () => {
  it('should parse valid Admin Empresa evaluations response', () => {
    const validPayload = {
      data: {
        empresa_id: 123,
        evaluaciones: [
          {
            cliente_id: '8c2a7d0e-5412-4c22-b5e1-88f6c5bbde93',
            cliente_nombre: 'Acme Corp',
            promedio_puntuacion: 8.5,
            total_evaluaciones: 10
          }
        ]
      }
    }
    const result = evaluationsResponseSchema.safeParse(validPayload)
    expect(result.success).toBe(true)
  })

  it('should reject invalid Admin Empresa evaluations response (missing fields)', () => {
    const invalidPayload = {
      data: {
        empresa_id: 'not-a-number'
      }
    }
    const result = evaluationsResponseSchema.safeParse(invalidPayload)
    expect(result.success).toBe(false)
  })

  it('should parse valid Employees count response', () => {
    const validPayload = {
      data: {
        empresa_id: 123,
        total_empleados: 45
      }
    }
    const result = employeesCountResponseSchema.safeParse(validPayload)
    expect(result.success).toBe(true)
  })

  it('should parse valid Activities count response', () => {
    const validPayload = {
      data: {
        empresa_id: 123,
        total_actividades_finalizadas: 88,
        periodo: 'month'
      }
    }
    const result = activitiesCountResponseSchema.safeParse(validPayload)
    expect(result.success).toBe(true)
  })

  it('should parse valid Absences count response', () => {
    const validPayload = {
      data: {
        empresa_id: 123,
        total_ausencias: 12,
        year: 2026
      }
    }
    const result = absencesCountResponseSchema.safeParse(validPayload)
    expect(result.success).toBe(true)
  })

  it('should parse valid Services Localidad response', () => {
    const validPayload = {
      data: {
        cliente_id: '9b1b8c0d-e2f4-4321-a1b2-c3d4e5f6a7b8',
        localidades: [
          {
            localidad_id: '8c2a7d0e-5412-4c22-b5e1-88f6c5bbde93',
            localidad_nombre: 'Downtown',
            servicios: [
              {
                servicio_id: 1,
                servicio_nombre: 'Cleaning',
                empleados_asignados: 3
              }
            ]
          }
        ]
      }
    }
    const result = servicesLocalidadResponseSchema.safeParse(validPayload)
    expect(result.success).toBe(true)
  })

  it('should parse valid Ticket Status Breakdown response', () => {
    const validPayload = {
      data: {
        cliente_id: '9b1b8c0d-e2f4-4321-a1b2-c3d4e5f6a7b8',
        tickets: {
          pendiente: 5,
          en_proceso: 2,
          finalizado: 10
        },
        total_tickets: 17
      }
    }
    const result = ticketStatusBreakdownResponseSchema.safeParse(validPayload)
    expect(result.success).toBe(true)
  })

  it('should parse valid RH Total Trabajos Activos response', () => {
    const validPayload = {
      data: {
        empresa_id: 123,
        total_vacantes_activas: 4,
        vacantes: [
          {
            vacante_id: 'vac-1',
            titulo: 'Developer',
            departamento: 'IT',
            fecha_publicacion: '2026-06-11'
          }
        ]
      }
    }
    const result = totalTrabajosActivosResponseSchema.safeParse(validPayload)
    expect(result.success).toBe(true)
  })

  it('should parse valid RH Total Ausencia Trabajadores response', () => {
    const validPayload = {
      data: {
        empresa_id: 123,
        tasa_absentismo: 2.5,
        dias_laborables_totales: 200,
        total_inasistencias: 5,
        empleados_afectados: 1
      }
    }
    const result = totalAusenciaTrabajadoresResponseSchema.safeParse(validPayload)
    expect(result.success).toBe(true)
  })
})
