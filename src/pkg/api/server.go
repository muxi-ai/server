package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/muxi-ai/server/pkg/auth"
	"github.com/muxi-ai/server/pkg/config"
	"github.com/muxi-ai/server/pkg/process"
	"github.com/muxi-ai/server/pkg/proxy"
	"github.com/muxi-ai/server/pkg/registry"
	"github.com/muxi-ai/server/pkg/updates"
	"github.com/rs/zerolog"
)

// Server represents the HTTP API server
type Server struct {
	router         *mux.Router
	httpServer     *http.Server
	config         *config.Config
	processManager *process.Manager
	registry       *registry.Registry
	authMiddleware *auth.Middleware
	proxyHandler   *proxy.Handler
	logger         *zerolog.Logger
	version        string // Server version (injected at build time)
}

// NewServer creates a new API server
func NewServer(
	cfg *config.Config,
	processManager *process.Manager,
	registry *registry.Registry,
	authMiddleware *auth.Middleware,
	logger *zerolog.Logger,
	version string,
) *Server {
	router := mux.NewRouter()

	server := &Server{
		router:         router,
		config:         cfg,
		processManager: processManager,
		registry:       registry,
		authMiddleware: authMiddleware,
		proxyHandler:   proxy.NewHandler(registry, processManager, version),
		logger:         logger,
		version:        version,
	}

	// Setup routes
	server.setupRoutes()

	// Setup HTTP server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	server.httpServer = &http.Server{
		Addr:         addr,
		Handler:      server.router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 0, // Disabled for SSE streaming (health checks can take 2+ minutes)
		IdleTimeout:  60 * time.Second,
	}

	return server
}

// setupRoutes configures all API routes
func (s *Server) setupRoutes() {
	// Add logging middleware (all routes)
	s.router.Use(s.loggingMiddleware)

	// Add CORS middleware (all routes, for development)
	s.router.Use(s.corsMiddleware)

	// ====================================
	// PUBLIC ENDPOINTS (no auth)
	// ====================================
	s.router.HandleFunc("/health", s.HandleHealth).Methods(http.MethodGet)
	s.router.HandleFunc("/ping", s.HandlePing).Methods(http.MethodGet)
	s.router.HandleFunc("/docs", s.HandleDocs).Methods(http.MethodGet)

	// ====================================
	// MANAGEMENT API /rpc/* (requires auth)
	// ====================================
	rpc := s.router.PathPrefix("/rpc").Subrouter()
	rpc.Use(s.authMiddleware.Authenticate)
	rpc.Use(s.auditMiddleware) // Audit logging for all /rpc/* requests

	// Formation management
	rpc.HandleFunc("/formations", s.HandleDeploy).Methods(http.MethodPost)
	rpc.HandleFunc("/formations", s.HandleList).Methods(http.MethodGet)
	rpc.HandleFunc("/formations/{id}", s.HandleGet).Methods(http.MethodGet)
	rpc.HandleFunc("/formations/{id}", s.HandleUpdate).Methods(http.MethodPut)
	rpc.HandleFunc("/formations/{id}", s.HandleDelete).Methods(http.MethodDelete)

	// Formation actions
	rpc.HandleFunc("/formations/{id}/start", s.HandleStart).Methods(http.MethodPost)
	rpc.HandleFunc("/formations/{id}/stop", s.HandleStop).Methods(http.MethodPost)
	rpc.HandleFunc("/formations/{id}/restart", s.HandleRestart).Methods(http.MethodPost)
	rpc.HandleFunc("/formations/{id}/rollback", s.HandleRollback).Methods(http.MethodPost)
	rpc.HandleFunc("/formations/{id}/cancel-update", s.HandleCancelUpdate).Methods(http.MethodPost)
	rpc.HandleFunc("/formations/{id}/logs", s.HandleLogs).Methods(http.MethodGet)
	rpc.HandleFunc("/formations/{id}/download", s.HandleDownload).Methods(http.MethodGet)
	rpc.HandleFunc("/formations/{id}/draft/files", s.HandleDraft).Methods(http.MethodPost)

	// Server management
	rpc.HandleFunc("/server/status", s.HandleServerStatus).Methods(http.MethodGet)
	rpc.HandleFunc("/server/logs", s.HandleServerLogs).Methods(http.MethodGet)

	// ====================================
	// FORMATION PROXY /api/* (no auth)
	// ====================================
	// Pattern: /api/{formation_id}/*
	// Example: /api/my-api/v1/chat → http://127.0.0.1:8001/v1/chat
	apiRouter := s.router.PathPrefix("/api").Subrouter()
	apiRouter.Use(s.sdkVersionMiddleware) // Add SDK version header for update notifications
	apiRouter.PathPrefix("/{formation_id}/{path:.*}").HandlerFunc(s.proxyHandler.ProxyRequest)
	apiRouter.PathPrefix("/{formation_id}").HandlerFunc(s.proxyHandler.ProxyRequest)

	// /api with no formation ID → 404
	s.router.HandleFunc("/api", s.handle404).Methods(http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete)

	// ====================================
	// CATCH-ALL (404)
	// ====================================
	s.router.NotFoundHandler = http.HandlerFunc(s.handle404)
}

// handle404 returns a 404 error
func (s *Server) handle404(w http.ResponseWriter, r *http.Request) {
	RespondError(w, http.StatusNotFound, "Endpoint not found")
}

// Start starts the HTTP server
func (s *Server) Start() error {
	// Server starting silently (logged in main.go)

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start server: %w", err)
	}

	return nil
}

// Stop gracefully stops the HTTP server
func (s *Server) Stop(ctx context.Context) error {
	s.logger.Info().Msg("Stopping HTTP server")

	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown server: %w", err)
	}

	return nil
}

// HandleHealth handles GET /health
func (s *Server) HandleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	version := s.version
	if version == "" {
		version = "1.0.0-dev"
	}

	fmt.Fprintf(w, `{"success":true,"status":"ok","version":"%s"}`, version)
}

// Middleware

// loggingMiddleware logs all HTTP requests
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Call next handler
		next.ServeHTTP(w, r)

		// Log request
		s.logger.Info().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Str("remote_addr", r.RemoteAddr).
			Dur("duration", time.Since(start)).
			Msg("HTTP request")
	})
}

// corsMiddleware adds CORS headers
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Handle preflight
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// sdkVersionMiddleware adds X-Muxi-SDK-Latest header when X-Muxi-SDK is present.
// This allows SDKs to know when updates are available.
func (s *Server) sdkVersionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse incoming SDK header: "typescript/0.1.0"
		sdkHeader := r.Header.Get("X-Muxi-SDK")
		if sdkHeader != "" {
			sdk, _ := updates.ParseSDKHeader(sdkHeader)
			if latest := updates.GetSDKLatest(sdk); latest != "" {
				w.Header().Set("X-Muxi-SDK-Latest", latest)
			}
		}

		next.ServeHTTP(w, r)
	})
}
