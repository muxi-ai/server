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
}

// NewServer creates a new API server
func NewServer(
	cfg *config.Config,
	processManager *process.Manager,
	registry *registry.Registry,
	authMiddleware *auth.Middleware,
	logger *zerolog.Logger,
) *Server {
	router := mux.NewRouter()

	server := &Server{
		router:         router,
		config:         cfg,
		processManager: processManager,
		registry:       registry,
		authMiddleware: authMiddleware,
		proxyHandler:   proxy.NewHandler(registry),
		logger:         logger,
	}

	// Setup routes
	server.setupRoutes()

	// Setup HTTP server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	server.httpServer = &http.Server{
		Addr:         addr,
		Handler:      server.router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
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

	// Health check (no auth)
	s.router.HandleFunc("/health", s.HandleHealth).Methods(http.MethodGet)

	// Management API (requires auth)
	mgmt := s.router.PathPrefix("/formations").Subrouter()
	mgmt.Use(s.authMiddleware.Authenticate)

	// Formation management
	mgmt.HandleFunc("/deploy", s.HandleDeploy).Methods(http.MethodPost)
	mgmt.HandleFunc("", s.HandleList).Methods(http.MethodGet)
	mgmt.HandleFunc("/{id}", s.HandleGet).Methods(http.MethodGet)
	mgmt.HandleFunc("/{id}", s.HandleDelete).Methods(http.MethodDelete)
	mgmt.HandleFunc("/{id}/stop", s.HandleStop).Methods(http.MethodPost)
	mgmt.HandleFunc("/{id}/restart", s.HandleRestart).Methods(http.MethodPost)
	mgmt.HandleFunc("/{id}/logs", s.HandleLogs).Methods(http.MethodGet)

	// Proxy API (no auth - transparent pass-through)
	// Pattern: /v1/{formation_id}/{path:.*}
	// Example: /v1/my-api/chat → http://localhost:8001/chat
	s.router.PathPrefix("/v1/{formation_id}/{path:.*}").HandlerFunc(s.proxyHandler.ProxyRequest)
	s.router.PathPrefix("/v1/{formation_id}").HandlerFunc(s.proxyHandler.ProxyRequest)
}

// Start starts the HTTP server
func (s *Server) Start() error {
	s.logger.Info().
		Str("addr", s.httpServer.Addr).
		Msg("Starting HTTP server")

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
	available, allocated, total := s.registry.PortPoolStatus()

	health := map[string]interface{}{
		"status":     "ok",
		"formations": s.registry.Count(),
		"port_pool": map[string]int{
			"available": available,
			"allocated": allocated,
			"total":     total,
		},
	}

	RespondSuccess(w, health)
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
