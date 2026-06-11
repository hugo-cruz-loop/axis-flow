package service_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	identity "axis-flow-back/internal/domain"
	"axis-flow-back/internal/parametrizacion"
	"axis-flow-back/internal/parametrizacion/service"

	"github.com/google/uuid"
)

func TestConfigurarEvaluacionServicioCommitsBeforeCacheEvictionAndEvent(t *testing.T) {
	clock := fixedClock()
	fake := newFakeUOW(clock)
	cache := &recordingCache{order: &fake.order}
	publisher := &recordingPublisher{order: &fake.order}
	svc := service.NewApplicationService(fake, cache, publisher, nil, clock)

	got, err := svc.ConfigurarEvaluacionServicio(context.Background(), parametrizacion.EvaluacionServicio{EmpresaID: 12, ServicioID: 5, PeriodicidadID: 2, Activa: true})
	if err != nil {
		t.Fatalf("ConfigurarEvaluacionServicio returned error: %v", err)
	}
	if got.ID != 101 || got.EmpresaID != 12 || got.ServicioID != 5 || got.PeriodicidadID != 2 || !got.Activa {
		t.Fatalf("unexpected persisted config: %#v", got)
	}
	wantOrder := []string{"begin", "evaluacion_servicio.upsert", "commit", "cache.delete:parametrizacion:evaluacion_servicio:empresa:12:servicio:5", "event:EvaluacionServicioConfigurada"}
	if !reflect.DeepEqual(fake.order, wantOrder) {
		t.Fatalf("order mismatch\ngot:  %#v\nwant: %#v", fake.order, wantOrder)
	}
	if len(publisher.events) != 1 || publisher.events[0].Name != parametrizacion.EventEvaluacionServicioConfigurada {
		t.Fatalf("expected EvaluacionServicioConfigurada event, got %#v", publisher.events)
	}
}

func TestAgregarDiaInactivoEvictsYearKeyAndPublishesConfiguredEvent(t *testing.T) {
	clock := fixedClock()
	fake := newFakeUOW(clock)
	cache := &recordingCache{order: &fake.order}
	publisher := &recordingPublisher{order: &fake.order}
	svc := service.NewApplicationService(fake, cache, publisher, nil, clock)

	got, err := svc.AgregarDiaInactivo(context.Background(), parametrizacion.DiaInactivo{EmpresaID: 12, Fecha: time.Date(2026, 12, 25, 0, 0, 0, 0, time.UTC), Descripcion: "Navidad"})
	if err != nil {
		t.Fatalf("AgregarDiaInactivo returned error: %v", err)
	}
	if got.ID != 154 || got.Fecha.Year() != 2026 {
		t.Fatalf("unexpected inactive day: %#v", got)
	}
	wantOrder := []string{"begin", "dias.add", "commit", "cache.delete:parametrizacion:dias_inactivos:empresa:12:year:2026", "event:DiaInactivoConfigurado"}
	if !reflect.DeepEqual(fake.order, wantOrder) {
		t.Fatalf("order mismatch\ngot:  %#v\nwant: %#v", fake.order, wantOrder)
	}
}

func TestUpsertSistemaParametroAuditsEveryUpdateAndEvictsSystemCache(t *testing.T) {
	clock := fixedClock()
	fake := newFakeUOW(clock)
	cache := &recordingCache{order: &fake.order}
	publisher := &recordingPublisher{order: &fake.order}
	audit := &recordingAudit{order: &fake.order}
	svc := service.NewApplicationService(fake, cache, publisher, audit, clock)

	actorID := uuid.MustParse("00000000-0000-0000-0000-000000000012")
	got, err := svc.UpsertSistemaParametro(context.Background(), parametrizacion.SistemaParametro{ClaveParametro: "WEBSOCKET_CHANNEL_NAME", Valor: "axis_flow"}, parametrizacion.Actor{UserID: actorID, IPAddress: "127.0.0.1", TraceID: "trace-1"})
	if err != nil {
		t.Fatalf("UpsertSistemaParametro returned error: %v", err)
	}
	if got.ClaveParametro != "WEBSOCKET_CHANNEL_NAME" || got.Valor != "axis_flow" {
		t.Fatalf("unexpected setting: %#v", got)
	}
	wantOrder := []string{"begin", "sistema.upsert", "audit:PARAMETRIZACION_SISTEMA_UPDATED", "commit", "cache.delete:parametrizacion:sistema:WEBSOCKET_CHANNEL_NAME", "event:SystemSettingUpdated"}
	if !reflect.DeepEqual(fake.order, wantOrder) {
		t.Fatalf("order mismatch\ngot:  %#v\nwant: %#v", fake.order, wantOrder)
	}
	if len(audit.entries) != 1 {
		t.Fatalf("expected one audit entry, got %#v", audit.entries)
	}
	entry := audit.entries[0]
	if entry.ActorUserID == nil || *entry.ActorUserID != actorID || entry.Action != "PARAMETRIZACION_SISTEMA_UPDATED" {
		t.Fatalf("audit actor/action mismatch: %#v", entry)
	}
	if entry.Metadata["clave_parametro"] != "WEBSOCKET_CHANNEL_NAME" {
		t.Fatalf("audit metadata must include clave_parametro, got %#v", entry.Metadata)
	}
}

