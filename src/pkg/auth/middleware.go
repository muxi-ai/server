package auth

import (
	"encoding/json"
	"net/http"

	"github.com/muxi-ai/server/pkg/config"
	"github.com/rs/zerolog"
)

// respondError sends an error response (local copy to avoid import cycle)
func respondError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error":   http.StatusText(status),
		"message": message,
		"code":    status,
	})
}

// Middleware provides authentication for HTTP requests
type Middleware struct {
	config *config.AuthConfig
	logger *zerolog.Logger
}

// NewMiddleware creates a new auth middleware
func NewMiddleware(cfg *config.AuthConfig, logger *zerolog.Logger) *Middleware {
	return &Middleware{
		config: cfg,
		logger: logger,
	}
}

// Authenticate returns middleware that validates HMAC signatures
func (m *Middleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip if auth disabled
		if !m.config.Enabled {
			next.ServeHTTP(w, r)
			return
		}

		// Get authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			m.logger.Warn().
				Str("path", r.URL.Path).
				Str("remote_addr", r.RemoteAddr).
				Msg("Missing authorization header")

			respondError(w, http.StatusUnauthorized, "Missing authorization header")
			return
		}

		// Parse header
		key, timestamp, signature, err := ParseAuthHeader(authHeader)
		if err != nil {
			m.logger.Warn().
				Err(err).
				Str("path", r.URL.Path).
				Str("header", authHeader).
				Msg("Failed to parse authorization header")

			respondError(w, http.StatusUnauthorized, "Invalid authorization header format")
			return
		}

		// Validate key
		if key != m.config.Key {
			m.logger.Warn().
				Str("key", key).
				Str("path", r.URL.Path).
				Str("remote_addr", r.RemoteAddr).
				Msg("Invalid key")

			respondError(w, http.StatusUnauthorized, "Invalid key")
			return
		}

		// Validate timestamp
		if err := ValidateTimestamp(timestamp, m.config.TimestampTolerance); err != nil {
			m.logger.Warn().
				Err(err).
				Str("timestamp", timestamp).
				Str("path", r.URL.Path).
				Msg("Invalid timestamp")

			respondError(w, http.StatusUnauthorized, "Request expired (timestamp too old or in future)")
			return
		}

		// Compute expected signature
		expected := ComputeHMAC(m.config.Secret, timestamp, r.Method, r.URL.Path)

		// Compare signatures (constant time)
		if !CompareSignatures(signature, expected) {
			m.logger.Warn().
				Str("path", r.URL.Path).
				Str("method", r.Method).
				Str("remote_addr", r.RemoteAddr).
				Msg("Invalid signature")

			respondError(w, http.StatusUnauthorized, "Invalid signature")
			return
		}

		// Authentication successful
		m.logger.Debug().
			Str("key", key).
			Str("path", r.URL.Path).
			Str("method", r.Method).
			Msg("Request authenticated")

		next.ServeHTTP(w, r)
	})
}
