package service_test

import (
	"axis-flow-back/internal/formularios/repository"
	"axis-flow-back/internal/formularios/service"
)

// Compile-time port satisfaction: the concrete Redis invalidator from
// PR-2 must satisfy the service-layer CacheInvalidator port. If a method
// signature drifts in either file, this test fails to compile.
var _ service.CacheInvalidator = (*repository.RedisFormulariosCacheInvalidator)(nil)
