package parametrizacion_test

import (
	"context"
	"testing"
	"time"

	"axis-flow-back/internal/parametrizacion"
)

func TestCacheKeyBuildersUseSpecPatterns(t *testing.T) {
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"service evaluation", parametrizacion.EvaluacionServicioCacheKey(12, 5), "parametrizacion:evaluacion_servicio:empresa:12:servicio:5"},
		{"personal evaluation", parametrizacion.EvaluacionPersonalCacheKey(12), "parametrizacion:evaluacion_personal:empresa:12"},
		{"inactive days", parametrizacion.DiasInactivosCacheKey(12, 2026), "parametrizacion:dias_inactivos:empresa:12:year:2026"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Fatalf("got %q, want %q", tc.got, tc.want)
			}
		})
	}
}

func TestEventNameConstantsMatchIntegrationContract(t *testing.T) {
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"service evaluation", parametrizacion.EventEvaluacionServicioConfigurada, "EvaluacionServicioConfigurada"},
		{"inactive day", parametrizacion.EventDiaInactivoConfigurado, "DiaInactivoConfigurado"},
		{"system setting", parametrizacion.EventSystemSettingUpdated, "SystemSettingUpdated"},
	}

	seen := map[string]bool{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Fatalf("got %q, want %q", tc.got, tc.want)
			}
			if seen[tc.got] {
				t.Fatalf("duplicate event name %q", tc.got)
			}
			seen[tc.got] = true
		})
	}
}

func TestDomainValidationRejectsInvalidThresholdAndSystemKey(t *testing.T) {
	if err := (parametrizacion.DiasInactivosUmbral{EmpresaID: 12, UmbralDias: -1}).Validate(); err == nil {
		t.Fatal("negative inactive-days threshold must be rejected")
	}
	if err := (parametrizacion.SistemaParametro{ClaveParametro: "channel_name", Valor: "x"}).Validate(); err == nil {
		t.Fatal("lowercase system parameter key must be rejected")
	}

	validDate := time.Date(2026, time.December, 25, 0, 0, 0, 0, time.UTC)
	if err := (parametrizacion.DiaInactivo{EmpresaID: 12, Fecha: validDate, Descripcion: "Christmas"}).Validate(); err != nil {
		t.Fatalf("valid inactive day rejected: %v", err)
	}
}

func TestPortsCompile(t *testing.T) {
	var _ parametrizacion.SistemaRepository = (*sistemaRepositoryStub)(nil)
	var _ parametrizacion.EvaluacionServicioRepository = (*evaluacionServicioRepositoryStub)(nil)
	var _ parametrizacion.EvaluacionPersonalRepository = (*evaluacionPersonalRepositoryStub)(nil)
	var _ parametrizacion.DiasInactivosRepository = (*diasInactivosRepositoryStub)(nil)
	var _ parametrizacion.CachePort = (*cachePortStub)(nil)
	var _ parametrizacion.EventPublisher = (*eventPublisherStub)(nil)
}

type sistemaRepositoryStub struct{}

func (sistemaRepositoryStub) Upsert(ctx context.Context, parametro *parametrizacion.SistemaParametro) error {
	return nil
}
func (sistemaRepositoryStub) GetByClave(ctx context.Context, clave string) (*parametrizacion.SistemaParametro, error) {
	return nil, nil
}
func (sistemaRepositoryStub) List(ctx context.Context) ([]*parametrizacion.SistemaParametro, error) {
	return nil, nil
}

type evaluacionServicioRepositoryStub struct{}

func (evaluacionServicioRepositoryStub) Upsert(ctx context.Context, config *parametrizacion.EvaluacionServicio) error {
	return nil
}
func (evaluacionServicioRepositoryStub) GetByEmpresaServicio(ctx context.Context, empresaID, servicioID int64) (*parametrizacion.EvaluacionServicio, error) {
	return nil, nil
}
func (evaluacionServicioRepositoryStub) ListByEmpresa(ctx context.Context, empresaID int64) ([]*parametrizacion.EvaluacionServicio, error) {
	return nil, nil
}

type evaluacionPersonalRepositoryStub struct{}

func (evaluacionPersonalRepositoryStub) Upsert(ctx context.Context, config *parametrizacion.EvaluacionPersonal) error {
	return nil
}
func (evaluacionPersonalRepositoryStub) GetByEmpresa(ctx context.Context, empresaID int64) (*parametrizacion.EvaluacionPersonal, error) {
	return nil, nil
}

type diasInactivosRepositoryStub struct{}

func (diasInactivosRepositoryStub) AddDia(ctx context.Context, dia *parametrizacion.DiaInactivo) error {
	return nil
}
func (diasInactivosRepositoryStub) DeleteDia(ctx context.Context, id, empresaID int64) error {
	return nil
}
func (diasInactivosRepositoryStub) ListByEmpresaYear(ctx context.Context, empresaID int64, year int) ([]*parametrizacion.DiaInactivo, error) {
	return nil, nil
}
func (diasInactivosRepositoryStub) UpsertUmbral(ctx context.Context, umbral *parametrizacion.DiasInactivosUmbral) error {
	return nil
}
func (diasInactivosRepositoryStub) GetUmbral(ctx context.Context, empresaID int64) (*parametrizacion.DiasInactivosUmbral, error) {
	return nil, nil
}

type cachePortStub struct{}

func (cachePortStub) Get(ctx context.Context, key string, dest any) (bool, error) { return false, nil }
func (cachePortStub) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	return nil
}
func (cachePortStub) Delete(ctx context.Context, keys ...string) error { return nil }

type eventPublisherStub struct{}

func (eventPublisherStub) Publish(ctx context.Context, event parametrizacion.DomainEvent) error {
	return nil
}
