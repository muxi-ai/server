package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/muxi-ai/server/pkg/registry"
)

func TestHandleServerStatus(t *testing.T) {
	t.Run("server status with no formations", func(t *testing.T) {
		server := createTestServer(t)

		req := httptest.NewRequest("GET", "/rpc/server/status", nil)
		w := httptest.NewRecorder()

		server.HandleServerStatus(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusOK)
		}

		body, _ := io.ReadAll(resp.Body)
		var result SuccessResponse
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if !result.Success {
			t.Error("Server status should return success=true")
		}

		// Check status structure
		data, ok := result.Data.(map[string]interface{})
		if !ok {
			t.Fatal("Data is not a map")
		}

		// Check server info
		if _, ok := data["server"]; !ok {
			t.Error("Missing 'server' field")
		}

		// Check formations info
		formations, ok := data["formations"].(map[string]interface{})
		if !ok {
			t.Fatal("Missing or invalid 'formations' field")
		}

		// Should have 0 total formations
		if total, ok := formations["total"].(float64); ok {
			if total != 0 {
				t.Errorf("Total formations = %.0f, want 0", total)
			}
		}

		// Check port_pool info
		if _, ok := data["port_pool"]; !ok {
			t.Error("Missing 'port_pool' field")
		}

		// Check runtime info
		if _, ok := data["runtime"]; !ok {
			t.Error("Missing 'runtime' field")
		}
	})

	t.Run("server status with formations", func(t *testing.T) {
		server := createTestServer(t)

		// Register test formations with different statuses
		server.registry.Register(&registry.Formation{
			ID:     "running-1",
			Port:   8080,
			Status: "running",
		})
		server.registry.Register(&registry.Formation{
			ID:     "running-2",
			Port:   8081,
			Status: "running",
		})
		server.registry.Register(&registry.Formation{
			ID:     "stopped-1",
			Port:   8082,
			Status: "stopped",
		})
		server.registry.Register(&registry.Formation{
			ID:     "crashed-1",
			Port:   8083,
			Status: "crashed",
		})

		req := httptest.NewRequest("GET", "/rpc/server/status", nil)
		w := httptest.NewRecorder()

		server.HandleServerStatus(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusOK)
		}

		body, _ := io.ReadAll(resp.Body)
		var result SuccessResponse
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		data, ok := result.Data.(map[string]interface{})
		if !ok {
			t.Fatal("Data is not a map")
		}

		// Check formations counts
		formations, ok := data["formations"].(map[string]interface{})
		if !ok {
			t.Fatal("Missing or invalid 'formations' field")
		}

		// Total should be 4
		if total, ok := formations["total"].(float64); ok {
			if total != 4 {
				t.Errorf("Total formations = %.0f, want 4", total)
			}
		}

		// Running should be 2
		if running, ok := formations["running"].(float64); ok {
			if running != 2 {
				t.Errorf("Running formations = %.0f, want 2", running)
			}
		}

		// Stopped should be 1
		if stopped, ok := formations["stopped"].(float64); ok {
			if stopped != 1 {
				t.Errorf("Stopped formations = %.0f, want 1", stopped)
			}
		}

		// Crashed should be 1
		if crashed, ok := formations["crashed"].(float64); ok {
			if crashed != 1 {
				t.Errorf("Crashed formations = %.0f, want 1", crashed)
			}
		}
	})

	t.Run("server status includes server ID", func(t *testing.T) {
		server := createTestServer(t)

		req := httptest.NewRequest("GET", "/rpc/server/status", nil)
		w := httptest.NewRecorder()

		server.HandleServerStatus(w, req)

		resp := w.Result()
		body, _ := io.ReadAll(resp.Body)
		var result SuccessResponse
		json.Unmarshal(body, &result)

		data, ok := result.Data.(map[string]interface{})
		if !ok {
			t.Fatal("Data is not a map")
		}

		serverInfo, ok := data["server"].(map[string]interface{})
		if !ok {
			t.Fatal("Missing or invalid 'server' field")
		}

		// Should have server ID
		if _, ok := serverInfo["id"]; !ok {
			t.Error("Missing server ID")
		}

		// Should have version
		if version, ok := serverInfo["version"].(string); ok {
			if version == "" {
				t.Error("Server version is empty")
			}
		}

		// Should have uptime
		if uptime, ok := serverInfo["uptime"].(float64); ok {
			if uptime < 0 {
				t.Error("Server uptime should be >= 0")
			}
		}
	})

	t.Run("server status includes port pool info", func(t *testing.T) {
		server := createTestServer(t)

		// Allocate some ports
		server.registry.Register(&registry.Formation{
			ID:   "test-1",
			Port: 8080,
		})
		server.registry.Register(&registry.Formation{
			ID:   "test-2",
			Port: 8081,
		})

		req := httptest.NewRequest("GET", "/rpc/server/status", nil)
		w := httptest.NewRecorder()

		server.HandleServerStatus(w, req)

		resp := w.Result()
		body, _ := io.ReadAll(resp.Body)
		var result SuccessResponse
		json.Unmarshal(body, &result)

		data, ok := result.Data.(map[string]interface{})
		if !ok {
			t.Fatal("Data is not a map")
		}

		portPool, ok := data["port_pool"].(map[string]interface{})
		if !ok {
			t.Fatal("Missing or invalid 'port_pool' field")
		}

		// Should have total
		if total, ok := portPool["total"].(float64); ok {
			if total <= 0 {
				t.Error("Port pool total should be > 0")
			}
		}

		// Should have available
		if available, ok := portPool["available"].(float64); ok {
			if available < 0 {
				t.Error("Port pool available should be >= 0")
			}
		}

		// Should have allocated (should be 2 from our test formations)
		if allocated, ok := portPool["allocated"].(float64); ok {
			if allocated < 2 {
				t.Errorf("Port pool allocated = %.0f, want >= 2", allocated)
			}
		}
	})

	t.Run("server status includes runtime info", func(t *testing.T) {
		server := createTestServer(t)

		req := httptest.NewRequest("GET", "/rpc/server/status", nil)
		w := httptest.NewRecorder()

		server.HandleServerStatus(w, req)

		resp := w.Result()
		body, _ := io.ReadAll(resp.Body)
		var result SuccessResponse
		json.Unmarshal(body, &result)

		data, ok := result.Data.(map[string]interface{})
		if !ok {
			t.Fatal("Data is not a map")
		}

		runtimeInfo, ok := data["runtime"].(map[string]interface{})
		if !ok {
			t.Fatal("Missing or invalid 'runtime' field")
		}

		// Should have goroutines count
		if goroutines, ok := runtimeInfo["goroutines"].(float64); ok {
			if goroutines <= 0 {
				t.Error("Goroutines count should be > 0")
			}
		}

		// Should have Go version
		if goVersion, ok := runtimeInfo["go_version"].(string); ok {
			if goVersion == "" {
				t.Error("Go version is empty")
			}
		}
	})

	t.Run("server status content type is JSON", func(t *testing.T) {
		server := createTestServer(t)

		req := httptest.NewRequest("GET", "/rpc/server/status", nil)
		w := httptest.NewRecorder()

		server.HandleServerStatus(w, req)

		resp := w.Result()
		contentType := resp.Header.Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", contentType)
		}
	})
}
