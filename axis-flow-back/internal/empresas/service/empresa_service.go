package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"axis-flow-back/internal/empresas"

	"github.com/redis/go-redis/v9"
)

const empresaCacheTTL = 24 * time.Hour

// EmpresaFullRepo is the full interface needed by EmpresaService.
type EmpresaFullRepo interface {
	EmpresaCreatorRepo
	FindByID(ctx context.Context, id int64) (*empresas.Empresa, error)
	Update(ctx context.Context, e *empresas.Empresa) error
	Delete(ctx context.Context, id int64) error
}

// EmpresaService handles empresa CRUD with Redis cache-aside.
type EmpresaService struct {
	repo EmpresaFullRepo
	rdb  *redis.Client
}

// NewEmpresaService creates a new EmpresaService.
func NewEmpresaService(repo EmpresaFullRepo, rdb *redis.Client) *EmpresaService {
	return &EmpresaService{repo: repo, rdb: rdb}
}

func empresaCacheKey(id int64) string {
	return fmt.Sprintf("empresas:empresa:%d:status", id)
}

// GetByID returns an empresa by ID, using Redis cache-aside.
func (s *EmpresaService) GetByID(ctx context.Context, id int64) (*empresas.Empresa, error) {
	key := empresaCacheKey(id)

	raw, err := s.rdb.Get(ctx, key).Bytes()
	if err == nil {
		var e empresas.Empresa
		if jsonErr := json.Unmarshal(raw, &e); jsonErr == nil {
			return &e, nil
		}
	}

	e, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if data, jsonErr := json.Marshal(e); jsonErr == nil {
		_ = s.rdb.Set(ctx, key, data, empresaCacheTTL).Err()
	}

	return e, nil
}

// Update persists changes to an empresa and invalidates the cache.
func (s *EmpresaService) Update(ctx context.Context, e *empresas.Empresa) error {
	if err := s.repo.Update(ctx, e); err != nil {
		return fmt.Errorf("empresa_service.Update: %w", err)
	}
	_ = s.rdb.Unlink(ctx, empresaCacheKey(e.ID)).Err()
	return nil
}

// Delete removes an empresa (cascade via repo) and invalidates the cache.
func (s *EmpresaService) Delete(ctx context.Context, id int64) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("empresa_service.Delete: %w", err)
	}
	_ = s.rdb.Del(ctx, empresaCacheKey(id)).Err()
	return nil
}
