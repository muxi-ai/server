package updates

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// SDK version file URLs
var sdkVersionURLs = map[string]string{
	"go":         "https://raw.githubusercontent.com/muxi-ai/muxi-go/main/.version",
	"python":     "https://raw.githubusercontent.com/muxi-ai/muxi-python/main/.version",
	"typescript": "https://raw.githubusercontent.com/muxi-ai/muxi-typescript/main/.version",
	"ruby":       "https://raw.githubusercontent.com/muxi-ai/muxi-ruby/main/.version",
	"php":        "https://raw.githubusercontent.com/muxi-ai/muxi-php/main/.version",
	"csharp":     "https://raw.githubusercontent.com/muxi-ai/muxi-csharp/main/.version",
	"swift":      "https://raw.githubusercontent.com/muxi-ai/muxi-swift/main/.version",
	"kotlin":     "https://raw.githubusercontent.com/muxi-ai/muxi-kotlin/main/.version",
	"dart":       "https://raw.githubusercontent.com/muxi-ai/muxi-dart/main/.version",
	"java":       "https://raw.githubusercontent.com/muxi-ai/muxi-java/main/.version",
	"rust":       "https://raw.githubusercontent.com/muxi-ai/muxi-rust/main/.version",
	"cpp":        "https://raw.githubusercontent.com/muxi-ai/muxi-cpp/main/.version",
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

// RefreshSDKVersions fetches latest versions from all SDK repos.
// Called on startup and periodically. Errors are logged but don't fail.
func RefreshSDKVersions() {
	log.Debug().Msg("Refreshing SDK version cache")

	client := &http.Client{Timeout: fetchTimeout}
	var wg sync.WaitGroup
	results := make(chan struct {
		sdk     string
		version string
	}, len(sdkVersionURLs))

	for sdk, url := range sdkVersionURLs {
		wg.Add(1)
		go func(sdk, url string) {
			defer wg.Done()
			version := fetchVersion(client, sdk, url)
			if version != "" {
				results <- struct {
					sdk     string
					version string
				}{sdk, version}
			}
		}(sdk, url)
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

// fetchVersion fetches a single SDK version file.
func fetchVersion(client *http.Client, sdk, url string) string {
	ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		log.Debug().Str("sdk", sdk).Err(err).Msg("Failed to create version request")
		return ""
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Debug().Str("sdk", sdk).Err(err).Msg("Failed to fetch SDK version")
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Debug().Str("sdk", sdk).Int("status", resp.StatusCode).Msg("SDK version fetch returned non-200")
		return ""
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 100)) // Version should be tiny
	if err != nil {
		log.Debug().Str("sdk", sdk).Err(err).Msg("Failed to read SDK version response")
		return ""
	}

	version := strings.TrimSpace(string(body))
	log.Debug().Str("sdk", sdk).Str("version", version).Msg("Fetched SDK version")
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
