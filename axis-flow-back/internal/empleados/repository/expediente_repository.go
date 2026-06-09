package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	empleados "axis-flow-back/internal/empleados"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ExpedienteRepository persists expediente subresources.
type ExpedienteRepository struct{ db dbConn }

func NewExpedienteRepository(db dbConn) *ExpedienteRepository { return &ExpedienteRepository{db: db} }
func NewPgxExpedienteRepository(pool *pgxpool.Pool) *ExpedienteRepository {
	return NewExpedienteRepository(pgxDB{pool: pool})
}

func (r *ExpedienteRepository) GetUbicacion(ctx context.Context, empleadoID, empresaID int64) (*empleados.Ubicacion, error) {
	var u empleados.Ubicacion
	err := r.db.QueryRow(ctx,
		`SELECT u.empleado_id, u.curp, u.nss, u.calle, u.numero_exterior, COALESCE(u.numero_interior,''),
                u.colonia, u.codigo_postal, u.ciudad_id, u.estado_id, u.pais_id, u.created_at, u.updated_at
         FROM empleados.empleados_ubicacion u
         JOIN empleados.empleados_empleado e ON e.num_empleado=u.empleado_id
         WHERE u.empleado_id=$1 AND e.empresa_id=$2`, empleadoID, empresaID).
		Scan(&u.EmpleadoID, &u.CURP, &u.NSS, &u.Calle, &u.NumeroExterior, &u.NumeroInterior, &u.Colonia, &u.CodigoPostal, &u.CiudadID, &u.EstadoID, &u.PaisID, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, empleados.ErrEmpleadoNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("expedienteRepository.GetUbicacion: %w", err)
	}
	return &u, nil
}

func (r *ExpedienteRepository) UpsertUbicacion(ctx context.Context, u *empleados.Ubicacion, empresaID int64) error {
	tag, err := r.db.Exec(ctx,
		`INSERT INTO empleados.empleados_ubicacion
            (empleado_id, curp, nss, calle, numero_exterior, numero_interior, colonia, codigo_postal, ciudad_id, estado_id, pais_id)
         SELECT $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11
         WHERE EXISTS (SELECT 1 FROM empleados.empleados_empleado e WHERE e.num_empleado=$1 AND e.empresa_id=$12)
         ON CONFLICT (empleado_id) DO UPDATE SET
            curp=EXCLUDED.curp, nss=EXCLUDED.nss, calle=EXCLUDED.calle, numero_exterior=EXCLUDED.numero_exterior,
            numero_interior=EXCLUDED.numero_interior, colonia=EXCLUDED.colonia, codigo_postal=EXCLUDED.codigo_postal,
            ciudad_id=EXCLUDED.ciudad_id, estado_id=EXCLUDED.estado_id, pais_id=EXCLUDED.pais_id, updated_at=NOW()`,
		u.EmpleadoID, u.CURP, u.NSS, u.Calle, u.NumeroExterior, u.NumeroInterior, u.Colonia, u.CodigoPostal, u.CiudadID, u.EstadoID, u.PaisID, empresaID)
	if err != nil {
		return mapUbicacionUpsertError(err)
	}
	if rowsAffected(tag) == 0 {
		return empleados.ErrEmpleadoNotFound
	}
	return nil
}

func (r *ExpedienteRepository) GetAdicionales(ctx context.Context, empleadoID, empresaID int64) (*empleados.Adicionales, error) {
	var a empleados.Adicionales
	var beneficiarios []byte
	err := r.db.QueryRow(ctx,
		`SELECT a.empleado_id, a.contacto_emergencia_nombre, a.contacto_emergencia_telefono,
                a.contacto_emergencia_parentesco, a.beneficiarios::jsonb, a.created_at, a.updated_at
         FROM empleados.empleados_adicionales a
         JOIN empleados.empleados_empleado e ON e.num_empleado=a.empleado_id
         WHERE a.empleado_id=$1 AND e.empresa_id=$2`, empleadoID, empresaID).
		Scan(&a.EmpleadoID, &a.ContactoEmergenciaNombre, &a.ContactoEmergenciaTelefono, &a.ContactoEmergenciaParentesco, &beneficiarios, &a.CreatedAt, &a.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, empleados.ErrEmpleadoNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("expedienteRepository.GetAdicionales: %w", err)
	}
	if err := json.Unmarshal(beneficiarios, &a.Beneficiarios); err != nil {
		return nil, fmt.Errorf("expedienteRepository.GetAdicionales beneficiaries: %w", err)
	}
	return &a, nil
}

