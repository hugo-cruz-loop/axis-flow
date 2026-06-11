package service

import (
	"context"
	"time"

	identity "axis-flow-back/internal/domain"
	"axis-flow-back/internal/parametrizacion"
)

type Repositories struct {
	Sistema            parametrizacion.SistemaRepository
	EvaluacionServicio parametrizacion.EvaluacionServicioRepository
	EvaluacionPersonal parametrizacion.EvaluacionPersonalRepository
	DiasInactivos      parametrizacion.DiasInactivosRepository
}

type UnitOfWork interface {
	WithinTx(context.Context, func(context.Context, Repositories) error) error
}
type AuditAppender interface {
	Append(context.Context, identity.AuditEntry) error
}

type ApplicationService struct {
	uow       UnitOfWork
	cache     parametrizacion.CachePort
	publisher parametrizacion.EventPublisher
	audit     AuditAppender
	clock     func() time.Time
}

func NewApplicationService(uow UnitOfWork, cache parametrizacion.CachePort, publisher parametrizacion.EventPublisher, audit AuditAppender, clock func() time.Time) *ApplicationService {
	if clock == nil {
		clock = time.Now
	}
	return &ApplicationService{uow: uow, cache: cache, publisher: publisher, audit: audit, clock: clock}
}

func (s *ApplicationService) ConfigurarEvaluacionServicio(ctx context.Context, cfg parametrizacion.EvaluacionServicio) (*parametrizacion.EvaluacionServicio, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	var saved *parametrizacion.EvaluacionServicio
	if err := s.uow.WithinTx(ctx, func(ctx context.Context, repos Repositories) error {
		c := cfg
		if err := repos.EvaluacionServicio.Upsert(ctx, &c); err != nil {
			return err
		}
		saved = &c
		return nil
	}); err != nil {
		return nil, err
	}
	s.delete(ctx, parametrizacion.EvaluacionServicioCacheKey(saved.EmpresaID, saved.ServicioID))
	s.publish(ctx, parametrizacion.EventEvaluacionServicioConfigurada, map[string]any{"id": saved.ID, "empresa_id": saved.EmpresaID, "servicio_id": saved.ServicioID, "periodicidad_id": saved.PeriodicidadID, "activa": saved.Activa})
	return saved, nil
}
func (s *ApplicationService) ListEvaluacionServicio(ctx context.Context, empresaID int64) ([]*parametrizacion.EvaluacionServicio, error) {
	var out []*parametrizacion.EvaluacionServicio
	err := s.uow.WithinTx(ctx, func(ctx context.Context, repos Repositories) error {
		var err error
		out, err = repos.EvaluacionServicio.ListByEmpresa(ctx, empresaID)
		return err
	})
	return out, err
}
func (s *ApplicationService) ConfigurarEvaluacionPersonal(ctx context.Context, cfg parametrizacion.EvaluacionPersonal) (*parametrizacion.EvaluacionPersonal, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	var saved *parametrizacion.EvaluacionPersonal
	if err := s.uow.WithinTx(ctx, func(ctx context.Context, repos Repositories) error {
		c := cfg
		if err := repos.EvaluacionPersonal.Upsert(ctx, &c); err != nil {
			return err
		}
		saved = &c
		return nil
	}); err != nil {
		return nil, err
	}
	s.delete(ctx, parametrizacion.EvaluacionPersonalCacheKey(saved.EmpresaID))
	return saved, nil
}
func (s *ApplicationService) ListEvaluacionPersonal(ctx context.Context, empresaID int64) ([]*parametrizacion.EvaluacionPersonal, error) {
	var item *parametrizacion.EvaluacionPersonal
	err := s.uow.WithinTx(ctx, func(ctx context.Context, repos Repositories) error {
		var err error
		item, err = repos.EvaluacionPersonal.GetByEmpresa(ctx, empresaID)
		if err == parametrizacion.ErrNotFound {
			item = nil
			return nil
		}
		return err
	})
	if item == nil {
		return []*parametrizacion.EvaluacionPersonal{}, err
	}
	return []*parametrizacion.EvaluacionPersonal{item}, err
}
func (s *ApplicationService) AgregarDiaInactivo(ctx context.Context, dia parametrizacion.DiaInactivo) (*parametrizacion.DiaInactivo, error) {
	if err := dia.Validate(); err != nil {
		return nil, err
	}
	var saved *parametrizacion.DiaInactivo
	if err := s.uow.WithinTx(ctx, func(ctx context.Context, repos Repositories) error {
		d := dia
		if err := repos.DiasInactivos.AddDia(ctx, &d); err != nil {
			return err
		}
		saved = &d
		return nil
	}); err != nil {
		return nil, err
	}
	s.delete(ctx, parametrizacion.DiasInactivosCacheKey(saved.EmpresaID, saved.Fecha.Year()))
	s.publish(ctx, parametrizacion.EventDiaInactivoConfigurado, map[string]any{"id": saved.ID, "empresa_id": saved.EmpresaID, "fecha": saved.Fecha.Format("2006-01-02"), "descripcion": saved.Descripcion, "action": "ADDED"})
	return saved, nil
}
func (s *ApplicationService) EliminarDiaInactivo(ctx context.Context, id, empresaID int64) error {
	year := s.clock().Year()
	if err := s.uow.WithinTx(ctx, func(ctx context.Context, repos Repositories) error {
		return repos.DiasInactivos.DeleteDia(ctx, id, empresaID)
	}); err != nil {
		return err
	}
	s.delete(ctx, parametrizacion.DiasInactivosCacheKey(empresaID, year))
	s.publish(ctx, parametrizacion.EventDiaInactivoConfigurado, map[string]any{"id": id, "empresa_id": empresaID, "action": "DELETED"})
	return nil
}
func (s *ApplicationService) ConfigurarUmbralDiasInactivos(ctx context.Context, u parametrizacion.DiasInactivosUmbral) (*parametrizacion.DiasInactivosUmbral, error) {
	if err := u.Validate(); err != nil {
		return nil, err
	}
	var saved *parametrizacion.DiasInactivosUmbral
	if err := s.uow.WithinTx(ctx, func(ctx context.Context, repos Repositories) error {
		v := u
		if err := repos.DiasInactivos.UpsertUmbral(ctx, &v); err != nil {
			return err
		}
		saved = &v
		return nil
	}); err != nil {
		return nil, err
	}
	s.delete(ctx, parametrizacion.DiasInactivosCacheKey(saved.EmpresaID, s.clock().Year()))
	return saved, nil
}
func (s *ApplicationService) GetDiasInactivosEmpresa(ctx context.Context, empresaID int64, year int) (*parametrizacion.CompanyInactiveDays, error) {
	key := parametrizacion.DiasInactivosCacheKey(empresaID, year)
	var cached parametrizacion.CompanyInactiveDays
	if s.cache != nil {
		if hit, err := s.cache.Get(ctx, key, &cached); err != nil {
			return nil, err
		} else if hit {
			return &cached, nil
		}
	}
	var out *parametrizacion.CompanyInactiveDays
	if err := s.uow.WithinTx(ctx, func(ctx context.Context, repos Repositories) error {
		days, err := repos.DiasInactivos.ListByEmpresaYear(ctx, empresaID, year)
		if err != nil {
			return err
		}
		umbral, err := repos.DiasInactivos.GetUmbral(ctx, empresaID)
		if err != nil {
			return err
		}
		out = &parametrizacion.CompanyInactiveDays{EmpresaID: empresaID, UmbralDias: umbral.UmbralDias, DiasInactivos: days}
		return nil
	}); err != nil {
		return nil, err
	}
	if s.cache != nil {
		_ = s.cache.Set(ctx, key, out, parametrizacion.CacheTTL)
	}
	return out, nil
}
func (s *ApplicationService) UpsertSistemaParametro(ctx context.Context, p parametrizacion.SistemaParametro, actor parametrizacion.Actor) (*parametrizacion.SistemaParametro, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	var saved *parametrizacion.SistemaParametro
	if err := s.uow.WithinTx(ctx, func(ctx context.Context, repos Repositories) error {
		v := p
		if err := repos.Sistema.Upsert(ctx, &v); err != nil {
			return err
		}
		saved = &v
		if s.audit != nil {
			actorID := actor.UserID
			return s.audit.Append(ctx, identity.AuditEntry{ActorUserID: &actorID, Action: "PARAMETRIZACION_SISTEMA_UPDATED", IPAddress: actor.IPAddress, TraceID: actor.TraceID, Metadata: map[string]any{"clave_parametro": v.ClaveParametro}})
		}
		return nil
	}); err != nil {
		return nil, err
	}
	s.delete(ctx, parametrizacion.SistemaCacheKey(saved.ClaveParametro))
	s.publish(ctx, parametrizacion.EventSystemSettingUpdated, map[string]any{"clave_parametro": saved.ClaveParametro, "valor": saved.Valor, "updated_by": actor.UserID.String()})
	return saved, nil
}
func (s *ApplicationService) GetSistemaParametro(ctx context.Context, clave string) (*parametrizacion.SistemaParametro, error) {
	key := parametrizacion.SistemaCacheKey(clave)
	var cached parametrizacion.SistemaParametro
	if s.cache != nil {
		if hit, err := s.cache.Get(ctx, key, &cached); err != nil {
			return nil, err
		} else if hit {
			return &cached, nil
		}
	}
	var out *parametrizacion.SistemaParametro
	if err := s.uow.WithinTx(ctx, func(ctx context.Context, repos Repositories) error {
		var err error
		out, err = repos.Sistema.GetByClave(ctx, clave)
		return err
	}); err != nil {
		return nil, err
	}
	if s.cache != nil {
		_ = s.cache.Set(ctx, key, out, parametrizacion.SystemSettingCacheTTL)
	}
	return out, nil
}
func (s *ApplicationService) ListSistemaParametros(ctx context.Context) ([]*parametrizacion.SistemaParametro, error) {
	var out []*parametrizacion.SistemaParametro
	err := s.uow.WithinTx(ctx, func(ctx context.Context, repos Repositories) error {
		var err error
		out, err = repos.Sistema.List(ctx)
		return err
	})
	return out, err
}
func (s *ApplicationService) delete(ctx context.Context, keys ...string) {
	if s.cache != nil {
		_ = s.cache.Delete(ctx, keys...)
	}
}
func (s *ApplicationService) publish(ctx context.Context, name string, payload any) {
	if s.publisher != nil {
		_ = s.publisher.Publish(ctx, parametrizacion.DomainEvent{Name: name, OccurredAt: s.clock().UTC(), Payload: payload})
	}
}
