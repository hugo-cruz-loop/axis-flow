package empleados_test

import (
	"testing"

	"axis-flow-back/internal/empleados"
)

func TestConstants(t *testing.T) {
	if empleados.EmpleadoStatusActivo != 1 {
		t.Errorf("EmpleadoStatusActivo: want 1, got %d", empleados.EmpleadoStatusActivo)
	}
	if empleados.EmpleadoStatusIncompleto != 2 {
		t.Errorf("EmpleadoStatusIncompleto: want 2, got %d", empleados.EmpleadoStatusIncompleto)
	}
	if empleados.EmpleadoStatusBaja != 4 {
		t.Errorf("EmpleadoStatusBaja: want 4, got %d", empleados.EmpleadoStatusBaja)
	}
	if empleados.ObservacionPendiente != 1 {
		t.Errorf("ObservacionPendiente: want 1, got %d", empleados.ObservacionPendiente)
	}
	if empleados.ObservacionValidada != 2 {
		t.Errorf("ObservacionValidada: want 2, got %d", empleados.ObservacionValidada)
	}
	if empleados.ObservacionRechazada != 3 {
		t.Errorf("ObservacionRechazada: want 3, got %d", empleados.ObservacionRechazada)
	}
	if empleados.RangoEnRango != 1 {
		t.Errorf("RangoEnRango: want 1, got %d", empleados.RangoEnRango)
	}
	if empleados.RangoFueraRango != 2 {
		t.Errorf("RangoFueraRango: want 2, got %d", empleados.RangoFueraRango)
	}
	if empleados.TipoEntradaLaboral != "ENTRADA_LABORAL" {
		t.Errorf("TipoEntradaLaboral: want ENTRADA_LABORAL, got %s", empleados.TipoEntradaLaboral)
	}
	if empleados.TipoSalidaLaboral != "SALIDA_LABORAL" {
		t.Errorf("TipoSalidaLaboral: want SALIDA_LABORAL, got %s", empleados.TipoSalidaLaboral)
	}
	if empleados.TipoEntradaComida != "ENTRADA_COMIDA" {
		t.Errorf("TipoEntradaComida: want ENTRADA_COMIDA, got %s", empleados.TipoEntradaComida)
	}
	if empleados.TipoSalidaComida != "SALIDA_COMIDA" {
		t.Errorf("TipoSalidaComida: want SALIDA_COMIDA, got %s", empleados.TipoSalidaComida)
	}
}

func TestSentinelErrors(t *testing.T) {
	errs := []error{
		empleados.ErrEmpleadoNotFound,
		empleados.ErrEmpleadoAlreadyExists,
		empleados.ErrDuplicateCURP,
		empleados.ErrDuplicateNSS,
		empleados.ErrDuplicateIdEmpleado,
		empleados.ErrTenantMismatch,
		empleados.ErrAsistenciaImmutable,
		empleados.ErrDeviceNotFound,
		empleados.ErrInvalidMIMEType,
		empleados.ErrFileTooLarge,
	}
	for _, err := range errs {
		if err == nil {
			t.Errorf("sentinel error must not be nil")
		}
	}
}

func TestDomainStructsCompile(t *testing.T) {
	// Verifies struct fields exist and are addressable at compile time.
	var e empleados.Empleado
	_ = e.NumEmpleado
	_ = e.IDEmpleado
	_ = e.UsuarioID
	_ = e.EmpresaID
	_ = e.Nombre
	_ = e.ApellidoPaterno
	_ = e.ApellidoMaterno
	_ = e.Status
	_ = e.CreatedAt
	_ = e.UpdatedAt

	var u empleados.Ubicacion
	_ = u.EmpleadoID
	_ = u.CURP
	_ = u.NSS

	var ad empleados.Adicionales
	_ = ad.EmpleadoID
	_ = ad.Beneficiarios

	var doc empleados.Documentos
	_ = doc.EmpleadoID
	_ = doc.EstatusValidacion

	var a empleados.Asistencia
	_ = a.ID
	_ = a.EstatusObservacionEntrada
	_ = a.SimilitudFacial

	var ac empleados.AsistenciaComida
	_ = ac.AsistenciaID

	var ina empleados.Inasistencia
	_ = ina.ResolvedBy

	var f empleados.Fotologin
	_ = f.FotoBaseURL

	var d empleados.UserDevice
	_ = d.FCMToken
	_ = d.IsActive
}
