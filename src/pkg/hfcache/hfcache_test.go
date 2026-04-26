package hfcache

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// newHFStub spins up an httptest server that mirrors HF's resolve-endpoint
// shape (/<repo>/resolve/main/<file>) and serves the provided file → body
// map. The returned counter tracks request count per path so tests can
// assert both correctness and idempotency with the same fixture.
func newHFStub(t *testing.T, files map[string]string) (*httptest.Server, *atomic.Int64) {
	t.Helper()

	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		// Strip the "<repo>/resolve/main/" prefix. The stub doesn't parse
		// the repo part — tests use a single fixture repo.
		parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/"), "/resolve/main/", 2)
		if len(parts) != 2 {
			http.Error(w, "bad path", http.StatusBadRequest)
			return
		}
		body, ok := files[parts[1]]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

// withBaseURL swaps the package-level HFBaseURL for the duration of a
// test, restoring it on cleanup. Keeps tests from stepping on each other
// when run in parallel (though we don't run them parallel here; this
// helper is defensive against future changes).
func withBaseURL(t *testing.T, url string) {
	t.Helper()
	prev := HFBaseURL
	HFBaseURL = url
	t.Cleanup(func() { HFBaseURL = prev })
}

// TestEnsureModel_DownloadsEveryRequestedFile is the happy path: all
// files in the provided list land at the right cache paths, subdirs
// preserved, content matches what the stub served.
func TestEnsureModel_DownloadsEveryRequestedFile(t *testing.T) {
	cacheDir := t.TempDir()

	fixtures := map[string]string{
		"config.json":           `{"model":"nomic"}`,
		"tokenizer.json":        `{"tok":true}`,
		"onnx/model.onnx":       "binary-placeholder",
		"1_Pooling/config.json": `{"pooling":"mean"}`,
	}
	srv, _ := newHFStub(t, fixtures)
	withBaseURL(t, srv.URL)

	files := []string{
		"config.json", "tokenizer.json", "onnx/model.onnx", "1_Pooling/config.json",
	}
	alreadyCached, err := EnsureModel(cacheDir, "nomic-ai/nomic-embed-text-v1.5", files, nil)
	if err != nil {
		t.Fatalf("EnsureModel() error = %v", err)
	}
	if alreadyCached {
		t.Errorf("alreadyCached = true on a cold cache; want false")
	}

	// Assert directory shape: HF Hub layout
	// <cache>/models--<org>--<model>/snapshots/main/<file-preserving-subdirs>.
	snapshotDir := filepath.Join(
		cacheDir,
		"models--nomic-ai--nomic-embed-text-v1.5",
		"snapshots", "main",
	)
	for relPath, wantBody := range fixtures {
		got, err := os.ReadFile(filepath.Join(snapshotDir, relPath))
		if err != nil {
			t.Errorf("missing file %q: %v", relPath, err)
			continue
		}
		if string(got) != wantBody {
			t.Errorf("body mismatch for %q: got %q, want %q", relPath, got, wantBody)
		}
	}

	// refs/main must point to the snapshot revision so
	// huggingface_hub.hf_hub_download(revision="main") resolves correctly.
	refsMain := filepath.Join(
		cacheDir, "models--nomic-ai--nomic-embed-text-v1.5", "refs", "main",
	)
	gotRef, err := os.ReadFile(refsMain)
	if err != nil {
		t.Errorf("refs/main missing: %v", err)
	} else if string(gotRef) != "main" {
		t.Errorf("refs/main = %q, want %q", gotRef, "main")
	}
}

// TestEnsureModel_IsIdempotent_SkipsExistingFiles guards the re-run case:
// running `muxi-server init` twice must not re-download. The stub hit
// counter catches any regression that would cause the second run to
// fetch files that are already cached — that'd be wasted bandwidth and
// a slow UX on every upgrade.
func TestEnsureModel_IsIdempotent_SkipsExistingFiles(t *testing.T) {
	cacheDir := t.TempDir()
	fixtures := map[string]string{
		"config.json":    `{"a":1}`,
		"tokenizer.json": `{"b":2}`,
	}
	srv, hits := newHFStub(t, fixtures)
	withBaseURL(t, srv.URL)

	files := []string{"config.json", "tokenizer.json"}
	repo := "nomic-ai/nomic-embed-text-v1.5"

	firstCached, err := EnsureModel(cacheDir, repo, files, nil)
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	if firstCached {
		t.Errorf("first run reported alreadyCached=true on cold cache")
	}
	firstRunHits := hits.Load()
	if firstRunHits != 2 {
		t.Fatalf("first run expected 2 fetches, got %d", firstRunHits)
	}

	secondCached, err := EnsureModel(cacheDir, repo, files, nil)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if !secondCached {
		t.Errorf("second run reported alreadyCached=false when all files present")
	}
	if hits.Load() != firstRunHits {
		t.Errorf("second run made %d new requests, want 0 (idempotency broken)",
			hits.Load()-firstRunHits)
	}
}

// TestEnsureModel_FullyCachedSkipsAllHTTP is the explicit guard on the
// re-init / upgrade scenario the user flagged: when every file is
// already on disk, no HTTP client should be spun up, no connection
// attempted, no stat-races run. The httptest stub's hit counter stays
// at zero across the call.
func TestEnsureModel_FullyCachedSkipsAllHTTP(t *testing.T) {
	cacheDir := t.TempDir()
	repo := "nomic-ai/nomic-embed-text-v1.5"
	files := []string{"config.json", "tokenizer.json", "onnx/model.onnx"}

	// Pre-populate the cache as if a previous init had succeeded
	// (HF Hub layout: snapshots/main/<file>).
	snapshotDir := filepath.Join(
		cacheDir,
		"models--nomic-ai--nomic-embed-text-v1.5",
		"snapshots", "main",
	)
	for _, f := range files {
		dest := filepath.Join(snapshotDir, f)
		if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dest, []byte("pretend-cached"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	srv, hits := newHFStub(t, map[string]string{})
	withBaseURL(t, srv.URL)

	cached, err := EnsureModel(cacheDir, repo, files, nil)
	if err != nil {
		t.Fatalf("EnsureModel() error = %v", err)
	}
	if !cached {
		t.Errorf("alreadyCached = false when every file was pre-populated")
	}
	if got := hits.Load(); got != 0 {
		t.Errorf("made %d HTTP requests on fully-cached path, want 0", got)
	}
}

// TestIsModelCached_SignalsMissingAndPresent covers the exported helper
// directly — useful for callers that want to render their own "already
// present" UX without invoking the downloader codepath.
func TestIsModelCached_SignalsMissingAndPresent(t *testing.T) {
	cacheDir := t.TempDir()
	repo := "o/m"
	files := []string{"config.json", "tokenizer.json"}

	// Nothing on disk → not cached.
	if IsModelCached(cacheDir, repo, files) {
		t.Errorf("IsModelCached() = true on empty cache; want false")
	}

	// One of two present → not cached.
	snapshotDir := filepath.Join(cacheDir, "models--o--m", "snapshots", "main")
	if err := os.MkdirAll(snapshotDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(snapshotDir, "config.json"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if IsModelCached(cacheDir, repo, files) {
		t.Errorf("IsModelCached() = true with only 1/2 files present; want false")
	}

	// Both present → cached.
	if err := os.WriteFile(filepath.Join(snapshotDir, "tokenizer.json"), []byte("y"), 0644); err != nil {
		t.Fatal(err)
	}
	if !IsModelCached(cacheDir, repo, files) {
		t.Errorf("IsModelCached() = false with all files present; want true")
	}

	// Zero-byte file → treated as not cached (guards the partial-write
	// edge case).
	if err := os.WriteFile(filepath.Join(snapshotDir, "tokenizer.json"), []byte{}, 0644); err != nil {
		t.Fatal(err)
	}
	if IsModelCached(cacheDir, repo, files) {
		t.Errorf("IsModelCached() = true with a 0-byte file; want false")
	}
}

// TestEnsureModel_PartialCacheResumesMissingOnly is the "init was killed
// mid-download" scenario: some files are already on disk, others aren't.
// The next run must only fetch what's missing.
func TestEnsureModel_PartialCacheResumesMissingOnly(t *testing.T) {
	cacheDir := t.TempDir()
	fixtures := map[string]string{
		"config.json":     `{"a":1}`,
		"tokenizer.json":  `{"b":2}`,
		"onnx/model.onnx": "fake-weights",
	}
	srv, hits := newHFStub(t, fixtures)
	withBaseURL(t, srv.URL)

	// Pre-create one file to simulate a partially completed prior init.
	snapshotDir := filepath.Join(
		cacheDir,
		"models--nomic-ai--nomic-embed-text-v1.5",
		"snapshots", "main",
	)
	if err := os.MkdirAll(snapshotDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(snapshotDir, "config.json"), []byte(`{"a":1}`), 0644); err != nil {
		t.Fatal(err)
	}

	files := []string{"config.json", "tokenizer.json", "onnx/model.onnx"}
	alreadyCached, err := EnsureModel(cacheDir, "nomic-ai/nomic-embed-text-v1.5", files, nil)
	if err != nil {
		t.Fatalf("EnsureModel() error = %v", err)
	}
	// Partial cache still requires work → alreadyCached must be false.
	if alreadyCached {
		t.Errorf("alreadyCached = true on partial cache; want false")
	}

	// Only 2 requests expected — config.json was already cached.
	if got := hits.Load(); got != 2 {
		t.Errorf("expected 2 requests (skipping pre-cached config.json), got %d", got)
	}
}

// TestEnsureModel_PropagatesHTTPFailure ensures a bad status code (404,
// 500, etc.) becomes an error rather than silently writing an empty or
// HTML error body into the cache — which would then be treated as valid
// by future idempotency checks.
func TestEnsureModel_PropagatesHTTPFailure(t *testing.T) {
	cacheDir := t.TempDir()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)
	withBaseURL(t, srv.URL)

	_, err := EnsureModel(cacheDir, "nonexistent/model", []string{"config.json"}, nil)
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error should mention 404, got %q", err.Error())
	}

	// Negative cache side-effect check: no .tmp leftovers.
	snapshotDir := filepath.Join(
		cacheDir, "models--nonexistent--model", "snapshots", "main",
	)
	entries, _ := os.ReadDir(snapshotDir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("leftover temp file after failed download: %s", e.Name())
		}
	}
}

// TestEnsureModel_ProgressWriterReceivesBytes verifies the optional
// io.Writer hook gets the bytes so a caller can implement a progress bar.
func TestEnsureModel_ProgressWriterReceivesBytes(t *testing.T) {
	cacheDir := t.TempDir()
	body := strings.Repeat("x", 1024)
	fixtures := map[string]string{"config.json": body}
	srv, _ := newHFStub(t, fixtures)
	withBaseURL(t, srv.URL)

	var progress bytes.Buffer
	if _, err := EnsureModel(cacheDir, "o/m", []string{"config.json"}, &progress); err != nil {
		t.Fatalf("EnsureModel() error = %v", err)
	}
	if progress.Len() != len(body) {
		t.Errorf("progress writer got %d bytes, want %d", progress.Len(), len(body))
	}
}

// TestSafeModelName_FlatPathSafety prevents a regression where a repoID
// containing a slash would escape the cache-dir by creating a nested
// path. Flattening with "--" is the invariant we rely on.
func TestSafeModelName_FlatPathSafety(t *testing.T) {
	cases := []struct {
		repoID string
		want   string
	}{
		{"nomic-ai/nomic-embed-text-v1.5", "models--nomic-ai--nomic-embed-text-v1.5"},
		{"sentence-transformers/all-MiniLM-L6-v2", "models--sentence-transformers--all-MiniLM-L6-v2"},
		{"no-slash-model", "models--no-slash-model"},
	}
	for _, tc := range cases {
		t.Run(tc.repoID, func(t *testing.T) {
			got := safeModelName(tc.repoID)
			if got != tc.want {
				t.Errorf("safeModelName(%q) = %q, want %q", tc.repoID, got, tc.want)
			}
			if strings.Contains(got, "/") || strings.Contains(got, `\`) {
				t.Errorf("safeModelName result must not contain separators: %q", got)
			}
		})
	}
}

// TestEnsureLeanModel_WiresDefaultRepoAndFiles is a smoke test that the
// public "lean" wrapper drives the generic EnsureModel with the baked-in
// constants. We can't assert the exact files without exporting the slice,
// so we check that the right model dir gets created and at least one
// known-required file (config.json) is fetched.
func TestEnsureLeanModel_WiresDefaultRepoAndFiles(t *testing.T) {
	cacheDir := t.TempDir()

	// Build a fixture covering every file in leanModelFiles so the call
	// succeeds without 404s. Content doesn't matter for this test.
	fixtures := map[string]string{}
	for _, f := range leanModelFiles {
		fixtures[f] = fmt.Sprintf("stub:%s", f)
	}
	srv, _ := newHFStub(t, fixtures)
	withBaseURL(t, srv.URL)

	if _, err := EnsureLeanModel(cacheDir, nil); err != nil {
		t.Fatalf("EnsureLeanModel() error = %v", err)
	}

	snapshotDir := filepath.Join(
		cacheDir,
		"models--nomic-ai--nomic-embed-text-v1.5",
		"snapshots", "main",
	)
	if _, err := os.Stat(filepath.Join(snapshotDir, "config.json")); err != nil {
		t.Errorf("lean model config.json missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(snapshotDir, "onnx", "model.onnx")); err != nil {
		t.Errorf("lean model onnx/model.onnx missing: %v", err)
	}
}
