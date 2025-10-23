package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleDocs(t *testing.T) {
	// Create test server
	s := createTestServer(t)

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/docs", nil)
	w := httptest.NewRecorder()

	// Handle request
	s.router.ServeHTTP(w, req)

	// Check status code
	if w.Code != http.StatusFound {
		t.Errorf("expected status %d, got %d", http.StatusFound, w.Code)
	}

	// Check redirect location
	location := w.Header().Get("Location")
	expectedLocation := "https://muxi.org/docs"
	if location != expectedLocation {
		t.Errorf("expected redirect to %s, got %s", expectedLocation, location)
	}
}
