package auth

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/muxi-ai/server/pkg/config"
	"github.com/rs/zerolog"
)

func TestNewMiddleware(t *testing.T) {
	logger := zerolog.Nop()
	cfg := &config.AuthConfig{
		Enabled:            true,
		Key:                "test-key",
		Secret:             "test-secret",
		TimestampTolerance: 300,
	}

	middleware := NewMiddleware(cfg, &logger)

	if middleware == nil {
		t.Fatal("NewMiddleware() returned nil")
	}

	if middleware.config != cfg {
		t.Error("Middleware config not set correctly")
	}

	if middleware.logger != &logger {
		t.Error("Middleware logger not set correctly")
	}
}

func TestMiddleware_AuthDisabled(t *testing.T) {
	logger := zerolog.Nop()
	cfg := &config.AuthConfig{
		Enabled: false, // Auth disabled
	}

	middleware := NewMiddleware(cfg, &logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	req := httptest.NewRequest("GET", "/test", nil)
	// No authorization header
	w := httptest.NewRecorder()

	middleware.Authenticate(handler).ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Status = %d, want %d (auth disabled)", resp.StatusCode, http.StatusOK)
	}
}

func TestMiddleware_MissingAuthHeader(t *testing.T) {
	logger := zerolog.Nop()
	cfg := &config.AuthConfig{
		Enabled:            true,
		Key:                "test-key",
		Secret:             "test-secret",
		TimestampTolerance: 300,
	}

	middleware := NewMiddleware(cfg, &logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	middleware.Authenticate(handler).ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Status = %d, want %d for missing auth header", resp.StatusCode, http.StatusUnauthorized)
	}

	body, _ := io.ReadAll(resp.Body)
	if !contains(string(body), "Missing authorization header") {
		t.Errorf("Body = %q, want error about missing header", string(body))
	}
}

func TestMiddleware_InvalidHeaderFormat(t *testing.T) {
	logger := zerolog.Nop()
	cfg := &config.AuthConfig{
		Enabled:            true,
		Key:                "test-key",
		Secret:             "test-secret",
		TimestampTolerance: 300,
	}

	middleware := NewMiddleware(cfg, &logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	tests := []struct {
		name   string
		header string
	}{
		{"wrong prefix", "Bearer token123"},
		{"malformed", "MUXI-HMAC invalid"},
		{"missing key", "MUXI-HMAC timestamp=123, signature=abc"},
		{"missing timestamp", "MUXI-HMAC key=abc, signature=xyz"},
		{"missing signature", "MUXI-HMAC key=abc, timestamp=123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("Authorization", tt.header)
			w := httptest.NewRecorder()

			middleware.Authenticate(handler).ServeHTTP(w, req)

			resp := w.Result()
			if resp.StatusCode != http.StatusUnauthorized {
				t.Errorf("Status = %d, want %d for invalid header", resp.StatusCode, http.StatusUnauthorized)
			}
		})
	}
}

func TestMiddleware_InvalidKey(t *testing.T) {
	logger := zerolog.Nop()
	cfg := &config.AuthConfig{
		Enabled:            true,
		Key:                "correct-key",
		Secret:             "test-secret",
		TimestampTolerance: 300,
	}

	middleware := NewMiddleware(cfg, &logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	signature := ComputeHMAC("test-secret", timestamp, "GET", "/test")

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", fmt.Sprintf("MUXI-HMAC key=wrong-key, timestamp=%s, signature=%s", timestamp, signature))
	w := httptest.NewRecorder()

	middleware.Authenticate(handler).ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Status = %d, want %d for invalid key", resp.StatusCode, http.StatusUnauthorized)
	}

	body, _ := io.ReadAll(resp.Body)
	if !contains(string(body), "Invalid key") {
		t.Errorf("Body = %q, want error about invalid key", string(body))
	}
}

