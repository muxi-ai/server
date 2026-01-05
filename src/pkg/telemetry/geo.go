package telemetry

import (
	"encoding/json"
	"net/http"
	"time"
)

// GetCountry returns the country code, cached permanently after first fetch
func GetCountry() string {
	// Check cache first
	if country := getCachedCountry(); country != "" {
		return country
	}

	// Fetch from ipapi.co
	country := fetchCountry()
	if country != "" {
		cacheCountry(country)
	}

	return country
}

func fetchCountry() string {
	client := &http.Client{Timeout: 2 * time.Second}

	resp, err := client.Get("https://ipapi.co/json/")
	if err != nil {
		return "XX"
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "XX"
	}

	var data struct {
		CountryCode string `json:"country_code"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "XX"
	}

	if data.CountryCode == "" {
		return "XX"
	}

	return data.CountryCode
}
