package main

import (
	"net/http"

	"axis-flow-back/internal/atencionseguimiento/events"
	atencionhandler "axis-flow-back/internal/atencionseguimiento/handler"
	atencionrepo "axis-flow-back/internal/atencionseguimiento/repository"
	atencionsvc "axis-flow-back/internal/atencionseguimiento/service"
	"axis-flow-back/internal/config"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// atencionModule holds the constructed handlers and consumers for the AtencionSeguimiento domain.
type atencionModule struct {
	quejaH      *atencionhandler.QuejaHandler
	ticketH     *atencionhandler.TicketHandler
	incidenciaH *atencionhandler.IncidenciaHandler

	EmpleadoDeBajaConsumer  *events.EmpleadoDeBajaConsumer
	ClienteInactivoConsumer *events.ClienteInactivoConsumer
}

// newAtencionModule wires repositories → services → handlers for the atencion domain.
func newAtencionModule(pool *pgxpool.Pool, rdb *redis.Client, _ *config.Config) *atencionModule {
	// Infrastructure
	publisher := events.NewRedisStreamPublisher(rdb)

	// Repositories
	quejaRepo := atencionrepo.NewPgxQuejaRepository(pool, rdb)
	ticketRepo := atencionrepo.NewPgxTicketRepository(pool, rdb)
	incidenciaRepo := atencionrepo.NewPgxIncidenciaRepository(pool)

	// Services
	quejaSvc := atencionsvc.NewQuejaService(quejaRepo, publisher)
	ticketSvc := atencionsvc.NewTicketService(ticketRepo, publisher)
	incidenciaSvc := atencionsvc.NewIncidenciaService(incidenciaRepo, publisher)

	// Handlers
	quejaH := atencionhandler.NewQuejaHandler(quejaSvc)
	ticketH := atencionhandler.NewTicketHandler(ticketSvc)
	incidenciaH := atencionhandler.NewIncidenciaHandler(incidenciaSvc)

	// Cross-domain event consumers
	empleadoConsumer := events.NewEmpleadoDeBajaConsumer(rdb, quejaSvc)
	clienteConsumer := events.NewClienteInactivoConsumer(rdb, ticketSvc)

	return &atencionModule{
		quejaH:                  quejaH,
		ticketH:                 ticketH,
		incidenciaH:             incidenciaH,
		EmpleadoDeBajaConsumer:  empleadoConsumer,
		ClienteInactivoConsumer: clienteConsumer,
	}
}

// registerAtencionRoutes mounts all atencion-seguimiento routes under /api/v1/atencion.
//
// Route matrix:
//
//	POST   /queja                                JWTAuth + RequireRoles("EMPLEADO") → quejaH.CreateQueja
//	GET    /queja/{id}                           JWTAuth + RequireRoles("EMPLEADO","RH","ADMIN") → quejaH.GetQueja
//	GET    /queja/{id}/mensajes                 JWTAuth → quejaH.GetMensajes
//	POST   /queja/{id}/mensaje                  JWTAuth → quejaH.CreateMensaje
//	GET    /empresa/{empresa_id}/quejas         JWTAuth + RequireRoles("RH","ADMIN") → quejaH.GetQuejasByEmpresa
//
//	POST   /ticket                               JWTAuth + RequireRoles("CLIENTE") → ticketH.CreateTicket
//	GET    /ticket/{id}/mensajes                JWTAuth → ticketH.GetMensajesServicio
//	POST   /ticket/{id}/mensaje                 JWTAuth → ticketH.CreateMensajeServicio
//	PATCH  /ticket/{id}/status                 JWTAuth + RequireRoles("GESTOR","ADMIN") → ticketH.UpdateTicketEstatus
//	GET    /cliente/{cliente_id}/tickets        JWTAuth → ticketH.GetTicketsByCliente
//	GET    /empresa/{empresa_id}/tickets/stats  JWTAuth + RequireRoles("GESTOR","RH","ADMIN") → ticketH.GetTicketStats
//
//	POST   /incidencia                           JWTAuth + RequireRoles("SUPERVISOR","ADMIN") → incidenciaH.CreateIncidencia
//	GET    /empresa/{empresa_id}/incidencias    JWTAuth + RequireRoles("ADMIN","RH") → incidenciaH.GetIncidenciasByEmpresa
func registerAtencionRoutes(
	r chi.Router,
	m *atencionModule,
	jwtAuth func(http.Handler) http.Handler,
	requireRoles func(...string) func(http.Handler) http.Handler,
) {
	r.Route("/api/v1/atencion", func(r chi.Router) {
		// ── Queja ────────────────────────────────────────────────────────────────
		r.With(jwtAuth, requireRoles("EMPLEADO")).Post("/queja", m.quejaH.CreateQueja)
		r.With(jwtAuth, requireRoles("EMPLEADO", "RH", "ADMIN")).Get("/queja/{id}", m.quejaH.GetQueja)
		r.With(jwtAuth).Get("/queja/{id}/mensajes", m.quejaH.GetMensajes)
		r.With(jwtAuth).Post("/queja/{id}/mensaje", m.quejaH.CreateMensaje)
		r.With(jwtAuth, requireRoles("RH", "ADMIN")).Get("/empresa/{empresa_id}/quejas", m.quejaH.GetQuejasByEmpresa)

		// ── Ticket ───────────────────────────────────────────────────────────────
		r.With(jwtAuth, requireRoles("CLIENTE")).Post("/ticket", m.ticketH.CreateTicket)
		r.With(jwtAuth).Get("/ticket/{id}/mensajes", m.ticketH.GetMensajesServicio)
		r.With(jwtAuth).Post("/ticket/{id}/mensaje", m.ticketH.CreateMensajeServicio)
		r.With(jwtAuth, requireRoles("GESTOR", "ADMIN")).Patch("/ticket/{id}/status", m.ticketH.UpdateTicketEstatus)
		r.With(jwtAuth).Get("/cliente/{cliente_id}/tickets", m.ticketH.GetTicketsByCliente)
		r.With(jwtAuth, requireRoles("GESTOR", "RH", "ADMIN")).Get("/empresa/{empresa_id}/tickets/stats", m.ticketH.GetTicketStats)

		// ── Incidencia ───────────────────────────────────────────────────────────
		r.With(jwtAuth, requireRoles("SUPERVISOR", "ADMIN")).Post("/incidencia", m.incidenciaH.CreateIncidencia)
		r.With(jwtAuth, requireRoles("ADMIN", "RH")).Get("/empresa/{empresa_id}/incidencias", m.incidenciaH.GetIncidenciasByEmpresa)
	})
}