func TestMiddleware_ExpiredTimestamp(t *testing.T) {
	logger := zerolog.Nop()
	cfg := &config.AuthConfig{
		Enabled:            true,
		Key:                "test-key",
		Secret:             "test-secret",
		TimestampTolerance: 300,
	}

	middleware := NewMiddleware(cfg, &logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Timestamp 10 minutes in the past (beyond tolerance)
	oldTimestamp := fmt.Sprintf("%d", time.Now().Unix()-600)
	signature := ComputeHMAC("test-secret", oldTimestamp, "GET", "/test")

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", fmt.Sprintf("MUXI-HMAC key=test-key, timestamp=%s, signature=%s", oldTimestamp, signature))
	w := httptest.NewRecorder()

	middleware.Authenticate(handler).ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Status = %d, want %d for expired timestamp", resp.StatusCode, http.StatusUnauthorized)
	}

	body, _ := io.ReadAll(resp.Body)
	if !contains(string(body), "expired") {
		t.Errorf("Body = %q, want error about expired timestamp", string(body))
	}
}

func TestMiddleware_FutureTimestamp(t *testing.T) {
	logger := zerolog.Nop()
	cfg := &config.AuthConfig{
		Enabled:            true,
		Key:                "test-key",
		Secret:             "test-secret",
		TimestampTolerance: 300,
	}

	middleware := NewMiddleware(cfg, &logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Timestamp 10 minutes in the future (beyond tolerance)
	futureTimestamp := fmt.Sprintf("%d", time.Now().Unix()+600)
	signature := ComputeHMAC("test-secret", futureTimestamp, "GET", "/test")

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", fmt.Sprintf("MUXI-HMAC key=test-key, timestamp=%s, signature=%s", futureTimestamp, signature))
	w := httptest.NewRecorder()

	middleware.Authenticate(handler).ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Status = %d, want %d for future timestamp", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestMiddleware_InvalidSignature(t *testing.T) {
	logger := zerolog.Nop()
	cfg := &config.AuthConfig{
		Enabled:            true,
		Key:                "test-key",
		Secret:             "test-secret",
		TimestampTolerance: 300,
	}

	middleware := NewMiddleware(cfg, &logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	wrongSignature := "wrong-signature-value"

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", fmt.Sprintf("MUXI-HMAC key=test-key, timestamp=%s, signature=%s", timestamp, wrongSignature))
	w := httptest.NewRecorder()

	middleware.Authenticate(handler).ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Status = %d, want %d for invalid signature", resp.StatusCode, http.StatusUnauthorized)
	}

	body, _ := io.ReadAll(resp.Body)
	if !contains(string(body), "Invalid signature") {
		t.Errorf("Body = %q, want error about invalid signature", string(body))
	}
}

func TestMiddleware_ValidAuthentication(t *testing.T) {
	logger := zerolog.Nop()
	cfg := &config.AuthConfig{
		Enabled:            true,
		Key:                "test-key",
		Secret:             "test-secret",
		TimestampTolerance: 300,
	}

	middleware := NewMiddleware(cfg, &logger)

	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	signature := ComputeHMAC("test-secret", timestamp, "GET", "/test")

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", fmt.Sprintf("MUXI-HMAC key=test-key, timestamp=%s, signature=%s", timestamp, signature))
	w := httptest.NewRecorder()

	middleware.Authenticate(handler).ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Status = %d, want %d for valid auth", resp.StatusCode, http.StatusOK)
	}

	if !handlerCalled {
		t.Error("Handler should be called for valid authentication")
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "success" {
		t.Errorf("Body = %q, want %q", string(body), "success")
	}
}

func TestMiddleware_DifferentMethods(t *testing.T) {
	logger := zerolog.Nop()
	cfg := &config.AuthConfig{
		Enabled:            true,
		Key:                "test-key",
		Secret:             "test-secret",
		TimestampTolerance: 300,
	}

	middleware := NewMiddleware(cfg, &logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH"}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			timestamp := fmt.Sprintf("%d", time.Now().Unix())
			signature := ComputeHMAC("test-secret", timestamp, method, "/test")

			req := httptest.NewRequest(method, "/test", nil)
			req.Header.Set("Authorization", fmt.Sprintf("MUXI-HMAC key=test-key, timestamp=%s, signature=%s", timestamp, signature))
			w := httptest.NewRecorder()

			middleware.Authenticate(handler).ServeHTTP(w, req)

			resp := w.Result()
			if resp.StatusCode != http.StatusOK {
				t.Errorf("%s: Status = %d, want %d", method, resp.StatusCode, http.StatusOK)
			}
		})
	}
}

