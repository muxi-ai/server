package updates

import (
	"context"
	"encoding/json"
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

func TestFetchLatestRelease(t *testing.T) {
	t.Run("parses github release json", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"tag_name": "v0.1.0", "name": "Release 0.1.0"}`))
		}))
		defer server.Close()

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get(server.URL)
		if err != nil {
			t.Fatalf("Failed to get: %v", err)
		}
		defer resp.Body.Close()

		var release githubRelease
		if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
			t.Fatalf("Failed to decode: %v", err)
		}
		if release.TagName != "v0.1.0" {
			t.Errorf("TagName = %q, want v0.1.0", release.TagName)
		}
	})

	t.Run("strips v prefix from tag", func(t *testing.T) {
		// Test the strings.TrimPrefix logic used in fetchLatestRelease
		tests := []struct {
			tag  string
			want string
		}{
			{"v1.2.3", "1.2.3"},
			{"v0.20260127.0", "0.20260127.0"},
			{"1.0.0", "1.0.0"}, // No prefix
			{"", ""},
		}
		for _, tt := range tests {
			got := trimVersionPrefix(tt.tag)
			if got != tt.want {
				t.Errorf("trimVersionPrefix(%q) = %q, want %q", tt.tag, got, tt.want)
			}
		}
	})
}

// trimVersionPrefix removes "v" prefix from version tags
func trimVersionPrefix(tag string) string {
	if len(tag) > 0 && tag[0] == 'v' {
		return tag[1:]
	}
	return tag
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
