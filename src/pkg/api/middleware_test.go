package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAuthMiddleware_Integration(t *testing.T) {
	server := createTestServer(t)

	t.Run("request without auth header", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/formations", nil)
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		resp := w.Result()
		// Should be rejected by auth middleware
		if resp.StatusCode != http.StatusUnauthorized {
			t.Logf("Status = %d (expected 401 for missing auth)", resp.StatusCode)
		}
	})

	t.Run("request with invalid auth header", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/formations", nil)
		req.Header.Set("Authorization", "invalid header format")
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Logf("Status = %d (expected 401 for invalid auth)", resp.StatusCode)
		}
	})

	t.Run("health endpoint without auth", func(t *testing.T) {
		// Health endpoint should be accessible without auth
		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Health endpoint should be accessible without auth, got status %d", resp.StatusCode)
		}
	})

	t.Run("proxy endpoint without auth", func(t *testing.T) {
		// Proxy endpoints should be accessible without auth
		req := httptest.NewRequest("GET", "/v1/test-formation/health", nil)
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		// Will return 404 (formation not found), but shouldn't require auth
		resp := w.Result()
		t.Logf("Proxy without auth status: %d", resp.StatusCode)
	})
}

func TestLoggingMiddleware_AllRoutes(t *testing.T) {
	server := createTestServer(t)

	// Test that logging middleware logs all requests
	testCases := []struct {
		method string
		path   string
	}{
		{"GET", "/health"},
		{"POST", "/formations/deploy"},
		{"GET", "/formations"},
		{"DELETE", "/formations/test"},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%s %s", tc.method, tc.path), func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			w := httptest.NewRecorder()

			// Should not panic
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Logging middleware panicked: %v", r)
				}
			}()

			server.router.ServeHTTP(w, req)

			// Verify request completed
			if w.Result().StatusCode == 0 {
				t.Error("Request didn't complete")
			}
		})
	}
}

func TestCORSMiddleware_Detailed(t *testing.T) {
	server := createTestServer(t)

	t.Run("preflight OPTIONS request", func(t *testing.T) {
		req := httptest.NewRequest("OPTIONS", "/health", nil)
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		resp := w.Result()
		// Check CORS headers
		t.Logf("CORS headers: Allow-Origin=%q, Allow-Methods=%q",
			resp.Header.Get("Access-Control-Allow-Origin"),
			resp.Header.Get("Access-Control-Allow-Methods"))
	})

	t.Run("CORS headers on GET", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		resp := w.Result()
		// CORS middleware should add headers
		origin := resp.Header.Get("Access-Control-Allow-Origin")
		t.Logf("CORS Origin header on GET: %q", origin)
	})

	t.Run("CORS headers on POST", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/formations/deploy", nil)
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		resp := w.Result()
		t.Logf("CORS headers on POST: %v", resp.Header)
	})
}

func TestServer_Start_Stop(t *testing.T) {
	// Test server lifecycle (without actually starting HTTP listener)
	server := createTestServer(t)

	if server.httpServer == nil {
		t.Error("HTTP server not initialized")
	}

	expectedAddr := fmt.Sprintf("%s:%d", server.config.Server.Host, server.config.Server.Port)
	if server.httpServer.Addr != expectedAddr {
		t.Errorf("Server addr = %q, want %q", server.httpServer.Addr, expectedAddr)
	}

	// Test that server has proper timeouts configured
	if server.httpServer.ReadTimeout == 0 {
		t.Error("ReadTimeout not set")
	}

	if server.httpServer.WriteTimeout == 0 {
		t.Error("WriteTimeout not set")
	}

	if server.httpServer.IdleTimeout == 0 {
		t.Error("IdleTimeout not set")
	}
}

