package updates

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// SDK GitHub repos (owner/repo format)
var sdkRepos = map[string]string{
	"go":         "muxi-ai/muxi-go",
	"python":     "muxi-ai/muxi-python",
	"typescript": "muxi-ai/muxi-typescript",
	"ruby":       "muxi-ai/muxi-ruby",
	"php":        "muxi-ai/muxi-php",
	"csharp":     "muxi-ai/muxi-csharp",
	"swift":      "muxi-ai/muxi-swift",
	"kotlin":     "muxi-ai/muxi-kotlin",
	"dart":       "muxi-ai/muxi-dart",
	"java":       "muxi-ai/muxi-java",
	"rust":       "muxi-ai/muxi-rust",
	"cpp":        "muxi-ai/muxi-cpp",
}

// githubRelease represents the GitHub API response for latest release
type githubRelease struct {
	TagName string `json:"tag_name"`
}

const (
	fetchTimeout  = 5 * time.Second
	refreshPeriod = 24 * time.Hour
)

var (
	versionCache = make(map[string]string)
	cacheMutex   sync.RWMutex
	stopCh       chan struct{}
	running      bool
	runningMutex sync.Mutex
)

// GetSDKLatest returns the cached latest version for an SDK, or empty string if not found.
func GetSDKLatest(sdk string) string {
	cacheMutex.RLock()
	defer cacheMutex.RUnlock()
	return versionCache[sdk]
}

// ParseSDKHeader parses "typescript/0.1.0" into ("typescript", "0.1.0")
func ParseSDKHeader(header string) (sdk string, version string) {
	parts := strings.SplitN(header, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return header, ""
}

// RefreshSDKVersions fetches latest versions from all SDK repos via GitHub API.
// Called on startup and periodically. Errors are logged but don't fail.
func RefreshSDKVersions() {
	log.Debug().Msg("Refreshing SDK version cache")

	client := &http.Client{Timeout: fetchTimeout}
	var wg sync.WaitGroup
	results := make(chan struct {
		sdk     string
		version string
	}, len(sdkRepos))

	for sdk, repo := range sdkRepos {
		wg.Add(1)
		go func(sdk, repo string) {
			defer wg.Done()
			version := fetchLatestRelease(client, sdk, repo)
			if version != "" {
				results <- struct {
					sdk     string
					version string
				}{sdk, version}
			}
		}(sdk, repo)
	}

	// Wait for all fetches to complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	newCache := make(map[string]string)
	for result := range results {
		newCache[result.sdk] = result.version
	}

	// Update cache
	cacheMutex.Lock()
	for sdk, version := range newCache {
		versionCache[sdk] = version
	}
	cacheMutex.Unlock()

	log.Info().Int("sdk_count", len(newCache)).Msg("SDK version cache refreshed")
}

// fetchLatestRelease fetches the latest release version from GitHub API.
func fetchLatestRelease(client *http.Client, sdk, repo string) string {
	ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
	defer cancel()

	url := "https://api.github.com/repos/" + repo + "/releases/latest"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		log.Debug().Str("sdk", sdk).Err(err).Msg("Failed to create GitHub API request")
		return ""
	}

	// GitHub API requires User-Agent header
	req.Header.Set("User-Agent", "muxi-server")
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		log.Debug().Str("sdk", sdk).Err(err).Msg("Failed to fetch latest release")
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Debug().Str("sdk", sdk).Int("status", resp.StatusCode).Msg("GitHub API returned non-200")
		return ""
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		log.Debug().Str("sdk", sdk).Err(err).Msg("Failed to decode GitHub API response")
		return ""
	}

	// Strip "v" prefix if present (e.g., "v0.1.0" → "0.1.0")
	version := strings.TrimPrefix(release.TagName, "v")
	log.Debug().Str("sdk", sdk).Str("version", version).Msg("Fetched SDK version from GitHub")
	return version
}

// StartBackgroundRefresh starts the daily version refresh goroutine.
// Safe to call multiple times - only starts once.
func StartBackgroundRefresh(ctx context.Context) {
	runningMutex.Lock()
	if running {
		runningMutex.Unlock()
		return
	}
	running = true
	stopCh = make(chan struct{})
	runningMutex.Unlock()

	// Initial refresh
	RefreshSDKVersions()

	// Background refresh loop
	go func() {
		ticker := time.NewTicker(refreshPeriod)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				RefreshSDKVersions()
			case <-stopCh:
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	log.Info().Dur("refresh_period", refreshPeriod).Msg("SDK version background refresh started")
}

// StopBackgroundRefresh stops the background refresh goroutine.
func StopBackgroundRefresh() {
	runningMutex.Lock()
	defer runningMutex.Unlock()

	if running && stopCh != nil {
		close(stopCh)
		running = false
	}
}
