// Package cache provides Redis cache helpers for the Clientes module.
// It reuses the generic Get/Set/Del helpers from the catalogos cache package.
package cache

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// ClienteCacheKey returns the cache key for a cliente record scoped to an empresa.
// Format: "clientes:{empresaID}:{clienteID}"
func ClienteCacheKey(empresaID int64, clienteID uuid.UUID) string {
	return fmt.Sprintf("clientes:%d:%s", empresaID, clienteID)
}

// LocalidadCacheKey returns the cache key for a localidad record.
// Format: "clientes:localidad:{localidadID}"
func LocalidadCacheKey(localidadID uuid.UUID) string {
	return fmt.Sprintf("clientes:localidad:%s", localidadID)
}

// ActivationFlagKey returns the cache key used to flag an in-progress activation.
// Format: "clientes:activation:{clienteID}"
func ActivationFlagKey(clienteID uuid.UUID) string {
	return fmt.Sprintf("clientes:activation:%s", clienteID)
}

// InvalidateCliente removes the cached cliente entry from Redis using Unlink (async, non-blocking).
func InvalidateCliente(ctx context.Context, rdb *redis.Client, empresaID int64, clienteID uuid.UUID) error {
	key := ClienteCacheKey(empresaID, clienteID)
	if err := rdb.Unlink(ctx, key).Err(); err != nil {
		return fmt.Errorf("clientes/cache.InvalidateCliente: %w", err)
	}
	return nil
}

// InvalidateLocalidad removes the cached localidad entry from Redis using Unlink (async, non-blocking).
func InvalidateLocalidad(ctx context.Context, rdb *redis.Client, localidadID uuid.UUID) error {
	key := LocalidadCacheKey(localidadID)
	if err := rdb.Unlink(ctx, key).Err(); err != nil {
		return fmt.Errorf("clientes/cache.InvalidateLocalidad: %w", err)
	}
	return nil
}
