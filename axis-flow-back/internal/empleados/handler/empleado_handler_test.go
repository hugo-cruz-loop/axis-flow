package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	empleados "axis-flow-back/internal/empleados"
	"axis-flow-back/internal/empleados/handler"
	empservice "axis-flow-back/internal/empleados/service"
	"axis-flow-back/internal/middleware"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubEmpleadoService struct {
	createFn    func(context.Context, empservice.CreateEmpleadoRequest) (*empleados.Empleado, error)
	getFn       func(context.Context, int64, int64) (*empleados.Empleado, error)
	listFn      func(context.Context, int64) ([]empleados.Empleado, error)
	getByUserFn func(context.Context, uuid.UUID, int64) (*empleados.Empleado, error)
	updateFn    func(context.Context, int64, int64, empservice.UpdateEmpleadoRequest) (*empleados.Empleado, error)
	deleteFn    func(context.Context, int64, int64) error
}

func (s *stubEmpleadoService) CreateEmpleado(ctx context.Context, req empservice.CreateEmpleadoRequest) (*empleados.Empleado, error) {
	return s.createFn(ctx, req)
}
func (s *stubEmpleadoService) GetEmpleado(ctx context.Context, numEmpleado, empresaID int64) (*empleados.Empleado, error) {
	return s.getFn(ctx, numEmpleado, empresaID)
}
func (s *stubEmpleadoService) ListEmpleados(ctx context.Context, empresaID int64) ([]empleados.Empleado, error) {
	return s.listFn(ctx, empresaID)
}
func (s *stubEmpleadoService) GetEmpleadoByUser(ctx context.Context, userID uuid.UUID, empresaID int64) (*empleados.Empleado, error) {
	return s.getByUserFn(ctx, userID, empresaID)
}
func (s *stubEmpleadoService) UpdateEmpleado(ctx context.Context, numEmpleado, empresaID int64, req empservice.UpdateEmpleadoRequest) (*empleados.Empleado, error) {
	return s.updateFn(ctx, numEmpleado, empresaID, req)
}
func (s *stubEmpleadoService) DeleteEmpleado(ctx context.Context, numEmpleado, empresaID int64) error {
	return s.deleteFn(ctx, numEmpleado, empresaID)
}

type stubExpedienteStore struct {
	getUbicacionFn      func(context.Context, int64, int64) (*empleados.Ubicacion, error)
	upsertUbicacionFn   func(context.Context, *empleados.Ubicacion, int64) error
	getAdicionalesFn    func(context.Context, int64, int64) (*empleados.Adicionales, error)
	upsertAdicionalesFn func(context.Context, *empleados.Adicionales, int64) error
	getDocumentosFn     func(context.Context, int64, int64) (*empleados.Documentos, error)
	upsertDocumentosFn  func(context.Context, *empleados.Documentos, int64) error
}

func (s *stubExpedienteStore) GetUbicacion(ctx context.Context, empleadoID, empresaID int64) (*empleados.Ubicacion, error) {
	return s.getUbicacionFn(ctx, empleadoID, empresaID)
}
func (s *stubExpedienteStore) UpsertUbicacion(ctx context.Context, u *empleados.Ubicacion, empresaID int64) error {
	return s.upsertUbicacionFn(ctx, u, empresaID)
}
func (s *stubExpedienteStore) GetAdicionales(ctx context.Context, empleadoID, empresaID int64) (*empleados.Adicionales, error) {
	return s.getAdicionalesFn(ctx, empleadoID, empresaID)
}
func (s *stubExpedienteStore) UpsertAdicionales(ctx context.Context, a *empleados.Adicionales, empresaID int64) error {
	return s.upsertAdicionalesFn(ctx, a, empresaID)
}
func (s *stubExpedienteStore) GetDocumentos(ctx context.Context, empleadoID, empresaID int64) (*empleados.Documentos, error) {
	return s.getDocumentosFn(ctx, empleadoID, empresaID)
}
func (s *stubExpedienteStore) UpsertDocumentos(ctx context.Context, d *empleados.Documentos, empresaID int64) error {
	return s.upsertDocumentosFn(ctx, d, empresaID)
}