func (r *ExpedienteRepository) UpsertAdicionales(ctx context.Context, a *empleados.Adicionales, empresaID int64) error {
	beneficiarios, err := json.Marshal(a.Beneficiarios)
	if err != nil {
		return fmt.Errorf("expedienteRepository.UpsertAdicionales beneficiaries: %w", err)
	}
	tag, err := r.db.Exec(ctx,
		`INSERT INTO empleados.empleados_adicionales
            (empleado_id, contacto_emergencia_nombre, contacto_emergencia_telefono, contacto_emergencia_parentesco, beneficiarios)
         SELECT $1,$2,$3,$4,$5::jsonb
         WHERE EXISTS (SELECT 1 FROM empleados.empleados_empleado e WHERE e.num_empleado=$1 AND e.empresa_id=$6)
         ON CONFLICT (empleado_id) DO UPDATE SET
            contacto_emergencia_nombre=EXCLUDED.contacto_emergencia_nombre,
            contacto_emergencia_telefono=EXCLUDED.contacto_emergencia_telefono,
            contacto_emergencia_parentesco=EXCLUDED.contacto_emergencia_parentesco,
            beneficiarios=EXCLUDED.beneficiarios,
            updated_at=NOW()`,
		a.EmpleadoID, a.ContactoEmergenciaNombre, a.ContactoEmergenciaTelefono, a.ContactoEmergenciaParentesco, beneficiarios, empresaID)
	if err != nil {
		return fmt.Errorf("expedienteRepository.UpsertAdicionales: %w", err)
	}
	if rowsAffected(tag) == 0 {
		return empleados.ErrEmpleadoNotFound
	}
	return nil
}

func (r *ExpedienteRepository) GetDocumentos(ctx context.Context, empleadoID, empresaID int64) (*empleados.Documentos, error) {
	var d empleados.Documentos
	err := r.db.QueryRow(ctx,
		`SELECT d.empleado_id, d.acta_url, d.ine_url, d.comprobante_domicilio_url, d.curp_pdf_url, d.nss_pdf_url,
                d.contrato_url, d.estatus_validacion, d.created_at, d.updated_at
         FROM empleados.empleados_documentos d
         JOIN empleados.empleados_empleado e ON e.num_empleado=d.empleado_id
         WHERE d.empleado_id=$1 AND e.empresa_id=$2`, empleadoID, empresaID).
		Scan(&d.EmpleadoID, &d.ActaURL, &d.INEURL, &d.ComprobanteDomicilioURL, &d.CURPPdfURL, &d.NSSPdfURL, &d.ContratoURL, &d.EstatusValidacion, &d.CreatedAt, &d.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, empleados.ErrEmpleadoNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("expedienteRepository.GetDocumentos: %w", err)
	}
	return &d, nil
}

func (r *ExpedienteRepository) UpsertDocumentos(ctx context.Context, d *empleados.Documentos, empresaID int64) error {
	tag, err := r.db.Exec(ctx,
		`INSERT INTO empleados.empleados_documentos
            (empleado_id, acta_url, ine_url, comprobante_domicilio_url, curp_pdf_url, nss_pdf_url, contrato_url, estatus_validacion)
         SELECT $1,$2,$3,$4,$5,$6,$7,$8
         WHERE EXISTS (SELECT 1 FROM empleados.empleados_empleado e WHERE e.num_empleado=$1 AND e.empresa_id=$9)
         ON CONFLICT (empleado_id) DO UPDATE SET
            acta_url=EXCLUDED.acta_url, ine_url=EXCLUDED.ine_url, comprobante_domicilio_url=EXCLUDED.comprobante_domicilio_url,
            curp_pdf_url=EXCLUDED.curp_pdf_url, nss_pdf_url=EXCLUDED.nss_pdf_url, contrato_url=EXCLUDED.contrato_url,
            estatus_validacion=EXCLUDED.estatus_validacion, updated_at=NOW()`,
		d.EmpleadoID, d.ActaURL, d.INEURL, d.ComprobanteDomicilioURL, d.CURPPdfURL, d.NSSPdfURL, d.ContratoURL, d.EstatusValidacion, empresaID)
	if err != nil {
		return fmt.Errorf("expedienteRepository.UpsertDocumentos: %w", err)
	}
	if rowsAffected(tag) == 0 {
		return empleados.ErrEmpleadoNotFound
	}
	return nil
}