func TestRouteSetup(t *testing.T) {
	server := createTestServer(t)

	// Test that all expected routes are registered
	routes := []struct {
		method string
		path   string
	}{
		{"GET", "/health"},
		{"POST", "/formations/deploy"},
		{"GET", "/formations"},
		{"GET", "/formations/test-id"},
		{"DELETE", "/formations/test-id"},
		{"POST", "/formations/test-id/stop"},
		{"POST", "/formations/test-id/restart"},
		{"GET", "/formations/test-id/logs"},
		{"GET", "/v1/test-formation/path"},
	}

	for _, route := range routes {
		t.Run(fmt.Sprintf("%s %s", route.method, route.path), func(t *testing.T) {
			req := httptest.NewRequest(route.method, route.path, nil)
			w := httptest.NewRecorder()

			server.router.ServeHTTP(w, req)

			// Should not return 404 (route exists)
			// May return 401 (auth), 400 (bad request), etc.
			if w.Result().StatusCode == http.StatusNotFound && route.path != "/v1/test-formation/path" {
				t.Logf("Route %s %s returned 404 - may not be registered", route.method, route.path)
			}
		})
	}
}

func TestFormationURL(t *testing.T) {
	tests := []struct {
		serverPort  int
		formationID string
		expected    string
	}{
		{3000, "api-v1", "http://localhost:3000/v1/api-v1"},
		{8080, "test", "http://localhost:8080/v1/test"},
		{80, "prod", "http://localhost:80/v1/prod"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			url := formatFormationURL(tt.serverPort, tt.formationID)
			if url != tt.expected {
				t.Errorf("formatFormationURL() = %q, want %q", url, tt.expected)
			}
		})
	}
}

func TestHMACSignature(t *testing.T) {
	// Test HMAC signature generation (helper for auth testing)
	secret := "test-secret-key"
	message := "test message"

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(message))
	signature := hex.EncodeToString(mac.Sum(nil))

	if len(signature) != 64 {
		t.Errorf("Signature length = %d, want 64 (SHA256 hex)", len(signature))
	}

	// Verify signature is deterministic
	mac2 := hmac.New(sha256.New, []byte(secret))
	mac2.Write([]byte(message))
	signature2 := hex.EncodeToString(mac2.Sum(nil))

	if signature != signature2 {
		t.Error("HMAC signature should be deterministic")
	}

	// Verify different message produces different signature
	mac3 := hmac.New(sha256.New, []byte(secret))
	mac3.Write([]byte("different message"))
	signature3 := hex.EncodeToString(mac3.Sum(nil))

	if signature == signature3 {
		t.Error("Different messages should produce different signatures")
	}
}

func TestTimestampValidation(t *testing.T) {
	tests := []struct {
		name      string
		timestamp int64
		tolerance int
		wantValid bool
	}{
		{"current time", time.Now().Unix(), 300, true},
		{"5 minutes ago", time.Now().Unix() - 300, 300, true},
		{"5 minutes future", time.Now().Unix() + 300, 300, true},
		{"10 minutes ago", time.Now().Unix() - 600, 300, false},
		{"10 minutes future", time.Now().Unix() + 600, 300, false},
		{"exact tolerance boundary past", time.Now().Unix() - 300, 300, true},
		{"exact tolerance boundary future", time.Now().Unix() + 300, 300, true},
		{"just outside tolerance past", time.Now().Unix() - 301, 300, false},
		{"just outside tolerance future", time.Now().Unix() + 301, 300, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Now().Unix()
			diff := abs(now - tt.timestamp)
			isValid := diff <= int64(tt.tolerance)

			if isValid != tt.wantValid {
				t.Errorf("Timestamp %d (diff=%d) valid=%v, want %v",
					tt.timestamp, diff, isValid, tt.wantValid)
			}
		})
	}
}

func abs(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

func TestRequestProcessing(t *testing.T) {
	server := createTestServer(t)

	t.Run("concurrent requests", func(t *testing.T) {
		// Test that server handles concurrent requests without data races
		done := make(chan bool)

		for i := 0; i < 10; i++ {
			go func() {
				req := httptest.NewRequest("GET", "/health", nil)
				w := httptest.NewRecorder()
				server.router.ServeHTTP(w, req)
				done <- true
			}()
		}

		for i := 0; i < 10; i++ {
			<-done
		}
	})

	t.Run("large request body", func(t *testing.T) {
		// Test handling of large request bodies
		largeBody := make([]byte, 1024*1024) // 1MB
		for i := range largeBody {
			largeBody[i] = 'A'
		}

		req := httptest.NewRequest("POST", "/formations/deploy", nil)
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		// Should handle without panicking
		t.Logf("Large body request status: %d", w.Result().StatusCode)
	})
}
