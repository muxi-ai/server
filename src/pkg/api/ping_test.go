package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandlePing(t *testing.T) {
	server := createTestServer(t)

	req := httptest.NewRequest("GET", "/ping", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	body := w.Body.String()
	if body != "pong" {
		t.Errorf("Body = %q, want %q", body, "pong")
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "text/plain" {
		t.Errorf("Content-Type = %q, want text/plain", contentType)
	}
}

func TestHandlePing_NoAuth(t *testing.T) {
	server := createTestServer(t)

	// Ping should work without auth
	req := httptest.NewRequest("GET", "/ping", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Ping without auth: Status = %d, want %d", w.Code, http.StatusOK)
	}
}
