// Package service implements business logic for the Clientes module.
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"axis-flow-back/internal/clientes"
	clicache "axis-flow-back/internal/clientes/cache"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const clienteCacheTTL = 86400 * time.Second

// ClienteRepository is the subset of repository.ClienteRepository used by the service layer.
type ClienteRepository interface {
	Create(ctx context.Context, c *clientes.Cliente) error
	FindByID(ctx context.Context, id uuid.UUID, empresaID int64) (*clientes.Cliente, error)
	FindByUserID(ctx context.Context, userID uuid.UUID, empresaID int64) (*clientes.Cliente, error)
	ListByEmpresa(ctx context.Context, empresaID int64, page, size int) ([]clientes.Cliente, int, error)
	Update(ctx context.Context, c *clientes.Cliente) error
	Delete(ctx context.Context, id uuid.UUID, empresaID int64) error
	CheckQualityGate(ctx context.Context, clienteID uuid.UUID) (*clientes.QualityGateStatus, error)
	PatchEstatus(ctx context.Context, clienteID uuid.UUID, empresaID int64, estatus int) (int, *clientes.QualityGateStatus, error)
}

// ClienteServicer defines the business operations for Cliente.
type ClienteServicer interface {
	CreateCliente(ctx context.Context, req CreateClienteRequest) (*clientes.Cliente, error)
	GetCliente(ctx context.Context, id uuid.UUID, empresaID int64) (*clientes.Cliente, error)
	ListClientes(ctx context.Context, empresaID int64, page, size int) ([]clientes.Cliente, int, error)
	UpdateCliente(ctx context.Context, id uuid.UUID, empresaID int64, req UpdateClienteRequest) (*clientes.Cliente, error)
	DeleteCliente(ctx context.Context, id uuid.UUID, empresaID int64) error
	PatchEstatus(ctx context.Context, id uuid.UUID, empresaID int64, estatus int) (httpStatus int, c *clientes.Cliente, gate *clientes.QualityGateStatus, err error)
	GetClienteByUser(ctx context.Context, userID uuid.UUID, empresaID int64) (*clientes.Cliente, error)
	GetStats(ctx context.Context, empresaID int64) (active, inactive int, err error)
}

// CreateClienteRequest holds the fields for creating a new Cliente.
type CreateClienteRequest struct {
	EmpresaID           int64
	RepresentanteID     uuid.UUID
	NombreComercial     string
	RazonSocial         string
	FechaInicioContrato time.Time
}

// UpdateClienteRequest holds optional fields for updating a Cliente.
type UpdateClienteRequest struct {
	NombreComercial     *string
	RazonSocial         *string
	FechaInicioContrato *time.Time
}

// ClienteService implements ClienteServicer.
type ClienteService struct {
	repo ClienteRepository
	rdb  *redis.Client
}

// NewClienteService constructs a ClienteService.
func NewClienteService(repo ClienteRepository, rdb *redis.Client) *ClienteService {
	return &ClienteService{repo: repo, rdb: rdb}
}

// CreateCliente inserts a new Cliente and invalidates any stale cache entry.
func (s *ClienteService) CreateCliente(ctx context.Context, req CreateClienteRequest) (*clientes.Cliente, error) {
	var fechaPtr *time.Time
	if !req.FechaInicioContrato.IsZero() {
		t := req.FechaInicioContrato
		fechaPtr = &t
	}

	c := &clientes.Cliente{
		EmpresaID:           req.EmpresaID,
		RepresentanteID:     req.RepresentanteID,
		NombreComercial:     req.NombreComercial,
		RazonSocial:         req.RazonSocial,
		FechaInicioContrato: fechaPtr,
		Estatus:             clientes.ClienteStatusIncompleto,
	}

	if err := s.repo.Create(ctx, c); err != nil {
		return nil, fmt.Errorf("clienteService.CreateCliente: %w", err)
	}

	// Fail-open: cache invalidation errors are not fatal
	_ = clicache.InvalidateCliente(ctx, s.rdb, c.EmpresaID, c.ID)

	return c, nil
}

