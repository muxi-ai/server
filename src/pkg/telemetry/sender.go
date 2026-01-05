package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	defaultEndpoint = "https://capture.muxi.org/v1/telemetry"
	sendTimeout     = 2 * time.Second
	flushInterval   = 1 * time.Hour
	schemaVersion   = 1
)

// Sender handles periodic telemetry transmission
type Sender struct {
	collector *Collector
	machineID string
	country   string
	endpoint  string
	mu        sync.Mutex
	started   bool
	stopCh    chan struct{}
}

// NewSender creates a new telemetry sender
func NewSender(collector *Collector) *Sender {
	endpoint := os.Getenv("TELEMETRY_URL")
	if endpoint == "" {
		endpoint = defaultEndpoint
	}

	return &Sender{
		collector: collector,
		machineID: GetMachineID(),
		country:   GetCountry(),
		endpoint:  endpoint,
		stopCh:    make(chan struct{}),
	}
}

// Start begins the periodic flush goroutine
func (s *Sender) Start(ctx context.Context) {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return
	}
	s.started = true
	s.mu.Unlock()

	go s.run(ctx)
}

// Stop stops the sender and performs a final flush
func (s *Sender) Stop() {
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return
	}
	s.started = false
	close(s.stopCh)
	s.mu.Unlock()

	// Final flush on shutdown
	s.flush()
}

func (s *Sender) run(ctx context.Context) {
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.flush() // Final flush on context cancellation
			return
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.flush()
		}
	}
}

func (s *Sender) flush() {
	if !IsEnabled() {
		log.Debug().Msg("Telemetry disabled, skipping flush")
		return
	}

	payload := s.collector.Snapshot()

	event := Event{
		Module:        "server",
		MachineID:     s.machineID,
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		Country:       s.country,
		SchemaVersion: schemaVersion,
		Payload:       payload,
	}

	if err := s.send(event); err != nil {
		// Send failed - keep accumulating, don't reset
		log.Debug().Err(err).Msg("Telemetry send failed, will retry next interval")
		return
	}

	// Success - reset counters
	s.collector.Reset()
	log.Debug().Msg("Telemetry sent successfully")
}

func (s *Sender) send(event Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: sendTimeout}

	// First attempt
	if err := s.doSend(client, data); err != nil {
		log.Debug().Err(err).Msg("Telemetry send failed, retrying in 5 seconds")

		// Wait 5 seconds and retry once
		time.Sleep(5 * time.Second)

		if err := s.doSend(client, data); err != nil {
			// Give up - will try again next hour
			return err
		}
	}

	return nil
}

func (s *Sender) doSend(client *http.Client, data []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), sendTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &httpError{StatusCode: resp.StatusCode}
	}

	return nil
}

type httpError struct {
	StatusCode int
}

func (e *httpError) Error() string {
	return "telemetry server returned non-2xx status"
}