type stubAsistenciaStore struct {
	createFn     func(context.Context, *empleados.Asistencia, int64) error
	byEmpleadoFn func(context.Context, int64, int64) ([]empleados.Asistencia, error)
	byEmpresaFn  func(context.Context, int64) ([]empleados.Asistencia, error)
}

func (s *stubAsistenciaStore) Create(ctx context.Context, a *empleados.Asistencia, empresaID int64) error {
	return s.createFn(ctx, a, empresaID)
}
func (s *stubAsistenciaStore) GetByEmpleado(ctx context.Context, empleadoID, empresaID int64) ([]empleados.Asistencia, error) {
	return s.byEmpleadoFn(ctx, empleadoID, empresaID)
}
func (s *stubAsistenciaStore) GetByEmpresa(ctx context.Context, empresaID int64) ([]empleados.Asistencia, error) {
	return s.byEmpresaFn(ctx, empresaID)
}

type stubInasistenciaStore struct {
	createFn     func(context.Context, *empleados.Inasistencia, int64) error
	byEmpleadoFn func(context.Context, int64, int64) ([]empleados.Inasistencia, error)
	updateFn     func(context.Context, uuid.UUID, int64, bool, *string, uuid.UUID) error
}

func (s *stubInasistenciaStore) Create(ctx context.Context, i *empleados.Inasistencia, empresaID int64) error {
	return s.createFn(ctx, i, empresaID)
}
func (s *stubInasistenciaStore) GetByEmpleado(ctx context.Context, empleadoID, empresaID int64) ([]empleados.Inasistencia, error) {
	return s.byEmpleadoFn(ctx, empleadoID, empresaID)
}
func (s *stubInasistenciaStore) UpdateStatus(ctx context.Context, id uuid.UUID, empresaID int64, aprobado bool, observaciones *string, resolvedBy uuid.UUID) error {
	return s.updateFn(ctx, id, empresaID, aprobado, observaciones, resolvedBy)
}

type stubDeviceStore struct {
	upsertFn     func(context.Context, *empleados.UserDevice, int64) error
	byEmpleadoFn func(context.Context, int64, int64) ([]empleados.UserDevice, error)
}

func (s *stubDeviceStore) Upsert(ctx context.Context, d *empleados.UserDevice, empresaID int64) error {
	return s.upsertFn(ctx, d, empresaID)
}
func (s *stubDeviceStore) GetByEmpleado(ctx context.Context, empleadoID, empresaID int64) ([]empleados.UserDevice, error) {
	return s.byEmpleadoFn(ctx, empleadoID, empresaID)
}

type stubStorage struct {
	uploadFn func(context.Context, string, string, io.Reader, int64, string) (string, error)
}

func (s *stubStorage) Upload(ctx context.Context, bucket, key string, r io.Reader, size int64, contentType string) (string, error) {
	return s.uploadFn(ctx, bucket, key, r, size, contentType)
}
func (s *stubStorage) Delete(context.Context, string, string) error { return nil }

func newHandler(opts ...func(*handler.Handler)) *handler.Handler {
	h := handler.NewHandler(nil, nil, nil, nil, nil, nil, nil, nil, "empleados-test")
	for _, opt := range opts {
		opt(h)
	}
	return h
}

func withRouteParam(r *http.Request, key, value string) *http.Request {
	rc := chi.NewRouteContext()
	rc.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rc))
}