func TestGetDiasInactivosEmpresaUsesCacheAside(t *testing.T) {
	clock := fixedClock()
	fake := newFakeUOW(clock)
	cache := &recordingCache{order: &fake.order}
	svc := service.NewApplicationService(fake, cache, &recordingPublisher{order: &fake.order}, nil, clock)

	first, err := svc.GetDiasInactivosEmpresa(context.Background(), 12, 2026)
	if err != nil {
		t.Fatalf("first GetDiasInactivosEmpresa returned error: %v", err)
	}
	if len(first.DiasInactivos) != 1 || first.UmbralDias != 5 {
		t.Fatalf("unexpected first response: %#v", first)
	}
	if cache.setKey != "parametrizacion:dias_inactivos:empresa:12:year:2026" || cache.setTTL != parametrizacion.CacheTTL {
		t.Fatalf("cache set mismatch: key=%q ttl=%s", cache.setKey, cache.setTTL)
	}

	cache.hit = true
	cache.cachedCompanyDays = &parametrizacion.CompanyInactiveDays{EmpresaID: 12, UmbralDias: 9}
	fake.order = nil
	second, err := svc.GetDiasInactivosEmpresa(context.Background(), 12, 2026)
	if err != nil {
		t.Fatalf("cached GetDiasInactivosEmpresa returned error: %v", err)
	}
	if second.UmbralDias != 9 || len(fake.order) != 0 {
		t.Fatalf("cache hit must bypass repository, got response=%#v order=%#v", second, fake.order)
	}
}

// fakes

type fakeUOW struct {
	repos fakeRepositories
	order []string
}

func newFakeUOW(clock func() time.Time) *fakeUOW {
	f := &fakeUOW{}
	f.repos = fakeRepositories{
		sistema:      &fakeSistemaRepo{order: &f.order, clock: clock},
		evalServicio: &fakeEvaluacionServicioRepo{order: &f.order, clock: clock},
		evalPersonal: &fakeEvaluacionPersonalRepo{order: &f.order, clock: clock},
		dias:         &fakeDiasRepo{order: &f.order, clock: clock},
	}
	return f
}

func (f *fakeUOW) WithinTx(ctx context.Context, fn func(context.Context, service.Repositories) error) error {
	f.order = append(f.order, "begin")
	if err := fn(ctx, service.Repositories{Sistema: f.repos.sistema, EvaluacionServicio: f.repos.evalServicio, EvaluacionPersonal: f.repos.evalPersonal, DiasInactivos: f.repos.dias}); err != nil {
		f.order = append(f.order, "rollback")
		return err
	}
	f.order = append(f.order, "commit")
	return nil
}

type fakeRepositories struct {
	sistema      *fakeSistemaRepo
	evalServicio *fakeEvaluacionServicioRepo
	evalPersonal *fakeEvaluacionPersonalRepo
	dias         *fakeDiasRepo
}

type fakeSistemaRepo struct {
	order *[]string
	clock func() time.Time
}

func (r *fakeSistemaRepo) Upsert(ctx context.Context, p *parametrizacion.SistemaParametro) error {
	*r.order = append(*r.order, "sistema.upsert")
	p.CreatedAt = r.clock()
	p.UpdatedAt = r.clock()
	return nil
}
func (r *fakeSistemaRepo) GetByClave(ctx context.Context, clave string) (*parametrizacion.SistemaParametro, error) {
	*r.order = append(*r.order, "sistema.get")
	return &parametrizacion.SistemaParametro{ClaveParametro: clave, Valor: "axis_flow", CreatedAt: r.clock(), UpdatedAt: r.clock()}, nil
}
func (r *fakeSistemaRepo) List(ctx context.Context) ([]*parametrizacion.SistemaParametro, error) {
	*r.order = append(*r.order, "sistema.list")
	return []*parametrizacion.SistemaParametro{{ClaveParametro: "WEBSOCKET_CHANNEL_NAME", Valor: "axis_flow"}}, nil
}

type fakeEvaluacionServicioRepo struct {
	order *[]string
	clock func() time.Time
}

