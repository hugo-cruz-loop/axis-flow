import { describe, it, expect, vi, beforeEach } from 'vitest'

// Use vi.hoisted so the mock instance is created before the axios mock factory
// runs (vi.mock is hoisted to the top of the file, before any imports).
const mocks = vi.hoisted(() => {
  const mockPost = vi.fn()
  const mockGet = vi.fn()
  const mockAxiosInstance = {
    post: mockPost,
    get: mockGet,
    interceptors: {
      request: { use: vi.fn() },
      response: { use: vi.fn() },
    },
    defaults: { headers: { common: {} } },
  }
  return { mockPost, mockGet, mockAxiosInstance }
})

vi.mock('axios', () => ({
  default: { create: vi.fn(() => mocks.mockAxiosInstance) },
}))

vi.mock('@/store/authStore', () => ({
  useAuthStore: { getState: () => ({ accessToken: 'test-token' }) },
}))

// Import after the mocks are in place.
import { formulariosClient } from '../api/formulariosClient'

const UUID = '8c2a7d0e-5412-4c22-b5e1-88f6c5bbde93'
const UUID2 = 'd041e2a8-0e1b-4d43-85f6-cbb18a4a5119'

beforeEach(() => {
  vi.clearAllMocks()
})

describe('formulariosClient', () => {
  describe('createFormulario', () => {
    it('POSTs to /formulario and parses response', async () => {
      mocks.mockPost.mockResolvedValueOnce({
        data: {
          success: true,
          data: {
            id: UUID,
            empresa_id: UUID,
            nombre: 'Limpieza',
            descripcion: '',
            activo: true,
          },
        },
      })

      const result = await formulariosClient.createFormulario({
        empresa_id: UUID,
        nombre: 'Limpieza',
        activo: true,
      })

      expect(mocks.mockPost).toHaveBeenCalledWith('/formulario', {
        empresa_id: UUID,
        nombre: 'Limpieza',
        activo: true,
      })
      expect(result.data.id).toBe(UUID)
    })
  })

  describe('getFormulariosByEmpresa', () => {
    it('GETs /formulario/byempresa/{id} with query params and parses list', async () => {
      mocks.mockGet.mockResolvedValueOnce({
        data: {
          success: true,
          data: [
            { id: UUID, empresa_id: UUID, nombre: 'F1', activo: true },
          ],
        },
      })

      const result = await formulariosClient.getFormulariosByEmpresa(UUID, {
        activo: true,
        page: 1,
        limit: 10,
      })

      expect(mocks.mockGet).toHaveBeenCalledWith(`/formulario/byempresa/${UUID}`, {
        params: { activo: true, page: 1, limit: 10 },
      })
      expect(result.data).toHaveLength(1)
    })

    it('works without query params', async () => {
      mocks.mockGet.mockResolvedValueOnce({ data: { success: true, data: [] } })

      await formulariosClient.getFormulariosByEmpresa(UUID)

      expect(mocks.mockGet).toHaveBeenCalledWith(`/formulario/byempresa/${UUID}`, {
        params: { activo: undefined, page: undefined, limit: undefined },
      })
    })
  })

  describe('createPregunta', () => {
    it('POSTs to /pregunta with the create body', async () => {
      mocks.mockPost.mockResolvedValueOnce({
        data: {
          success: true,
          data: {
            id: UUID2,
            formulario_id: UUID,
            orden: 1,
            texto_pregunta: 'Q',
            tipo_pregunta: 1,
            obligatoria: false,
          },
        },
      })

      const result = await formulariosClient.createPregunta({
        formulario_id: UUID,
        orden: 1,
        texto_pregunta: 'Q',
        tipo_pregunta: 1,
        obligatoria: false,
      })

      expect(mocks.mockPost).toHaveBeenCalledWith(
        '/pregunta',
        expect.objectContaining({
          formulario_id: UUID,
          texto_pregunta: 'Q',
        }),
      )
      expect(result.data.texto_pregunta).toBe('Q')
    })
  })

  describe('createEvento', () => {
    it('POSTs to /evento with the create body', async () => {
      mocks.mockPost.mockResolvedValueOnce({
        data: {
          success: true,
          data: {
            id: UUID,
            empresa_id: UUID,
            cliente_id: UUID2,
            nombre: 'Ronda',
            fecha_programada: '2026-06-10T22:00:00Z',
            estatus: 'pendiente',
          },
        },
      })

      const result = await formulariosClient.createEvento({
        empresa_id: UUID,
        cliente_id: UUID2,
        nombre: 'Ronda',
        fecha_programada: '2026-06-10T22:00:00Z',
      })

      expect(mocks.mockPost).toHaveBeenCalledWith(
        '/evento',
        expect.objectContaining({
          nombre: 'Ronda',
          cliente_id: UUID2,
        }),
      )
      expect(result.data.nombre).toBe('Ronda')
    })
  })

  describe('iniciarEvento', () => {
    it('POSTs to /evento_iniciado with evento_id and empleado_id', async () => {
      mocks.mockPost.mockResolvedValueOnce({
        data: {
          success: true,
          data: {
            id: UUID2,
            evento_id: UUID,
            empleado_id: 5,
            fecha_inicio: '2026-06-10T22:00:00Z',
            estatus: 'iniciado',
          },
        },
      })

      const result = await formulariosClient.iniciarEvento({
        evento_id: UUID,
        empleado_id: 5,
      })

      expect(mocks.mockPost).toHaveBeenCalledWith(
        '/evento_iniciado',
        expect.objectContaining({
          evento_id: UUID,
          empleado_id: 5,
        }),
      )
      expect(result.data.estatus).toBe('iniciado')
    })
  })

  describe('getEventosByEmpCte', () => {
    it('GETs /evento/byEmpId/{empId}/{cteId} with query params', async () => {
      mocks.mockGet.mockResolvedValueOnce({ data: { success: true, data: [] } })

      await formulariosClient.getEventosByEmpCte(UUID, UUID2, {
        estatus: 'pendiente',
        page: 1,
        limit: 10,
      })

      expect(mocks.mockGet).toHaveBeenCalledWith(`/evento/byEmpId/${UUID}/${UUID2}`, {
        params: { estatus: 'pendiente', page: 1, limit: 10 },
      })
    })
  })

  describe('submitRespuestaJSON', () => {
    it('POSTs JSON to /respuesta', async () => {
      mocks.mockPost.mockResolvedValueOnce({
        data: {
          success: true,
          data: {
            id: UUID,
            evento_iniciado_id: UUID,
            formulario_id: UUID,
            pregunta_id: UUID,
            respuesta_lista: { foo: 'bar' },
          },
        },
      })

      const result = await formulariosClient.submitRespuestaJSON({
        evento_iniciado_id: UUID,
        formulario_id: UUID,
        pregunta_id: UUID,
        respuesta_lista: { foo: 'bar' },
      })

      expect(mocks.mockPost).toHaveBeenCalledWith(
        '/respuesta',
        expect.objectContaining({
          pregunta_id: UUID,
          respuesta_lista: { foo: 'bar' },
        }),
      )
      expect(result.data.respuesta_lista).toEqual({ foo: 'bar' })
    })
  })

  describe('submitRespuestaMultipart', () => {
    it('POSTs multipart/form-data to /respuesta and surfaces 501 as a user-friendly error', async () => {
      const err501 = Object.assign(new Error('Request failed'), {
        response: {
          status: 501,
          data: { success: false, error: { code: 'MULTIPART_UPLOAD_NOT_IMPLEMENTED' } },
        },
      })
      mocks.mockPost.mockRejectedValueOnce(err501)

      const file = new File(['x'], 'evidence.jpg', { type: 'image/jpeg' })
      const form = new FormData()
      form.append('evento_iniciado_id', UUID)
      form.append('formulario_id', UUID)
      form.append('pregunta_id', UUID)
      form.append('evidencia1', file)

      await expect(formulariosClient.submitRespuestaMultipart(form)).rejects.toThrow(
        /photo upload via mobile camera is not yet enabled/i,
      )

      expect(mocks.mockPost).toHaveBeenCalledWith(
        '/respuesta',
        form,
        expect.objectContaining({
          headers: expect.objectContaining({ 'Content-Type': 'multipart/form-data' }),
        }),
      )
    })
  })

  describe('downloadReportePDF', () => {
    it('GETs the PDF endpoint with responseType=blob and returns the Blob', async () => {
      const fakeBlob = new Blob(['%PDF-1.4 fake'], { type: 'application/pdf' })
      mocks.mockGet.mockResolvedValueOnce({ data: fakeBlob })

      const result = await formulariosClient.downloadReportePDF(UUID)

      expect(mocks.mockGet).toHaveBeenCalledWith(
        `/respuesta/reporte/pregunta/${UUID}/pdf`,
        expect.objectContaining({ responseType: 'blob' }),
      )
      expect(result).toBe(fakeBlob)
    })
  })
})
