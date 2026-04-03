// cmd/server/main.go
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
	"go.uber.org/zap"

	"github.com/eguilde/egudoc/internal/auth"
	"github.com/eguilde/egudoc/internal/config"
	"github.com/eguilde/egudoc/internal/database"
	"github.com/eguilde/egudoc/internal/health"
	"github.com/eguilde/egudoc/internal/rbac"
	"github.com/eguilde/egudoc/internal/storage"
	"github.com/eguilde/egudoc/internal/users"
)

func main() {
	log, err := zap.NewProduction()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create logger: %v\n", err)
		os.Exit(1)
	}
	defer log.Sync() //nolint:errcheck

	cfg := config.Load()

	// Database
	pool, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatal("database connect failed", zap.Error(err))
	}
	defer pool.Close()

	if err := database.EnsureSchema(context.Background(), pool); err != nil {
		log.Fatal("schema migration failed", zap.Error(err))
	}

	// Seed RBAC default roles and permissions
	if err := rbac.SeedDefaultRolesAndPermissions(context.Background(), pool); err != nil {
		log.Fatal("RBAC seed failed", zap.Error(err))
	}

	// MinIO storage
	store, err := storage.NewClient(cfg.MinioEndpoint, cfg.MinioAccessKey, cfg.MinioSecretKey, cfg.MinioBucket, cfg.MinioUseSSL)
	if err != nil {
		log.Fatal("minio client failed", zap.Error(err))
	}
	if err := store.EnsureBucket(context.Background()); err != nil {
		log.Fatal("minio bucket setup failed", zap.Error(err))
	}

	// Auth
	jwksCache := auth.NewJWKSCache(cfg.OIDCJWKSURL, 5*time.Minute)
	authMiddleware := auth.RequireAuth(jwksCache)

	// RBAC
	rbacSvc := rbac.NewService(pool, log)

	// Users
	userSvc := users.NewService(pool)
	userHandler := users.NewHandler(userSvc)

	// Health
	healthHandler := health.NewHandler(pool)

	// Router
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))
	r.Use(httprate.LimitByIP(100, time.Minute))

	// CORS
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", cfg.FrontendURL)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Institution-ID, X-Request-ID")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	})

	// Public routes
	r.Mount("/", healthHandler.Routes())

	// Authenticated routes
	r.Group(func(r chi.Router) {
		r.Use(authMiddleware)
		r.Mount("/api/users", userHandler.Routes())

		// Admin routes — superadmin only
		r.Group(func(r chi.Router) {
			r.Use(rbacSvc.RequireRole("superadmin", "institution_admin"))
			// RBAC admin, institution management — mounted in later sub-plans
		})
	})

	_ = store   // will be used by document handlers in sub-plans B, C, D
	_ = rbacSvc // will be used by feature handlers in sub-plans B, C, D

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info("egudoc starting", zap.String("port", cfg.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("server error", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info("shutting down")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error("graceful shutdown failed", zap.Error(err))
	}
	log.Info("server stopped")
}