// GetCliente retrieves a Cliente by ID using cache-aside (fail-open on Redis errors).
func (s *ClienteService) GetCliente(ctx context.Context, id uuid.UUID, empresaID int64) (*clientes.Cliente, error) {
	key := clicache.ClienteCacheKey(empresaID, id)

	raw, err := s.rdb.Get(ctx, key).Bytes()
	if err == nil {
		var c clientes.Cliente
		if jsonErr := json.Unmarshal(raw, &c); jsonErr == nil {
			return &c, nil
		}
	}

	c, err := s.repo.FindByID(ctx, id, empresaID)
	if err != nil {
		return nil, err
	}

	if data, jsonErr := json.Marshal(c); jsonErr == nil {
		_ = s.rdb.Set(ctx, key, data, clienteCacheTTL).Err()
	}

	return c, nil
}

// ListClientes returns a paginated list of clientes for an empresa.
func (s *ClienteService) ListClientes(ctx context.Context, empresaID int64, page, size int) ([]clientes.Cliente, int, error) {
	return s.repo.ListByEmpresa(ctx, empresaID, page, size)
}

// UpdateCliente applies partial updates to a Cliente and invalidates the cache.
func (s *ClienteService) UpdateCliente(ctx context.Context, id uuid.UUID, empresaID int64, req UpdateClienteRequest) (*clientes.Cliente, error) {
	c, err := s.repo.FindByID(ctx, id, empresaID)
	if err != nil {
		return nil, err
	}
	if c.EmpresaID != empresaID {
		return nil, clientes.ErrTenantMismatch
	}

	if req.NombreComercial != nil {
		c.NombreComercial = *req.NombreComercial
	}
	if req.RazonSocial != nil {
		c.RazonSocial = *req.RazonSocial
	}
	if req.FechaInicioContrato != nil {
		c.FechaInicioContrato = req.FechaInicioContrato
	}

	if err := s.repo.Update(ctx, c); err != nil {
		return nil, fmt.Errorf("clienteService.UpdateCliente: %w", err)
	}

	_ = clicache.InvalidateCliente(ctx, s.rdb, c.EmpresaID, c.ID)

	return c, nil
}

// DeleteCliente removes a Cliente and invalidates the cache.
func (s *ClienteService) DeleteCliente(ctx context.Context, id uuid.UUID, empresaID int64) error {
	if err := s.repo.Delete(ctx, id, empresaID); err != nil {
		return fmt.Errorf("clienteService.DeleteCliente: %w", err)
	}
	_ = clicache.InvalidateCliente(ctx, s.rdb, empresaID, id)
	return nil
}

// PatchEstatus attempts to change a cliente's status.
// Returns (206, nil, gateStatus, nil) if quality gate blocks activation.
// Returns (200, cliente, nil, nil) on success — also invalidates cache.
func (s *ClienteService) PatchEstatus(ctx context.Context, id uuid.UUID, empresaID int64, estatus int) (int, *clientes.Cliente, *clientes.QualityGateStatus, error) {
	httpStatus, gate, err := s.repo.PatchEstatus(ctx, id, empresaID, estatus)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("clienteService.PatchEstatus: %w", err)
	}

	if httpStatus == 206 {
		return 206, nil, gate, nil
	}

	// 200: invalidate cache, then fetch updated record
	_ = clicache.InvalidateCliente(ctx, s.rdb, empresaID, id)
	_ = s.rdb.Unlink(ctx, clicache.ActivationFlagKey(id)).Err()

	c, err := s.repo.FindByID(ctx, id, empresaID)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("clienteService.PatchEstatus fetch: %w", err)
	}

	return 200, c, nil, nil
}

// GetClienteByUser returns the Cliente whose representante_id matches userID.
func (s *ClienteService) GetClienteByUser(ctx context.Context, userID uuid.UUID, empresaID int64) (*clientes.Cliente, error) {
	return s.repo.FindByUserID(ctx, userID, empresaID)
}

// GetStats returns the count of active and inactive clientes for an empresa.
func (s *ClienteService) GetStats(ctx context.Context, empresaID int64) (int, int, error) {
	all, total, err := s.repo.ListByEmpresa(ctx, empresaID, 1, int(^uint(0)>>1))
	if err != nil {
		return 0, 0, fmt.Errorf("clienteService.GetStats: %w", err)
	}
	_ = total
	active := 0
	for _, c := range all {
		if c.Estatus == clientes.ClienteStatusActivo {
			active++
		}
	}
	return active, len(all) - active, nil
}
