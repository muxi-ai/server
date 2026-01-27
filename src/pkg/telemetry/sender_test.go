package telemetry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewSender(t *testing.T) {
	c := NewCollector("1.0.0", "docker")
	s := NewSender(c)

	if s.collector != c {
		t.Error("expected sender to reference the given collector")
	}
	if s.endpoint != defaultEndpoint {
		t.Errorf("expected default endpoint, got %s", s.endpoint)
	}
}

func TestNewSender_CustomEndpoint(t *testing.T) {
	os.Setenv("TELEMETRY_URL", "http://custom.example.com/telemetry")
	defer os.Unsetenv("TELEMETRY_URL")

	c := NewCollector("1.0.0", "docker")
	s := NewSender(c)

	if s.endpoint != "http://custom.example.com/telemetry" {
		t.Errorf("expected custom endpoint, got %s", s.endpoint)
	}
}

func TestSender_StartStop(t *testing.T) {
	c := NewCollector("1.0.0", "docker")
	s := NewSender(c)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.Start(ctx)
	if !s.started {
		t.Error("expected sender to be started")
	}

	// Double start should be safe
	s.Start(ctx)

	s.Stop()
	if s.started {
		t.Error("expected sender to be stopped")
	}

	// Double stop should be safe
	s.Stop()
}

func TestSender_FlushSendsToEndpoint(t *testing.T) {
	var received atomic.Int32
	var lastEvent Event

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Add(1)
		json.NewDecoder(r.Body).Decode(&lastEvent)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	os.Setenv("TELEMETRY_URL", server.URL)
	defer os.Unsetenv("TELEMETRY_URL")

	c := NewCollector("2.0.0", "singularity")
	c.IncrementDeploy(true)
	c.IncrementCrash()

	s := NewSender(c)
	s.flush()

	if received.Load() != 1 {
		t.Errorf("expected 1 request, got %d", received.Load())
	}
	if lastEvent.Module != "server" {
		t.Errorf("expected module 'server', got %s", lastEvent.Module)
	}
	if lastEvent.SchemaVersion != schemaVersion {
		t.Errorf("expected schema version %d, got %d", schemaVersion, lastEvent.SchemaVersion)
	}

	// Collector should be reset after successful send
	snapshot := c.Snapshot()
	if snapshot.Deployments.Successful != 0 {
		t.Error("collector should be reset after successful flush")
	}
}

func TestSender_FlushDisabled(t *testing.T) {
	os.Setenv("MUXI_TELEMETRY", "0")
	defer os.Unsetenv("MUXI_TELEMETRY")

	var received atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	os.Setenv("TELEMETRY_URL", server.URL)
	defer os.Unsetenv("TELEMETRY_URL")

	c := NewCollector("1.0.0", "docker")
	c.IncrementDeploy(true)

	s := NewSender(c)
	s.flush()

	if received.Load() != 0 {
		t.Error("should not send when telemetry is disabled")
	}
}

func TestSender_FlushRetryOnFailure(t *testing.T) {
	var attempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := attempts.Add(1)
		if count <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	os.Setenv("TELEMETRY_URL", server.URL)
	defer os.Unsetenv("TELEMETRY_URL")

	c := NewCollector("1.0.0", "docker")
	c.IncrementDeploy(true)

	s := NewSender(c)

	// send() retries once after 5s -- but we just test doSend directly
	client := &http.Client{Timeout: sendTimeout}
	data, _ := json.Marshal(Event{Module: "test"})

	err := s.doSend(client, data)
	if err == nil {
		t.Error("expected error on 500 response")
	}

	httpErr, ok := err.(*httpError)
	if !ok {
		t.Errorf("expected *httpError, got %T", err)
	} else if httpErr.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", httpErr.StatusCode)
	}
}

func TestSender_DoSendSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	os.Setenv("TELEMETRY_URL", server.URL)
	defer os.Unsetenv("TELEMETRY_URL")

	c := NewCollector("1.0.0", "docker")
	s := NewSender(c)

	client := &http.Client{Timeout: sendTimeout}
	data, _ := json.Marshal(Event{Module: "test"})

	err := s.doSend(client, data)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestSender_RunContextCancel(t *testing.T) {
	var received atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	os.Setenv("TELEMETRY_URL", server.URL)
	defer os.Unsetenv("TELEMETRY_URL")

	c := NewCollector("1.0.0", "docker")
	c.IncrementDeploy(true)

	s := NewSender(c)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		s.run(ctx)
		close(done)
	}()

	// Cancel context triggers final flush
	cancel()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("run() did not exit after context cancel")
	}

	if received.Load() != 1 {
		t.Errorf("expected final flush on context cancel, got %d sends", received.Load())
	}
}

func TestHttpError_Error(t *testing.T) {
	e := &httpError{StatusCode: 503}
	if e.Error() != "telemetry server returned non-2xx status" {
		t.Errorf("unexpected error message: %s", e.Error())
	}
}
