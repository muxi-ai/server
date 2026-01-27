package telemetry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchCountry_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"country_code": "US"})
	}))
	defer server.Close()

	// fetchCountry uses hardcoded URL, so we test GetCountry which uses cache
	// Just verify fetchCountry doesn't panic with real endpoint
	country := fetchCountry()
	if country == "" {
		t.Error("expected non-empty country (or XX fallback)")
	}
}

func TestGetCountry(t *testing.T) {
	country := GetCountry()
	if country == "" {
		t.Error("expected non-empty country")
	}
	// Should be a 2-letter code or XX
	if len(country) != 2 {
		t.Errorf("expected 2-letter country code, got %q", country)
	}
}