// ListFotologin returns all biometric base photos for an employee scoped to empresa.
func (r *ExpedienteRepository) ListFotologin(ctx context.Context, empleadoID, empresaID int64) ([]empleados.Fotologin, error) {
	rows, err := r.db.Query(ctx,
		`SELECT f.id, f.empleado_id, f.foto_base_url, f.created_at, f.updated_at
         FROM empleados.empleados_fotologin f
         JOIN empleados.empleados_empleado e ON e.num_empleado=f.empleado_id
         WHERE f.empleado_id=$1 AND e.empresa_id=$2
         ORDER BY f.created_at ASC`, empleadoID, empresaID)
	if err != nil {
		return nil, fmt.Errorf("expedienteRepository.ListFotologin: %w", err)
	}
	defer rows.Close()
	out := []empleados.Fotologin{}
	for rows.Next() {
		var f empleados.Fotologin
		if err := rows.Scan(&f.ID, &f.EmpleadoID, &f.FotoBaseURL, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, fmt.Errorf("expedienteRepository.ListFotologin scan: %w", err)
		}
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("expedienteRepository.ListFotologin rows: %w", err)
	}
	return out, nil
}

// CreateFotologin inserts a new biometric base photo for an employee.
func (r *ExpedienteRepository) CreateFotologin(ctx context.Context, f *empleados.Fotologin, empresaID int64) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO empleados.empleados_fotologin (empleado_id, foto_base_url)
         SELECT $1,$2
         WHERE EXISTS (SELECT 1 FROM empleados.empleados_empleado e WHERE e.num_empleado=$1 AND e.empresa_id=$3)
         RETURNING id, created_at, updated_at`,
		f.EmpleadoID, f.FotoBaseURL, empresaID).
		Scan(&f.ID, &f.CreatedAt, &f.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return empleados.ErrEmpleadoNotFound
	}
	if err != nil {
		return fmt.Errorf("expedienteRepository.CreateFotologin: %w", err)
	}
	return nil
}

// KPIDocumentosComplete returns the count of employees with all 6 document fields non-null for an empresa.
func (r *ExpedienteRepository) KPIDocumentosComplete(ctx context.Context, empresaID int64) (int64, error) {
	var count int64
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*)
         FROM empleados.empleados_documentos d
         JOIN empleados.empleados_empleado e ON e.num_empleado=d.empleado_id
         WHERE e.empresa_id=$1
           AND d.acta_url IS NOT NULL AND d.ine_url IS NOT NULL
           AND d.comprobante_domicilio_url IS NOT NULL AND d.curp_pdf_url IS NOT NULL
           AND d.nss_pdf_url IS NOT NULL AND d.contrato_url IS NOT NULL`, empresaID).
		Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("expedienteRepository.KPIDocumentosComplete: %w", err)
	}
	return count, nil
}

// KPIDocumentosLack returns the count of employees missing contrato_url for an empresa.
func (r *ExpedienteRepository) KPIDocumentosLack(ctx context.Context, empresaID int64) (int64, error) {
	var count int64
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*)
         FROM empleados.empleados_documentos d
         JOIN empleados.empleados_empleado e ON e.num_empleado=d.empleado_id
         WHERE e.empresa_id=$1 AND d.contrato_url IS NULL`, empresaID).
		Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("expedienteRepository.KPIDocumentosLack: %w", err)
	}
	return count, nil
}

// KPIDocumentosPendiente returns the count of employees with estatus_validacion=1 (Pending) for an empresa.
func (r *ExpedienteRepository) KPIDocumentosPendiente(ctx context.Context, empresaID int64) (int64, error) {
	var count int64
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*)
         FROM empleados.empleados_documentos d
         JOIN empleados.empleados_empleado e ON e.num_empleado=d.empleado_id
         WHERE e.empresa_id=$1 AND d.estatus_validacion=$2`, empresaID, empleados.ObservacionPendiente).
		Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("expedienteRepository.KPIDocumentosPendiente: %w", err)
	}
	return count, nil
}

func mapUbicacionUpsertError(err error) error {
	mapped := mapUbicacionUniqueError(err)
	if errors.Is(mapped, empleados.ErrDuplicateCURP) || errors.Is(mapped, empleados.ErrDuplicateNSS) {
		return mapped
	}
	return fmt.Errorf("expedienteRepository.UpsertUbicacion: %w", err)
}
