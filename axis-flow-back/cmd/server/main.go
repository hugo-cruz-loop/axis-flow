// Package main is the entry point for the axis-flow identity service.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"axis-flow-back/internal/config"
	"axis-flow-back/internal/handler"
	"axis-flow-back/internal/logging"
	"axis-flow-back/internal/middleware"
	"axis-flow-back/internal/repository"
	"axis-flow-back/internal/service"
	"axis-flow-back/internal/telemetry"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// validateConfig checks invariants that must hold before the server starts.
func validateConfig(cfg config.Config) error {
	if cfg.Feature.DeleteAllData && cfg.AppEnv == "production" {
		return fmt.Errorf("FEATURE_DELETE_ALL_DATA=true is not allowed in production")
	}
	return nil
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load configuration", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// ── Production startup guard ─────────────────────────────────────────────
	if err := validateConfig(*cfg); err != nil {
		slog.Error("FATAL: " + err.Error())
		os.Exit(1)
	}

	// ── Structured logger ────────────────────────────────────────────────────
	logLevel := middleware.ParseSlogLevel(cfg.Observability.LogLevel)
	slog.SetDefault(logging.New(os.Stdout, cfg.Observability.LogFormat, logLevel))

	// ── OpenTelemetry ────────────────────────────────────────────────────────
	ctx := context.Background()
	shutdownTracer, err := telemetry.InitTracer(ctx, cfg.Observability)
	if err != nil {
		slog.Error("failed to initialise tracer", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer func() {
		if err := shutdownTracer(ctx); err != nil {
			slog.Error("tracer shutdown error", slog.String("error", err.Error()))
		}
	}()

	// ── Database ─────────────────────────────────────────────────────────────
	poolCfg, err := pgxpool.ParseConfig(cfg.DB.DSN())
	if err != nil {
		slog.Error("invalid db dsn", slog.String("error", err.Error()))
		os.Exit(1)
	}
	poolCfg.MaxConns = int32(cfg.DB.MaxOpenConns)
	poolCfg.MinConns = int32(cfg.DB.MaxIdleConns)

	dbPool, err := pgxpool.NewWithConfig(context.Background(), poolCfg)
	if err != nil {
		slog.Error("failed to connect to database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer dbPool.Close()

	if err := dbPool.Ping(context.Background()); err != nil {
		slog.Error("database ping failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	slog.Info("database connected")

	// ── Redis ─────────────────────────────────────────────────────────────────
	redisOpts, err := redis.ParseURL(cfg.Redis.URL)
	if err != nil {
		slog.Error("invalid redis url", slog.String("error", err.Error()))
		os.Exit(1)
	}
	redisClient := redis.NewClient(redisOpts)
	defer redisClient.Close()

	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		slog.Error("redis ping failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	slog.Info("redis connected")

	// ── Repositories ──────────────────────────────────────────────────────────
	userRepo := repository.NewPgxUserRepository(dbPool)
	sessionRepo := repository.NewPgxSessionRepository(dbPool)
	tokenRepo := repository.NewPgxTokenRepository(dbPool)
	auditRepo := repository.NewPgxAuditRepository(dbPool)

	// ── Services ──────────────────────────────────────────────────────────────
	authSvc := service.NewAuthService(
		userRepo,
		sessionRepo,
		cfg.JWT.Secret,
		cfg.JWT.AccessTTL,
		cfg.JWT.RefreshTTL,
	)
	tokenSvc := service.NewTokenService(tokenRepo)
	activationSvc := service.NewActivationService(*tokenSvc, userRepo)
	userSvc := service.NewUserService(userRepo, auditRepo, tokenSvc)

	// deleteEnabled: must not be true in production (already guarded above)
	deleteEnabled := cfg.AppEnv != "production" && cfg.Feature.DeleteAllData

	// ── Prometheus metrics ────────────────────────────────────────────────────
	metricsRegistry := prometheus.NewRegistry()
	appMetrics := middleware.NewMetrics(metricsRegistry)

	// ── Handlers ──────────────────────────────────────────────────────────────
	authHandler := handler.NewAuthHandler(authSvc)
	activationHandler := handler.NewActivationHandler(activationSvc)
	userHandler := handler.NewUserHandler(userSvc, deleteEnabled)

	// ── Router ────────────────────────────────────────────────────────────────
	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Recoverer)
	r.Use(middleware.NewLogger(cfg.Observability.LogLevel, cfg.Observability.LogFormat))
	r.Use(middleware.MetricsMiddleware(appMetrics))

	// Health endpoints (no auth)
	r.Get("/health/live", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	r.Get("/health/ready", func(w http.ResponseWriter, r *http.Request) {
		if err := dbPool.Ping(r.Context()); err != nil {
			http.Error(w, `{"status":"db_down"}`, http.StatusServiceUnavailable)
			return
		}
		if err := redisClient.Ping(r.Context()).Err(); err != nil {
			http.Error(w, `{"status":"redis_down"}`, http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	// Metrics endpoint (guarded by feature flag)
	if cfg.Observability.MetricsEnabled {
		r.Get("/metrics", promhttp.HandlerFor(metricsRegistry, promhttp.HandlerOpts{}).ServeHTTP)
	}

	// Public auth routes
	r.Route("/api/auth", func(r chi.Router) {
		// Rate-limit login: 10 req/min per IP
		r.With(middleware.RateLimitMiddleware(redisClient, 10, time.Minute)).
			Post("/login/", authHandler.Login)
		r.Post("/token/refresh/", authHandler.RefreshToken)

		// Protected: requires valid JWT
		r.Group(func(r chi.Router) {
			r.Use(middleware.JWTAuth(authSvc))
			r.Get("/me/", authHandler.Me)
		})
	})

	// User management routes
	r.Route("/api/users", func(r chi.Router) {
		// Public activation / password reset
		r.Patch("/reset-pass/", activationHandler.ResetPass)
		r.Patch("/reset-pass-app/{id}/", userHandler.ResetPassApp)

		// Protected: require valid JWT
		r.Group(func(r chi.Router) {
			r.Use(middleware.JWTAuth(authSvc))

			// Admin-only: create user
			r.With(middleware.RequireRoles("AdminCheckOn", "Administrador")).
				Post("/", userHandler.CreateUser)

			// Any authenticated user
			r.Get("/filtradoUser/{role}/", userHandler.ListByRole)
			r.Get("/ultimoID/", userHandler.GetLastCreatedID)
			r.Patch("/update-firebase-token/", userHandler.UpdateFirebaseToken)

			// Admin-only: read FCM token
			r.With(middleware.RequireRoles("AdminCheckOn", "Administrador")).
				Get("/get-firebase-token/{id}/", userHandler.GetFirebaseToken)
		})

		// TBD endpoints — registered as 501 Not Implemented stubs
		r.Post("/userApp/", handler.NotImplemented)
		r.Post("/userCliente/", handler.NotImplemented)
		r.Post("/googleUser/", handler.NotImplemented)
		r.Post("/facebookUser/", handler.NotImplemented)

		// Dev/test destructive reset — registered only when feature flag is active and not production
		if deleteEnabled {
			r.Group(func(r chi.Router) {
				r.Use(middleware.JWTAuth(authSvc))
				r.Delete("/delete-all-data/", userHandler.DeleteAllData)
			})
		} else {
			r.Delete("/delete-all-data/", func(w http.ResponseWriter, _ *http.Request) {
				http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			})
		}
	})

	// Role management routes (admin only)
	roleRepo := repository.NewPgxRoleRepository(dbPool)
	roleHandler := handler.NewRoleHandler(roleRepo)

	r.Route("/api/roles", func(r chi.Router) {
		r.Use(middleware.JWTAuth(authSvc))
		r.Use(middleware.RequireRoles("ADMIN_CHECK_ON", "ADMINISTRADOR"))
		r.Get("/", roleHandler.ListRoles)
		r.Get("/{code}/permissions", roleHandler.ListPermissions)
	})

	// ── HTTP Server ───────────────────────────────────────────────────────────
	// Wrap router with OTel HTTP instrumentation.
	handler := otelhttp.NewHandler(r, "axis-flow-back")

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Server.Port),
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		slog.Info("server starting", slog.String("addr", srv.Addr), slog.String("env", cfg.AppEnv))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	<-quit
	slog.Info("shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server forced shutdown", slog.String("error", err.Error()))
		os.Exit(1)
	}

	slog.Info("server exited gracefully")
}
