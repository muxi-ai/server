package runtime

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
)

func TestNewDownloader(t *testing.T) {
	logger := zerolog.Nop()
	d := NewDownloader("https://example.com", "test-image:latest", "/tmp/runtimes", &logger)

	if d == nil {
		t.Fatal("NewDownloader returned nil")
	}
	if d.sifBaseURL != "https://example.com" {
		t.Errorf("sifBaseURL = %s, want https://example.com", d.sifBaseURL)
	}
	if d.runtimeRunnerImage != "test-image:latest" {
		t.Errorf("runtimeRunnerImage = %s, want test-image:latest", d.runtimeRunnerImage)
	}
}

func TestDownloader_EnsureSIF_AlreadyExists(t *testing.T) {
	// Create temp directory with a fake SIF file
	tmpDir, err := os.MkdirTemp("", "test-runtimes")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create fake SIF file
	platform := getPlatform()
	sifName := "muxi-runtime-1.0.0-" + platform + ".sif"
	sifPath := filepath.Join(tmpDir, sifName)
	if err := os.WriteFile(sifPath, []byte("fake sif"), 0755); err != nil {
		t.Fatal(err)
	}

	logger := zerolog.Nop()
	d := NewDownloader("https://example.com", "test-image:latest", tmpDir, &logger)

	// Should return existing file without downloading
	path, resolvedVer, _, err := d.EnsureSIF("1.0.0")
	if err != nil {
		t.Fatalf("EnsureSIF() error = %v", err)
	}
	if path != sifPath {
		t.Errorf("EnsureSIF() path = %s, want %s", path, sifPath)
	}
	if resolvedVer != "1.0.0" {
		t.Errorf("EnsureSIF() version = %s, want 1.0.0", resolvedVer)
	}
}

func TestDownloader_EnsureSIF_Download(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("fake sif content"))
	}))
	defer server.Close()

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "test-runtimes")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	logger := zerolog.Nop()
	d := NewDownloader(server.URL, "test-image:latest", tmpDir, &logger)

	// Should download the file
	path, _, _, err := d.EnsureSIF("1.0.0")
	if err != nil {
		t.Fatalf("EnsureSIF() error = %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("Downloaded SIF file does not exist")
	}

	// Verify content
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "fake sif content" {
		t.Errorf("SIF content = %s, want 'fake sif content'", string(content))
	}
}

func TestDownloader_EnsureSIF_DownloadFails(t *testing.T) {
	// Create test server that returns 404
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "test-runtimes")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	logger := zerolog.Nop()
	d := NewDownloader(server.URL, "test-image:latest", tmpDir, &logger)

	// Should fail
	_, _, _, err = d.EnsureSIF("1.0.0")
	if err == nil {
		t.Error("EnsureSIF() should have failed for 404 response")
	}
}

func TestDownloader_EnsureSIF_DirectURL(t *testing.T) {
	// Create test server
	var requestedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("fake sif"))
	}))
	defer server.Close()

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "test-runtimes")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Use direct URL (not GitHub)
	logger := zerolog.Nop()
	d := NewDownloader(server.URL, "test-image:latest", tmpDir, &logger)

	_, _, _, err = d.EnsureSIF("1.0.0")
	if err != nil {
		t.Fatalf("EnsureSIF() error = %v", err)
	}

	// Verify URL format is direct (no version prefix)
	platform := getPlatform()
	expectedPath := "/muxi-runtime-1.0.0-" + platform + ".sif"
	if requestedPath != expectedPath {
		t.Errorf("Requested path = %s, want %s", requestedPath, expectedPath)
	}
}

func TestDownloader_InvalidURL(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-runtimes")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	logger := zerolog.Nop()
	d := NewDownloader("not-a-url", "test-image:latest", tmpDir, &logger)

	_, _, _, err = d.EnsureSIF("1.0.0")
	if err == nil {
		t.Error("EnsureSIF() should have failed for invalid URL")
	}
}

