package updates

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestParseSDKHeader(t *testing.T) {
	tests := []struct {
		header      string
		wantSDK     string
		wantVersion string
	}{
		{"typescript/0.1.0", "typescript", "0.1.0"},
		{"python/1.2.3", "python", "1.2.3"},
		{"go/0.20260203.0", "go", "0.20260203.0"},
		{"rust", "rust", ""},
		{"", "", ""},
		{"sdk/with/slashes/1.0", "sdk", "with/slashes/1.0"},
	}

	for _, tt := range tests {
		t.Run(tt.header, func(t *testing.T) {
			sdk, version := ParseSDKHeader(tt.header)
			if sdk != tt.wantSDK {
				t.Errorf("ParseSDKHeader(%q) sdk = %q, want %q", tt.header, sdk, tt.wantSDK)
			}
			if version != tt.wantVersion {
				t.Errorf("ParseSDKHeader(%q) version = %q, want %q", tt.header, version, tt.wantVersion)
			}
		})
	}
}

func TestGetSDKLatest(t *testing.T) {
	// Clear cache
	cacheMutex.Lock()
	versionCache = make(map[string]string)
	cacheMutex.Unlock()

	// Empty cache returns empty string
	if got := GetSDKLatest("typescript"); got != "" {
		t.Errorf("GetSDKLatest(typescript) = %q, want empty", got)
	}

	// Set cache directly
	cacheMutex.Lock()
	versionCache["typescript"] = "0.2.0"
	versionCache["python"] = "1.0.0"
	cacheMutex.Unlock()

	// Now should return cached values
	if got := GetSDKLatest("typescript"); got != "0.2.0" {
		t.Errorf("GetSDKLatest(typescript) = %q, want 0.2.0", got)
	}
	if got := GetSDKLatest("python"); got != "1.0.0" {
		t.Errorf("GetSDKLatest(python) = %q, want 1.0.0", got)
	}
	if got := GetSDKLatest("unknown"); got != "" {
		t.Errorf("GetSDKLatest(unknown) = %q, want empty", got)
	}
}

func TestFetchVersion(t *testing.T) {
	t.Run("successful fetch", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("0.1.0\n"))
		}))
		defer server.Close()

		client := &http.Client{Timeout: 5 * time.Second}
		version := fetchVersion(client, "test-sdk", server.URL)

		if version != "0.1.0" {
			t.Errorf("fetchVersion() = %q, want 0.1.0", version)
		}
	})

	t.Run("404 returns empty", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		client := &http.Client{Timeout: 5 * time.Second}
		version := fetchVersion(client, "test-sdk", server.URL)

		if version != "" {
			t.Errorf("fetchVersion() = %q, want empty", version)
		}
	})

	t.Run("invalid url returns empty", func(t *testing.T) {
		client := &http.Client{Timeout: 1 * time.Second}
		version := fetchVersion(client, "test-sdk", "http://invalid.local.test:9999/not-real")

		if version != "" {
			t.Errorf("fetchVersion() = %q, want empty", version)
		}
	})

	t.Run("trims whitespace", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("  1.2.3  \n\n"))
		}))
		defer server.Close()

		client := &http.Client{Timeout: 5 * time.Second}
		version := fetchVersion(client, "test-sdk", server.URL)

		if version != "1.2.3" {
			t.Errorf("fetchVersion() = %q, want 1.2.3", version)
		}
	})
}

func TestRefreshSDKVersions(t *testing.T) {
	// Clear cache
	cacheMutex.Lock()
	versionCache = make(map[string]string)
	cacheMutex.Unlock()

	// This will try to fetch real URLs - most will fail (private repos)
	// but typescript should succeed
	RefreshSDKVersions()

	// Check that typescript was fetched (public repo)
	tsVersion := GetSDKLatest("typescript")
	if tsVersion == "" {
		t.Log("typescript version not fetched (might be rate limited or network issue)")
	} else {
		t.Logf("typescript version: %s", tsVersion)
	}
}

func TestStartStopBackgroundRefresh(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Clear state
	runningMutex.Lock()
	running = false
	runningMutex.Unlock()

	cacheMutex.Lock()
	versionCache = make(map[string]string)
	cacheMutex.Unlock()

	// Start should work
	StartBackgroundRefresh(ctx)

	// Give it time to do initial refresh
	time.Sleep(100 * time.Millisecond)

	// Should be running
	runningMutex.Lock()
	isRunning := running
	runningMutex.Unlock()

	if !isRunning {
		t.Error("Expected background refresh to be running")
	}

	// Starting again should be no-op
	StartBackgroundRefresh(ctx)

	// Stop
	StopBackgroundRefresh()

	runningMutex.Lock()
	isRunning = running
	runningMutex.Unlock()

	if isRunning {
		t.Error("Expected background refresh to be stopped")
	}

	// Stopping again should be safe
	StopBackgroundRefresh()
}