func (r *fakeEvaluacionServicioRepo) Upsert(ctx context.Context, cfg *parametrizacion.EvaluacionServicio) error {
	*r.order = append(*r.order, "evaluacion_servicio.upsert")
	cfg.ID = 101
	cfg.CreatedAt = r.clock()
	cfg.UpdatedAt = r.clock()
	return nil
}
func (r *fakeEvaluacionServicioRepo) GetByEmpresaServicio(ctx context.Context, empresaID, servicioID int64) (*parametrizacion.EvaluacionServicio, error) {
	*r.order = append(*r.order, "evaluacion_servicio.get")
	return &parametrizacion.EvaluacionServicio{ID: 101, EmpresaID: empresaID, ServicioID: servicioID, PeriodicidadID: 2, Activa: true, CreatedAt: r.clock(), UpdatedAt: r.clock()}, nil
}
func (r *fakeEvaluacionServicioRepo) ListByEmpresa(ctx context.Context, empresaID int64) ([]*parametrizacion.EvaluacionServicio, error) {
	*r.order = append(*r.order, "evaluacion_servicio.list")
	return []*parametrizacion.EvaluacionServicio{{ID: 101, EmpresaID: empresaID, ServicioID: 5, PeriodicidadID: 2, Activa: true}}, nil
}

type fakeEvaluacionPersonalRepo struct {
	order *[]string
	clock func() time.Time
}

func (r *fakeEvaluacionPersonalRepo) Upsert(ctx context.Context, cfg *parametrizacion.EvaluacionPersonal) error {
	*r.order = append(*r.order, "evaluacion_personal.upsert")
	cfg.ID = 12
	cfg.CreatedAt = r.clock()
	cfg.UpdatedAt = r.clock()
	return nil
}
func (r *fakeEvaluacionPersonalRepo) GetByEmpresa(ctx context.Context, empresaID int64) (*parametrizacion.EvaluacionPersonal, error) {
	*r.order = append(*r.order, "evaluacion_personal.get")
	return &parametrizacion.EvaluacionPersonal{ID: 12, EmpresaID: empresaID, PeriodicidadID: 2, Activa: true}, nil
}

type fakeDiasRepo struct {
	order *[]string
	clock func() time.Time
}

func (r *fakeDiasRepo) AddDia(ctx context.Context, d *parametrizacion.DiaInactivo) error {
	*r.order = append(*r.order, "dias.add")
	d.ID = 154
	d.CreatedAt = r.clock()
	d.UpdatedAt = r.clock()
	return nil
}
func (r *fakeDiasRepo) DeleteDia(ctx context.Context, id, empresaID int64) error {
	*r.order = append(*r.order, "dias.delete")
	return nil
}
func (r *fakeDiasRepo) ListByEmpresaYear(ctx context.Context, empresaID int64, year int) ([]*parametrizacion.DiaInactivo, error) {
	*r.order = append(*r.order, "dias.list")
	return []*parametrizacion.DiaInactivo{{ID: 154, EmpresaID: empresaID, Fecha: time.Date(year, 12, 25, 0, 0, 0, 0, time.UTC), Descripcion: "Navidad"}}, nil
}
func (r *fakeDiasRepo) UpsertUmbral(ctx context.Context, u *parametrizacion.DiasInactivosUmbral) error {
	*r.order = append(*r.order, "dias.umbral.upsert")
	u.UpdatedAt = r.clock()
	return nil
}
func (r *fakeDiasRepo) GetUmbral(ctx context.Context, empresaID int64) (*parametrizacion.DiasInactivosUmbral, error) {
	*r.order = append(*r.order, "dias.umbral.get")
	return &parametrizacion.DiasInactivosUmbral{EmpresaID: empresaID, UmbralDias: 5}, nil
}

type recordingCache struct {
	order             *[]string
	hit               bool
	cachedCompanyDays *parametrizacion.CompanyInactiveDays
	setKey            string
	setTTL            time.Duration
}

func (c *recordingCache) Get(ctx context.Context, key string, dest any) (bool, error) {
	if !c.hit {
		return false, nil
	}
	if v, ok := dest.(*parametrizacion.CompanyInactiveDays); ok && c.cachedCompanyDays != nil {
		*v = *c.cachedCompanyDays
	}
	return true, nil
}
func (c *recordingCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	c.setKey = key
	c.setTTL = ttl
	return nil
}
func (c *recordingCache) Delete(ctx context.Context, keys ...string) error {
	if c.order != nil {
		for _, key := range keys {
			*c.order = append(*c.order, "cache.delete:"+key)
		}
	}
	return nil
}

type recordingPublisher struct {
	order  *[]string
	events []parametrizacion.DomainEvent
}

func (p *recordingPublisher) Publish(ctx context.Context, event parametrizacion.DomainEvent) error {
	p.events = append(p.events, event)
	if p.order != nil {
		*p.order = append(*p.order, "event:"+event.Name)
	}
	return nil
}

type recordingAudit struct {
	order   *[]string
	entries []identity.AuditEntry
}

func (a *recordingAudit) Append(ctx context.Context, entry identity.AuditEntry) error {
	a.entries = append(a.entries, entry)
	if a.order != nil {
		*a.order = append(*a.order, "audit:"+entry.Action)
	}
	return nil
}

func fixedClock() func() time.Time {
	return func() time.Time { return time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC) }
}
