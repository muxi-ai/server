// Package hfcache pre-populates the HuggingFace-style model cache during
// `muxi-server init` so the first formation deploy doesn't stall on a
// multi-hundred-megabyte model download.
//
// Why a new package: the model download is conceptually tied to
// muxi-server bootstrap (init time, runs once, best-effort), separate
// from the runtime-SIF download in pkg/runtime. Keeping it isolated
// means the rest of the server stays unaware of HuggingFace specifics.
//
// Layout on disk — we deliberately DO NOT replicate HuggingFace's
// full hub cache layout (hub/models--<org>--<model>/snapshots/<sha>/
// with blobs + symlinks + refs). That adds ~150 lines of filesystem
// choreography for no practical gain: the consuming runtime can point
// its model loader at a flat directory just as easily, and a flat dir
// survives filesystems that don't support symlinks (Windows without
// Developer Mode, some network-mounted volumes).
//
// Cache shape:
//
//	<cacheDir>/<org>--<model>/
//	    config.json
//	    tokenizer.json
//	    ...
//	    onnx/
//	        model.onnx
//
// The runtime SIF is expected to bind-mount <cacheDir> at /opt/hf-cache
// and load the model from /opt/hf-cache/<org>--<model>/.
package hfcache

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// HFBaseURL is the HuggingFace Hub "resolve" endpoint prefix. Using the
// resolve URL (not the blob URL) means LFS-hosted files redirect to the
// appropriate CDN automatically — Go's default http.Client follows
// redirects, so no special handling is needed.
//
// Exposed as a var (not const) so tests can point it at httptest.NewServer
// without needing DI plumbing through every caller. Production code must
// not mutate this; the runtime-safety assumption is "set once at import".
var HFBaseURL = "https://huggingface.co"

// LeanEmbeddingModel is the default ONNX embedding model preloaded during
// `muxi-server init`. Must match whatever the lean runtime SIF loads at
// startup by default; drifting these apart would defeat the point of
// pre-populating the cache.
const LeanEmbeddingModel = "nomic-ai/nomic-embed-text-v1.5"

// leanModelFiles is the minimal set the ONNX runtime needs to tokenize and
// run inference against nomic-embed-text-v1.5.
//
// Deliberately excludes:
//   - pytorch_model.bin (~270MB) — not used by the ONNX runtime
//   - onnx/model_{bnb4,fp16,q4,uint8,quantized}.onnx — alternatives to
//     the primary onnx/model.onnx; the runtime picks one, no need to
//     ship all of them
//   - README.md, .gitattributes — documentation, not functional
//
// If the runtime ever needs a different subset, update this slice; the
// download logic itself is list-agnostic.
var leanModelFiles = []string{
	"config.json",
	"tokenizer.json",
	"tokenizer_config.json",
	"special_tokens_map.json",
	"vocab.txt",
	"sentence_bert_config.json",
	"modules.json",
	"1_Pooling/config.json",
	"config_sentence_transformers.json",
	"onnx/model.onnx",
}

// defaultTimeout bounds the total time each file download can take.
// 10 minutes comfortably fits a ~300MB file on a 5Mbps link with
// headroom for CDN hiccups, and short-circuits a wedged connection
// before it blocks init forever.
const defaultTimeout = 10 * time.Minute

// EnsureLeanModel downloads the default lean embedding model into cacheDir.
// Wraps EnsureModel with the baked-in repoID + file list so callers
// (currently just cmdInit) don't need to know HuggingFace specifics.
//
// Best-effort by contract: the caller is expected to log the error and
// continue — the runtime inside the SIF still downloads the model on
// first use if the cache is empty, so a failure here is a slow first
// deploy, not a broken install.
func EnsureLeanModel(cacheDir string, progress io.Writer) error {
	return EnsureModel(cacheDir, LeanEmbeddingModel, leanModelFiles, progress)
}

// EnsureModel is the generic downloader. Every file in `files` is fetched
// from HF's resolve endpoint for `repoID` and written to
// <cacheDir>/<org>--<model>/<file>, preserving subdirectories (so
// "onnx/model.onnx" lands at <modelDir>/onnx/model.onnx).
//
// Idempotent: existing files with non-zero size are trusted and skipped.
// Corruption-recovery is the operator's responsibility (delete the dir
// and re-run init); implementing full checksum verification here would
// require an extra HEAD request per file and would complicate init for
// a cold path.
//
// Progress: if `progress` is non-nil, each file's transfer is tee'd to
// it via io.MultiWriter so callers can show a progress bar. Nil is
// silent, which the tests rely on to stay quiet.
func EnsureModel(cacheDir, repoID string, files []string, progress io.Writer) error {
	modelDir := filepath.Join(cacheDir, safeModelName(repoID))
	if err := os.MkdirAll(modelDir, 0755); err != nil {
		return fmt.Errorf("create model dir %q: %w", modelDir, err)
	}

	for _, rel := range files {
		dest := filepath.Join(modelDir, rel)

		// Create subdirectories lazily so entries like "onnx/model.onnx"
		// or "1_Pooling/config.json" don't fail on a missing parent.
		if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			return fmt.Errorf("create subdir for %q: %w", rel, err)
		}

		url := fmt.Sprintf("%s/%s/resolve/main/%s", HFBaseURL, repoID, rel)
		if err := downloadFileIfMissing(url, dest, progress); err != nil {
			return fmt.Errorf("download %q: %w", rel, err)
		}
	}
	return nil
}

// safeModelName converts "nomic-ai/nomic-embed-text-v1.5" to
// "nomic-ai--nomic-embed-text-v1.5". Matches HuggingFace's own "models--"
// convention (sans prefix) and guarantees a path-safe directory name on
// all filesystems — slashes in model IDs would otherwise create an
// unintended nested directory.
func safeModelName(repoID string) string {
	return strings.ReplaceAll(repoID, "/", "--")
}

// downloadFileIfMissing fetches url -> dest unless dest already exists
// with non-zero size. Writes to a .tmp sibling and renames on success
// so a partial file from a killed init never gets trusted on re-run.
func downloadFileIfMissing(url, dest string, progress io.Writer) error {
	if stat, err := os.Stat(dest); err == nil && stat.Size() > 0 {
		return nil
	}

	client := &http.Client{Timeout: defaultTimeout}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("HTTP GET: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected HTTP %d from %s", resp.StatusCode, url)
	}

	tmp := dest + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}

	var writer io.Writer = out
	if progress != nil {
		writer = io.MultiWriter(out, progress)
	}

	_, copyErr := io.Copy(writer, resp.Body)
	closeErr := out.Close()
	if copyErr != nil {
		os.Remove(tmp)
		return fmt.Errorf("write: %w", copyErr)
	}
	if closeErr != nil {
		os.Remove(tmp)
		return fmt.Errorf("close temp: %w", closeErr)
	}

	if err := os.Rename(tmp, dest); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("rename temp: %w", err)
	}
	return nil
}