func TestMiddleware_DifferentPaths(t *testing.T) {
	logger := zerolog.Nop()
	cfg := &config.AuthConfig{
		Enabled:            true,
		Key:                "test-key",
		Secret:             "test-secret",
		TimestampTolerance: 300,
	}

	middleware := NewMiddleware(cfg, &logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	paths := []string{
		"/formations",
		"/formations/test-id",
		"/formations/test-id/stop",
		"/api/v1/test",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			timestamp := fmt.Sprintf("%d", time.Now().Unix())
			signature := ComputeHMAC("test-secret", timestamp, "GET", path)

			req := httptest.NewRequest("GET", path, nil)
			req.Header.Set("Authorization", fmt.Sprintf("MUXI-HMAC key=test-key, timestamp=%s, signature=%s", timestamp, signature))
			w := httptest.NewRecorder()

			middleware.Authenticate(handler).ServeHTTP(w, req)

			resp := w.Result()
			if resp.StatusCode != http.StatusOK {
				t.Errorf("Path %s: Status = %d, want %d", path, resp.StatusCode, http.StatusOK)
			}
		})
	}
}

func TestMiddleware_SignatureMustMatchPath(t *testing.T) {
	logger := zerolog.Nop()
	cfg := &config.AuthConfig{
		Enabled:            true,
		Key:                "test-key",
		Secret:             "test-secret",
		TimestampTolerance: 300,
	}

	middleware := NewMiddleware(cfg, &logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	// Sign for /wrong-path but request /test
	signature := ComputeHMAC("test-secret", timestamp, "GET", "/wrong-path")

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", fmt.Sprintf("MUXI-HMAC key=test-key, timestamp=%s, signature=%s", timestamp, signature))
	w := httptest.NewRecorder()

	middleware.Authenticate(handler).ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Status = %d, want %d (signature doesn't match path)", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestMiddleware_SignatureMustMatchMethod(t *testing.T) {
	logger := zerolog.Nop()
	cfg := &config.AuthConfig{
		Enabled:            true,
		Key:                "test-key",
		Secret:             "test-secret",
		TimestampTolerance: 300,
	}

	middleware := NewMiddleware(cfg, &logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	// Sign for POST but request GET
	signature := ComputeHMAC("test-secret", timestamp, "POST", "/test")

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", fmt.Sprintf("MUXI-HMAC key=test-key, timestamp=%s, signature=%s", timestamp, signature))
	w := httptest.NewRecorder()

	middleware.Authenticate(handler).ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Status = %d, want %d (signature doesn't match method)", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestRespondError(t *testing.T) {
	tests := []struct {
		status  int
		message string
	}{
		{http.StatusBadRequest, "Bad request"},
		{http.StatusUnauthorized, "Unauthorized"},
		{http.StatusForbidden, "Forbidden"},
		{http.StatusNotFound, "Not found"},
		{http.StatusInternalServerError, "Internal error"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d", tt.status), func(t *testing.T) {
			w := httptest.NewRecorder()
			respondError(w, tt.status, tt.message)

			resp := w.Result()
			if resp.StatusCode != tt.status {
				t.Errorf("Status = %d, want %d", resp.StatusCode, tt.status)
			}

			if resp.Header.Get("Content-Type") != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", resp.Header.Get("Content-Type"))
			}

			body, _ := io.ReadAll(resp.Body)
			if !contains(string(body), tt.message) {
				t.Errorf("Body doesn't contain message %q: %s", tt.message, string(body))
			}
		})
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
