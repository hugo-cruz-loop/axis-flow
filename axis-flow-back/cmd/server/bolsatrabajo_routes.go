package main

import (
	"net/http"
	"os"
	"time"

	"axis-flow-back/internal/bolsatrabajo/events"
	bthandler "axis-flow-back/internal/bolsatrabajo/handler"
	btrepo "axis-flow-back/internal/bolsatrabajo/repository"
	btsvc "axis-flow-back/internal/bolsatrabajo/service"
	"axis-flow-back/internal/bolsatrabajo/storage"
	"axis-flow-back/internal/bolsatrabajo/turnstile"
	"axis-flow-back/internal/config"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// bolsaTrabajoModule holds the constructed handlers for the bolsa de trabajo domain.
type bolsaTrabajoModule struct {
	trabajoH     *bthandler.TrabajoHandler
	postulacionH *bthandler.PostulacionHandler
	evaluacionH  *bthandler.EvaluacionHandler
	Consumer     *events.EmpresaDeBajaConsumer
}

// newBolsaTrabajoModule wires repositories → services → handlers for the bolsatrabajo domain.
func newBolsaTrabajoModule(pool *pgxpool.Pool, rdb *redis.Client, cfg *config.Config) *bolsaTrabajoModule {
	// Infrastructure — event publisher
	publisher := events.NewRedisStreamPublisher(rdb)

	// Repositories
	trabajoRepo := btrepo.NewPgxTrabajoRepository(pool)
	postulacionRepo := btrepo.NewPgxPostulacionRepository(pool)
	evaluacionRepo := btrepo.NewPgxEvaluacionRepository(pool)

	// Turnstile client
	var turnstileSecret string
	var turnstileEnabled bool
	if cfg != nil {
		turnstileSecret = cfg.Recruitment.TurnstileSecretKey
		turnstileEnabled = cfg.Recruitment.TurnstileEnabled
	}
	tc := turnstile.New(turnstileEnabled, turnstileSecret, "https://challenges.cloudflare.com/turnstile/v0/siteverify")

	// Storage — use S3Storage stub when a bucket is configured, otherwise LocalStorage.
	var cvStorage storage.BolsaTrabajoStorage
	if cfg != nil && cfg.Recruitment.AWSS3BucketName != "" {
		cvStorage = storage.S3Storage{}
	} else {
		cvStorage = storage.LocalStorage{BaseDir: os.TempDir()}
	}

	// Services
	trabajoSvc := btsvc.NewTrabajoService(trabajoRepo, publisher)
	postulacionSvc := btsvc.NewPostulacionService(postulacionRepo, tc, cvStorage, publisher)
	evaluacionSvc := btsvc.NewEvaluacionService(evaluacionRepo, publisher)

	// Handlers
	trabajoH := bthandler.NewTrabajoHandler(trabajoSvc)
	postulacionH := bthandler.NewPostulacionHandler(postulacionSvc,
		bthandler.WithRateLimit(10, time.Minute),
	)
	evaluacionH := bthandler.NewEvaluacionHandler(evaluacionSvc)

	// Consumer for cross-domain "empresa de baja" event
	consumer := events.NewEmpresaDeBajaConsumer(rdb, trabajoSvc)

	return &bolsaTrabajoModule{
		trabajoH:     trabajoH,
		postulacionH: postulacionH,
		evaluacionH:  evaluacionH,
		Consumer:     consumer,
	}
}

// registerBolsaTrabajoRoutes mounts all bolsa-trabajo routes under /api/v1/bolsa-trabajo.
//
// Route matrix:
//
//	POST   /trabajo                                requireRoles(ADMIN,RECLUTADOR) → trabajo.Create
//	GET    /trabajo/activeJobs                     public → trabajo.ActiveJobs
//	GET    /trabajo/by-empresa/{id}               jwtAuth → trabajo.ByEmpresa
//	GET    /trabajo/recent                         public → trabajo.Recent
//	PATCH  /trabajo/switch/{id}                   jwtAuth + requireRoles → trabajo.SwitchEstatus
//	POST   /postulacion/apply                      IPRateLimiter (baked in) → postulacion.Apply
//	GET    /postulacion/by-trabajo-stats/{id}      jwtAuth + requireRoles → postulacion.GetStats
//	PATCH  /postulacion/status/{id}               jwtAuth + requireRoles → postulacion.UpdateEstatus
//	POST   /evaluacion                             jwtAuth + requireRoles → evaluacion.Create
//	GET    /evaluacion/by-postulacion/{id}         jwtAuth + requireRoles → evaluacion.GetByPostulacion
func registerBolsaTrabajoRoutes(
	r chi.Router,
	m *bolsaTrabajoModule,
	jwtAuth func(http.Handler) http.Handler,
	requireRoles func(...string) func(http.Handler) http.Handler,
) {
	adminRecruiter := requireRoles("ADMIN", "RECLUTADOR")

	r.Route("/api/v1/bolsa-trabajo", func(r chi.Router) {
		// ── Trabajo ───────────────────────────────────────────────────────────
		r.With(jwtAuth, adminRecruiter).Post("/trabajo", m.trabajoH.Create)
		r.Get("/trabajo/activeJobs", m.trabajoH.ActiveJobs)
		r.With(jwtAuth).Get("/trabajo/by-empresa/{id}", m.trabajoH.ByEmpresa)
		r.Get("/trabajo/recent", m.trabajoH.Recent)
		r.With(jwtAuth, adminRecruiter).Patch("/trabajo/switch/{id}", m.trabajoH.SwitchEstatus)

		// ── Postulacion ───────────────────────────────────────────────────────
		// Apply is public but IP-rate-limited (limiter baked into the handler).
		r.Post("/postulacion/apply", m.postulacionH.Apply)
		r.With(jwtAuth, adminRecruiter).Get("/postulacion/by-trabajo-stats/{id}", m.postulacionH.GetStats)
		r.With(jwtAuth, adminRecruiter).Patch("/postulacion/status/{id}", m.postulacionH.UpdateEstatus)

		// ── Evaluacion ────────────────────────────────────────────────────────
		r.With(jwtAuth, adminRecruiter).Post("/evaluacion", m.evaluacionH.Create)
		r.With(jwtAuth, adminRecruiter).Get("/evaluacion/by-postulacion/{id}", m.evaluacionH.GetByPostulacion)
	})
}