// TestDownloader_EnsureSIFForVariant_AlreadyExists proves the variant-aware
// lookup finds a pre-existing SIF named with the variant-before-platform
// convention (not the lean filename).
func TestDownloader_EnsureSIFForVariant_AlreadyExists(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-runtimes")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	platform := getPlatform()
	// Pytorch variant filename: variant segment BEFORE platform.
	sifName := "muxi-runtime-1.0.0-pytorch-" + platform + ".sif"
	sifPath := filepath.Join(tmpDir, sifName)
	if err := os.WriteFile(sifPath, []byte("fake sif"), 0755); err != nil {
		t.Fatal(err)
	}

	logger := zerolog.Nop()
	d := NewDownloader("https://example.com", "test-image:latest", tmpDir, &logger)

	path, resolvedVer, downloaded, err := d.EnsureSIFForVariant("1.0.0", "pytorch")
	if err != nil {
		t.Fatalf("EnsureSIFForVariant() error = %v", err)
	}
	if path != sifPath {
		t.Errorf("path = %q, want %q", path, sifPath)
	}
	if resolvedVer != "1.0.0" {
		t.Errorf("version = %q, want 1.0.0", resolvedVer)
	}
	if downloaded {
		t.Error("downloaded = true, want false (file pre-existed)")
	}
}

// TestDownloader_EnsureSIFForVariant_Download proves variant-aware download
// constructs the correct URL path (variant segment BEFORE platform) and
// writes to the correct on-disk location.
func TestDownloader_EnsureSIFForVariant_Download(t *testing.T) {
	var requestedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("fake pytorch sif"))
	}))
	defer server.Close()

	tmpDir, err := os.MkdirTemp("", "test-runtimes")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	logger := zerolog.Nop()
	d := NewDownloader(server.URL, "test-image:latest", tmpDir, &logger)

	path, _, downloaded, err := d.EnsureSIFForVariant("1.0.0", "pytorch")
	if err != nil {
		t.Fatalf("EnsureSIFForVariant() error = %v", err)
	}
	if !downloaded {
		t.Error("downloaded = false, want true")
	}

	platform := getPlatform()
	wantPath := "/muxi-runtime-1.0.0-pytorch-" + platform + ".sif"
	if requestedPath != wantPath {
		t.Errorf("requested URL path = %q, want %q", requestedPath, wantPath)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("downloaded SIF missing at %q: %v", path, err)
	}
}

// TestDownloader_EnsureSIF_DelegatesToLean proves the back-compat shim
// (variant-less EnsureSIF) produces a result indistinguishable from the
// variant-aware call with DefaultVariant — any divergence would silently
// break existing callers once variants land.
func TestDownloader_EnsureSIF_DelegatesToLean(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-runtimes")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Pre-create the lean-flavored SIF so both calls short-circuit on
	// "already exists" and we only compare path + resolvedVer.
	platform := getPlatform()
	sifName := "muxi-runtime-1.0.0-" + platform + ".sif"
	sifPath := filepath.Join(tmpDir, sifName)
	if err := os.WriteFile(sifPath, []byte("fake"), 0755); err != nil {
		t.Fatal(err)
	}

	logger := zerolog.Nop()
	d := NewDownloader("https://example.com", "test-image:latest", tmpDir, &logger)

	pathOld, verOld, _, err := d.EnsureSIF("1.0.0")
	if err != nil {
		t.Fatalf("EnsureSIF() error = %v", err)
	}
	pathNew, verNew, _, err := d.EnsureSIFForVariant("1.0.0", DefaultVariant)
	if err != nil {
		t.Fatalf("EnsureSIFForVariant() error = %v", err)
	}
	if pathOld != pathNew || verOld != verNew {
		t.Errorf("delegation drift: old=(%q, %q) new=(%q, %q)",
			pathOld, verOld, pathNew, verNew)
	}
}
