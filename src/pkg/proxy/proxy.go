package proxy

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/muxi-ai/server/pkg/process"
	"github.com/muxi-ai/server/pkg/registry"
	"github.com/rs/zerolog/log"
)

// ProcessGetter interface for getting process info
type ProcessGetter interface {
	Get(id string) (*process.Process, error)
}

// Handler is the HTTP proxy handler that forwards requests to formations
type Handler struct {
	registry       *registry.Registry
	processManager ProcessGetter
	client         *http.Client
}

// NewHandler creates a new proxy handler
func NewHandler(reg *registry.Registry, pm ProcessGetter) *Handler {
	return &Handler{
		registry:       reg,
		processManager: pm,
		client: &http.Client{
			Timeout: 30 * time.Second,
			// Don't follow redirects - pass them through
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

// ProxyRequest handles incoming proxy requests
// Pattern: /v1/{formation_id}/{path}
func (h *Handler) ProxyRequest(w http.ResponseWriter, r *http.Request) {
	// Extract formation ID from URL
	vars := mux.Vars(r)
	formationID := vars["formation_id"]

	// Get formation from registry
	formation, err := h.registry.Get(formationID)
	if err != nil {
		log.Warn().
			Str("formation_id", formationID).
			Str("path", r.URL.Path).
			Err(err).
			Msg("Formation not found for proxy request")
		h.respondError(w, http.StatusNotFound, "Formation not found", fmt.Sprintf("No formation with id '%s'", formationID))
		return
	}

	// Update formation with latest process status
	if h.processManager != nil {
		if proc, err := h.processManager.Get(formationID); err == nil {
			formation.UpdateFromProcess(proc)
		}
	}

	// Check formation status
	if formation.Status != "running" {
		log.Warn().
			Str("formation_id", formationID).
			Str("status", formation.Status).
			Msg("Formation not running")
		h.respondError(w, http.StatusServiceUnavailable, "Formation unavailable", fmt.Sprintf("Formation '%s' is not running (status: %s)", formationID, formation.Status))
		return
	}

	// Check health status (optional - can proxy even if health unknown)
	if !formation.Healthy {
		log.Warn().
			Str("formation_id", formationID).
			Msg("Formation unhealthy, proxying anyway")
		// Still proxy, but log warning
	}

	// Build target URL
	// Original: /v1/{formation_id}/{remaining_path}
	// Target:   http://localhost:{port}/{remaining_path}
	remainingPath := vars["path"]
	if remainingPath == "" {
		remainingPath = "/" // Root path if no path specified
	} else if !strings.HasPrefix(remainingPath, "/") {
		remainingPath = "/" + remainingPath // Ensure leading slash
	}
	targetURL := fmt.Sprintf("http://localhost:%d%s", formation.Port, remainingPath)

	// Parse target URL
	target, err := url.Parse(targetURL)
	if err != nil {
		log.Error().
			Err(err).
			Str("formation_id", formationID).
			Str("target_url", targetURL).
			Msg("Failed to parse target URL")
		h.respondError(w, http.StatusInternalServerError, "Internal server error", "Failed to parse target URL")
		return
	}

	// Preserve query parameters
	target.RawQuery = r.URL.RawQuery

	log.Debug().
		Str("formation_id", formationID).
		Str("method", r.Method).
		Str("source_path", r.URL.Path).
		Str("target_url", target.String()).
		Msg("Proxying request")

	// Create proxy request
	proxyReq, err := http.NewRequest(r.Method, target.String(), r.Body)
	if err != nil {
		log.Error().
			Err(err).
			Str("formation_id", formationID).
			Msg("Failed to create proxy request")
		h.respondError(w, http.StatusInternalServerError, "Internal server error", "Failed to create proxy request")
		return
	}

	// Copy headers from original request (except Host)
	for name, values := range r.Header {
		if name == "Host" {
			continue // Don't copy Host header
		}
		for _, value := range values {
			proxyReq.Header.Add(name, value)
		}
	}

	// Add X-Forwarded-* headers
	proxyReq.Header.Set("X-Forwarded-For", getClientIP(r))
	proxyReq.Header.Set("X-Forwarded-Proto", getScheme(r))
	proxyReq.Header.Set("X-Forwarded-Host", r.Host)
	proxyReq.Header.Set("X-Formation-ID", formationID)

	// Make the request
	resp, err := h.client.Do(proxyReq)
	if err != nil {
		log.Error().
			Err(err).
			Str("formation_id", formationID).
			Str("target_url", target.String()).
			Msg("Formation request failed")
		h.respondError(w, http.StatusBadGateway, "Bad gateway", "Formation did not respond")
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for name, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}

	// Copy status code
	w.WriteHeader(resp.StatusCode)

	// Copy response body
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		log.Error().
			Err(err).
			Str("formation_id", formationID).
			Msg("Failed to copy response body")
		// Can't write error response here - headers already sent
		return
	}

	log.Debug().
		Str("formation_id", formationID).
		Int("status", resp.StatusCode).
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Msg("Proxy request completed")
}

// respondError sends a JSON error response
func (h *Handler) respondError(w http.ResponseWriter, code int, error string, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	fmt.Fprintf(w, `{"error": "%s", "message": "%s", "code": %d}`, error, message, code)
}

// getClientIP extracts the client IP from the request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		// X-Forwarded-For can be a list, take the first one
		parts := strings.Split(forwarded, ",")
		return strings.TrimSpace(parts[0])
	}

	// Check X-Real-IP header
	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	// Fall back to RemoteAddr
	ip := r.RemoteAddr
	// Remove port if present
	if strings.Contains(ip, ":") {
		parts := strings.Split(ip, ":")
		return parts[0]
	}

	return ip
}

// getScheme returns the request scheme (http or https)
func getScheme(r *http.Request) string {
	// Check X-Forwarded-Proto header
	forwarded := r.Header.Get("X-Forwarded-Proto")
	if forwarded != "" {
		return forwarded
	}

	// Check if TLS is being used
	if r.TLS != nil {
		return "https"
	}

	return "http"
}
