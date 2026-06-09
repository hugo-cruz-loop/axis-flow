package main

import (
	"net/http"

	"axis-flow-back/internal/config"
	"axis-flow-back/internal/cursos/gotenberg"
	cursoshandler "axis-flow-back/internal/cursos/handler"
	cursosmw "axis-flow-back/internal/cursos/middleware"
	cursosrepo "axis-flow-back/internal/cursos/repository"
	cursossvc "axis-flow-back/internal/cursos/service"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// cursosModule holds the constructed handlers for the cursos domain.
type cursosModule struct {
	catalogH  *cursoshandler.CatalogHandler
	contentH  *cursoshandler.ContentHandler
	enrollH   *cursoshandler.EnrollmentHandler
	examH     *cursoshandler.ExamHandler
	certH     *cursoshandler.CertificateHandler
	enrollRepo cursosrepo.EnrollmentRepository
}

// newCursosModule wires repositories → services → handlers for the cursos domain.
func newCursosModule(pool *pgxpool.Pool, rdb *redis.Client, cfg *config.Config) *cursosModule {
	// Repositories
	catalogRepo := cursosrepo.NewPgxCatalogRepository(pool, rdb)
	contentRepo := cursosrepo.NewPgxContentRepository(pool, rdb)
	enrollRepo := cursosrepo.NewPgxEnrollmentRepository(pool, rdb)
	examRepo := cursosrepo.NewPgxExamRepository(pool, rdb)

	// PDF client — uses config values loaded from environment.
	var pdfEndpoint string
	var pdfEnabled bool
	if cfg != nil {
		pdfEndpoint = cfg.PDF.Endpoint
		pdfEnabled = cfg.PDF.GotenbergEnabled
	}
	pdfClient := gotenberg.New(pdfEndpoint, pdfEnabled)

	// Services
	cursoSvc := cursossvc.NewCursoService(catalogRepo, contentRepo)
	examSvc := cursossvc.NewExamService(examRepo)
	certSvc := cursossvc.NewCertificateService(examRepo, pdfClient)

	// Handlers
	return &cursosModule{
		catalogH:   cursoshandler.NewCatalogHandler(cursoSvc),
		contentH:   cursoshandler.NewContentHandler(cursoSvc),
		enrollH:    cursoshandler.NewEnrollmentHandler(enrollRepo),
		examH:      cursoshandler.NewExamHandler(examSvc),
		certH:      cursoshandler.NewCertificateHandler(certSvc),
		enrollRepo: enrollRepo,
	}
}

// registerCursosRoutes mounts all cursos routes on the provided chi.Router.
// Every route is protected by jwtAuth. Admin-only mutations are additionally
// wrapped with requireRoles("ADMIN"). Enrollment/progress routes that require
// the caller to be enrolled are wrapped with the IsEnrolled middleware.
func registerCursosRoutes(
	r chi.Router,
	m *cursosModule,
	jwtAuth func(http.Handler) http.Handler,
	requireRoles func(...string) func(http.Handler) http.Handler,
) {
	isEnrolled := cursosmw.IsEnrolled(m.enrollRepo)
	adminOnly := requireRoles("ADMIN")

	r.Group(func(r chi.Router) {
		r.Use(jwtAuth)

		// ── Catalog (Admin only) ───────────────────────────────────────────
		r.With(adminOnly).Post("/api/v1/categoria", m.catalogH.CreateCategoria)
		r.Get("/api/v1/categorias", m.catalogH.ListCategorias)
		r.With(adminOnly).Put("/api/v1/categoria/{id}", m.catalogH.UpdateCategoria)
		r.With(adminOnly).Delete("/api/v1/categoria/{id}", m.catalogH.DeleteCategoria)
		r.With(adminOnly).Post("/api/v1/modulo", m.catalogH.CreateModulo)
		r.Get("/api/v1/modulos", m.catalogH.ListModulos)

		// ── Courses (public list; write = Admin) ───────────────────────────
		r.With(adminOnly).Post("/api/v1/curso", m.contentH.CreateCurso)
		r.Get("/api/v1/cursos", m.contentH.ListCursos)
		r.Get("/api/v1/curso/{id}", m.contentH.GetCurso)
		r.Get("/api/v1/curso/{id}/contenido", m.contentH.GetCursoContenido)
		r.With(adminOnly).Put("/api/v1/curso/{id}", m.contentH.UpdateCurso)
		r.With(adminOnly).Delete("/api/v1/curso/{id}", m.contentH.DeleteCurso)

		// ── Content structure (Admin) ──────────────────────────────────────
		r.With(adminOnly).Post("/api/v1/unidad", m.contentH.CreateUnidad)
		r.With(adminOnly).Put("/api/v1/unidad/{id}", m.contentH.UpdateUnidad)
		r.With(adminOnly).Delete("/api/v1/unidad/{id}", m.contentH.DeleteUnidad)
		r.With(adminOnly).Post("/api/v1/leccion", m.contentH.CreateLeccion)
		r.With(adminOnly).Put("/api/v1/leccion/{id}", m.contentH.UpdateLeccion)
		r.With(adminOnly).Delete("/api/v1/leccion/{id}", m.contentH.DeleteLeccion)

		// ── Enrollment + progress (authenticated employees) ────────────────
		r.Post("/api/v1/enroll", m.enrollH.Enroll)
		r.Get("/api/v1/enroll/by-empleado/{empleado_id}", m.enrollH.ListEnrollmentsByEmpleado)
		r.With(isEnrolled).Post("/api/v1/avance-leccion", m.enrollH.MarkLeccionCompleta)
		r.With(isEnrolled).Get("/api/v1/leccion/{id}/notas", m.enrollH.GetNota)
		r.With(isEnrolled).Post("/api/v1/leccion/{id}/notas", m.enrollH.UpsertNota)

		// ── Exam (enrolled only) ───────────────────────────────────────────
		r.With(isEnrolled).Get("/api/v1/examen/{curso_id}", m.examH.GetExamenForStudent)
		// isEnrolled reads curso_id from ?curso_id= query param for this route (no URL param).
		r.With(isEnrolled).Post("/api/v1/examen/resolver", m.examH.ResolverExamen)
		r.Get("/api/v1/resultados-examen", m.examH.GetResultados)

		// ── Certificate ────────────────────────────────────────────────────
		// TODO: add IsEnrolled guard once examen→curso lookup is added to enrollment repo.
		// The service already validates aprobado status (which requires enrollment), so risk is low.
		r.Get("/api/v1/certificado/{examen_id}/download", m.certH.DownloadCertificate)
	})
}