func postMultipart(t *testing.T, url string, fields map[string]string, fileField, fileName string, content []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for k, v := range fields {
		require.NoError(t, mw.WriteField(k, v))
	}
	fw, err := mw.CreateFormFile(fileField, fileName)
	require.NoError(t, err)
	_, err = fw.Write(content)
	require.NoError(t, err)
	require.NoError(t, mw.Close())
	req := httptest.NewRequest(http.MethodPost, url, &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

func TestCreateEmpleadoMapsRequestAndReturnsCreated(t *testing.T) {
	actorID := uuid.New()
	svc := &stubEmpleadoService{createFn: func(_ context.Context, req empservice.CreateEmpleadoRequest) (*empleados.Empleado, error) {
		assert.Equal(t, int64(12), req.EmpresaID)
		assert.Equal(t, "juan.perez@empresa.com", req.Email)
		assert.Equal(t, "EMP-001", req.IDEmpleado)
		assert.Equal(t, actorID, req.CreatedBy)
		return &empleados.Empleado{NumEmpleado: 105, IDEmpleado: req.IDEmpleado, EmpresaID: req.EmpresaID, Nombre: req.Nombre, ApellidoPaterno: req.ApellidoPaterno, Status: empleados.EmpleadoStatusIncompleto}, nil
	}}
	h := handler.NewHandler(svc, nil, nil, nil, nil, nil, nil, nil, "empleados-test")
	body, err := json.Marshal(map[string]any{"email": "juan.perez@empresa.com", "nombre": "Juan", "apellido_paterno": "Perez", "id_empleado": "EMP-001", "empresa_id": 12})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/empleado", bytes.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), middleware.ContextKeyUserID, actorID.String()))
	w := httptest.NewRecorder()

	h.CreateEmpleado(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
	var resp map[string]map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, float64(105), resp["data"]["num_empleado"])
}

func TestGetAndListEmpleadoUseEmpresaScope(t *testing.T) {
	svc := &stubEmpleadoService{
		getFn: func(_ context.Context, numEmpleado, empresaID int64) (*empleados.Empleado, error) {
			assert.Equal(t, int64(105), numEmpleado)
			assert.Equal(t, int64(12), empresaID)
			return &empleados.Empleado{NumEmpleado: numEmpleado, EmpresaID: empresaID, Nombre: "Juan"}, nil
		},
		listFn: func(_ context.Context, empresaID int64) ([]empleados.Empleado, error) {
			assert.Equal(t, int64(12), empresaID)
			return []empleados.Empleado{{NumEmpleado: 105, EmpresaID: empresaID, Nombre: "Juan"}}, nil
		},
	}
	h := handler.NewHandler(svc, nil, nil, nil, nil, nil, nil, nil, "empleados-test")

	getReq := withRouteParam(httptest.NewRequest(http.MethodGet, "/empleado/105?empresa_id=12", nil), "id", "105")
	getW := httptest.NewRecorder()
	h.GetEmpleado(getW, getReq)
	assert.Equal(t, http.StatusOK, getW.Code)

	listReq := withRouteParam(httptest.NewRequest(http.MethodGet, "/empresa/12/empleados", nil), "empresa_id", "12")
	listW := httptest.NewRecorder()
	h.ListEmpleados(listW, listReq)
	assert.Equal(t, http.StatusOK, listW.Code)
}

func TestUpdateAndDeleteEmpleado(t *testing.T) {
	svc := &stubEmpleadoService{
		updateFn: func(_ context.Context, numEmpleado, empresaID int64, req empservice.UpdateEmpleadoRequest) (*empleados.Empleado, error) {
			require.NotNil(t, req.Nombre)
			assert.Equal(t, int64(105), numEmpleado)
			assert.Equal(t, int64(12), empresaID)
			assert.Equal(t, "Juan Carlos", *req.Nombre)
			return &empleados.Empleado{NumEmpleado: numEmpleado, EmpresaID: empresaID, Nombre: *req.Nombre}, nil
		},
		deleteFn: func(_ context.Context, numEmpleado, empresaID int64) error {
			assert.Equal(t, int64(105), numEmpleado)
			assert.Equal(t, int64(12), empresaID)
			return nil
		},
	}
	h := handler.NewHandler(svc, nil, nil, nil, nil, nil, nil, nil, "empleados-test")

	body, err := json.Marshal(map[string]string{"nombre": "Juan Carlos", "apellido_paterno": "Perez"})
	require.NoError(t, err)
	putReq := withRouteParam(httptest.NewRequest(http.MethodPut, "/empleado/105?empresa_id=12", bytes.NewReader(body)), "id", "105")
	putW := httptest.NewRecorder()
	h.UpdateEmpleado(putW, putReq)
	assert.Equal(t, http.StatusOK, putW.Code)

	deleteReq := withRouteParam(httptest.NewRequest(http.MethodDelete, "/empleado/105?empresa_id=12", nil), "id", "105")
	deleteW := httptest.NewRecorder()
	h.DeleteEmpleado(deleteW, deleteReq)
	assert.Equal(t, http.StatusOK, deleteW.Code)
}

func TestExpedienteUbicacionAdicionalesAndDocumentUpload(t *testing.T) {
	var uploadedKey string
	exp := &stubExpedienteStore{
		getUbicacionFn: func(_ context.Context, empleadoID, empresaID int64) (*empleados.Ubicacion, error) {
			assert.Equal(t, int64(105), empleadoID)
			assert.Equal(t, int64(12), empresaID)
			return &empleados.Ubicacion{EmpleadoID: empleadoID, CURP: "PERJ850608HDFRRN09", NSS: "12345678901"}, nil
		},
		upsertUbicacionFn: func(_ context.Context, u *empleados.Ubicacion, empresaID int64) error {
			assert.Equal(t, int64(105), u.EmpleadoID)
			assert.Equal(t, int64(12), empresaID)
			assert.Equal(t, "06600", u.CodigoPostal)
			return nil
		},
		getAdicionalesFn: func(_ context.Context, empleadoID, empresaID int64) (*empleados.Adicionales, error) {
			return &empleados.Adicionales{EmpleadoID: empleadoID, ContactoEmergenciaNombre: "Maria", Beneficiarios: []map[string]any{{"nombre": "Ana"}}}, nil
		},
		upsertAdicionalesFn: func(_ context.Context, a *empleados.Adicionales, empresaID int64) error {
			assert.Equal(t, int64(105), a.EmpleadoID)
			assert.Len(t, a.Beneficiarios, 1)
			return nil
		},
		getDocumentosFn: func(_ context.Context, empleadoID, empresaID int64) (*empleados.Documentos, error) {
			return &empleados.Documentos{EmpleadoID: empleadoID}, nil
		},
		upsertDocumentosFn: func(_ context.Context, d *empleados.Documentos, empresaID int64) error {
			require.NotNil(t, d.ContratoURL)
			assert.Equal(t, "file://contrato.pdf", *d.ContratoURL)
			return nil
		},
	}
	storage := &stubStorage{uploadFn: func(_ context.Context, bucket, key string, r io.Reader, size int64, contentType string) (string, error) {
		assert.Equal(t, "empleados-test", bucket)
		assert.Positive(t, size)
		assert.Contains(t, key, "105/documentos/CONTRATO")
		uploadedKey = key
		return "file://contrato.pdf", nil
	}}
	h := handler.NewHandler(nil, exp, nil, nil, nil, nil, nil, storage, "empleados-test")

	getReq := withRouteParam(httptest.NewRequest(http.MethodGet, "/empleado/105/ubicacion?empresa_id=12", nil), "id", "105")
	getW := httptest.NewRecorder()
	h.GetUbicacion(getW, getReq)
	assert.Equal(t, http.StatusOK, getW.Code)

	ubicacionBody, err := json.Marshal(map[string]any{"curp": "PERJ850608HDFRRN09", "nss": "12345678901", "calle": "Reforma", "numero_exterior": "123", "colonia": "Juarez", "codigo_postal": "06600", "ciudad_id": 102, "estado_id": 15, "pais_id": 1})
	require.NoError(t, err)
	putReq := withRouteParam(httptest.NewRequest(http.MethodPut, "/empleado/105/ubicacion?empresa_id=12", bytes.NewReader(ubicacionBody)), "id", "105")
	putW := httptest.NewRecorder()
	h.UpdateUbicacion(putW, putReq)
	assert.Equal(t, http.StatusOK, putW.Code)

	adicionalesBody, err := json.Marshal(map[string]any{"contacto_emergencia_nombre": "Maria", "contacto_emergencia_telefono": "5512345678", "contacto_emergencia_parentesco": "Hermana", "beneficiarios": []map[string]any{{"nombre": "Ana", "porcentaje": 100}}})
	require.NoError(t, err)
	adicionalesReq := withRouteParam(httptest.NewRequest(http.MethodPut, "/empleado/105/adicionales?empresa_id=12", bytes.NewReader(adicionalesBody)), "id", "105")
	adicionalesW := httptest.NewRecorder()
	h.UpdateAdicionales(adicionalesW, adicionalesReq)
	assert.Equal(t, http.StatusOK, adicionalesW.Code)

	docReq := withRouteParam(postMultipart(t, "/empleado/105/documentos?empresa_id=12", map[string]string{"tipo_documento": "CONTRATO"}, "archivo", "contrato.pdf", []byte("%PDF test")), "id", "105")
	docW := httptest.NewRecorder()
	h.UploadDocumento(docW, docReq)
	assert.Equal(t, http.StatusCreated, docW.Code)
	assert.NotEmpty(t, uploadedKey)
}

func TestAsistenciaInasistenciaAndDevicesHandlers(t *testing.T) {
	asistenciaID := uuid.New()
	asistencias := &stubAsistenciaStore{
		createFn: func(_ context.Context, a *empleados.Asistencia, empresaID int64) error {
			assert.Equal(t, int64(105), a.EmpleadoID)
			assert.Equal(t, int64(12), empresaID)
			assert.Equal(t, empleados.TipoEntradaLaboral, a.TipoRegistro)
			a.ID = asistenciaID
			a.EstatusObservacionEntrada = empleados.ObservacionPendiente
			a.CreatedAt = time.Date(2026, 6, 8, 14, 4, 0, 0, time.UTC)
			return nil
		},
		byEmpleadoFn: func(_ context.Context, empleadoID, empresaID int64) ([]empleados.Asistencia, error) {
			assert.Equal(t, int64(12), empresaID)
			return []empleados.Asistencia{{ID: asistenciaID, EmpleadoID: empleadoID, TipoRegistro: empleados.TipoEntradaLaboral}}, nil
		},
	}
	inasistencias := &stubInasistenciaStore{
		createFn: func(_ context.Context, i *empleados.Inasistencia, empresaID int64) error {
			assert.Equal(t, "2", i.TipoIncidencia)
			assert.Equal(t, int64(12), empresaID)
			i.ID = uuid.New()
			return nil
		},
		byEmpleadoFn: func(_ context.Context, empleadoID, empresaID int64) ([]empleados.Inasistencia, error) {
			return []empleados.Inasistencia{{ID: uuid.New(), EmpleadoID: empleadoID, TipoIncidencia: "2"}}, nil
		},
		updateFn: func(_ context.Context, id uuid.UUID, empresaID int64, aprobado bool, observaciones *string, resolvedBy uuid.UUID) error {
			assert.Equal(t, int64(12), empresaID)
			assert.True(t, aprobado)
			require.NotNil(t, observaciones)
			assert.Equal(t, "Aprobada", *observaciones)
			assert.NotEqual(t, uuid.Nil, resolvedBy)
			return nil
		},
	}
	devices := &stubDeviceStore{
		upsertFn: func(_ context.Context, d *empleados.UserDevice, empresaID int64) error {
			assert.Equal(t, int64(105), d.EmpleadoID)
			assert.Equal(t, int64(12), empresaID)
			assert.Equal(t, "device-1", d.DeviceUUID)
			d.ID = uuid.New()
			d.IsActive = true
			return nil
		},
		byEmpleadoFn: func(_ context.Context, empleadoID, empresaID int64) ([]empleados.UserDevice, error) {
			return []empleados.UserDevice{{ID: uuid.New(), EmpleadoID: empleadoID, DeviceUUID: "device-1", IsActive: true}}, nil
		},
	}
	storage := &stubStorage{uploadFn: func(_ context.Context, bucket, key string, r io.Reader, size int64, contentType string) (string, error) {
		assert.Contains(t, key, "105/asistencias")
		return "file://asistencia.jpg", nil
	}}
	h := handler.NewHandler(nil, nil, asistencias, inasistencias, devices, nil, nil, storage, "empleados-test")

	asistenciaReq := postMultipart(t, "/empleado/asistencias?empresa_id=12&empleado_id=105", map[string]string{"tipo_registro": empleados.TipoEntradaLaboral, "geolocalizacion": "19.2891,-99.6534", "estatus_rango": "1"}, "foto", "entrada.jpg", []byte{0xFF, 0xD8, 0xFF, 0x00})
	asistenciaW := httptest.NewRecorder()
	h.CreateAsistencia(asistenciaW, asistenciaReq)
	assert.Equal(t, http.StatusCreated, asistenciaW.Code)

	listAsistenciaReq := httptest.NewRequest(http.MethodGet, "/empleado/asistencias?empresa_id=12&empleado_id=105", nil)
	listAsistenciaW := httptest.NewRecorder()
	h.ListAsistencias(listAsistenciaW, listAsistenciaReq)
	assert.Equal(t, http.StatusOK, listAsistenciaW.Code)

	inasistenciaBody, err := json.Marshal(map[string]any{"empleado_id": 105, "tipo_inasistencia_id": 2, "fecha_inicio": "2026-06-15", "fecha_fin": "2026-06-20", "motivo": "Vacaciones"})
	require.NoError(t, err)
	inasistenciaReq := httptest.NewRequest(http.MethodPost, "/empleado/inasistencia?empresa_id=12", bytes.NewReader(inasistenciaBody))
	inasistenciaW := httptest.NewRecorder()
	h.CreateInasistencia(inasistenciaW, inasistenciaReq)
	assert.Equal(t, http.StatusCreated, inasistenciaW.Code)

	resolverID := uuid.New()
	resolvedBy := uuid.New()
	resolverBody, err := json.Marshal(map[string]any{"status": "APROBADA", "observaciones": "Aprobada"})
	require.NoError(t, err)
	resolverReq := withRouteParam(httptest.NewRequest(http.MethodPut, "/empleado/inasistencia/"+resolverID.String()+"?empresa_id=12", bytes.NewReader(resolverBody)), "id", resolverID.String())
	resolverReq = resolverReq.WithContext(context.WithValue(resolverReq.Context(), middleware.ContextKeyUserID, resolvedBy.String()))
	resolverW := httptest.NewRecorder()
	h.ResolveInasistencia(resolverW, resolverReq)
	assert.Equal(t, http.StatusOK, resolverW.Code)

	deviceBody, err := json.Marshal(map[string]any{"empleado_id": 105, "device_id": "device-1", "device_name": "Pixel", "device_type": "ANDROID", "token_firebase": "token-secret"})
	require.NoError(t, err)
	deviceReq := httptest.NewRequest(http.MethodPost, "/empleado/devices?empresa_id=12", bytes.NewReader(deviceBody))
	deviceW := httptest.NewRecorder()
	h.RegisterDevice(deviceW, deviceReq)
	assert.Equal(t, http.StatusCreated, deviceW.Code)
	assert.NotContains(t, deviceW.Body.String(), "token-secret")

	listDeviceReq := httptest.NewRequest(http.MethodGet, "/empleado/devices?empresa_id=12&empleado_id=105", nil)
	listDeviceW := httptest.NewRecorder()
	h.ListDevices(listDeviceW, listDeviceReq)
	assert.Equal(t, http.StatusOK, listDeviceW.Code)
}

func TestErrorsUseProblemEnvelope(t *testing.T) {
	svc := &stubEmpleadoService{getFn: func(context.Context, int64, int64) (*empleados.Empleado, error) {
		return nil, empleados.ErrEmpleadoNotFound
	}}
	h := handler.NewHandler(svc, nil, nil, nil, nil, nil, nil, nil, "empleados-test")
	req := withRouteParam(httptest.NewRequest(http.MethodGet, "/empleado/105?empresa_id=12", nil), "id", "105")
	w := httptest.NewRecorder()

	h.GetEmpleado(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)
	var resp map[string]map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "NOT_FOUND", resp["error"]["code"])
}
