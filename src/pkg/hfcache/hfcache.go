// Package hfcache pre-populates the HuggingFace-style model cache during
// `muxi-server init` so the first formation deploy doesn't stall on a
// multi-hundred-megabyte model download.
//
// Why a new package: the model download is conceptually tied to
// muxi-server bootstrap (init time, runs once, best-effort), separate
// from the runtime-SIF download in pkg/runtime. Keeping it isolated
// means the rest of the server stays unaware of HuggingFace specifics.
//
// Layout on disk — we write the standard HuggingFace Hub cache layout
// so huggingface_hub.hf_hub_download (used by onellm and every other
// HF-using library inside the runtime) can read the cache without any
// translation layer. Earlier versions of this package wrote a flat
// custom layout (<org>--<repo>/...) which required a runtime-side shim
// to project into HF Hub layout for offline embedding loads. The shim
// still ships in the runtime as a backwards-compat fallback for legacy
// flat caches, but new caches written by this package are HF Hub-native.
//
// Cache shape:
//
//	<cacheDir>/models--<org>--<repo>/
//	    refs/
//	        main          # contains the revision name (we use "main")
//	    snapshots/
//	        main/
//	            config.json
//	            tokenizer.json
//	            ...
//	            onnx/
//	                model.onnx
//
// We deliberately do NOT replicate the full HF Hub blob+symlink scheme
// (blobs/<sha> + snapshots/<sha>/file -> ../../blobs/<sha>). hf_hub_download
// happily resolves files placed directly under snapshots/<revision>/<file>
// via the refs/<revision> -> revision-name indirection, so the simpler
// layout is functionally equivalent and avoids two failure modes:
//   - filesystems without symlink support (Windows w/o Developer Mode,
//     some network-mounted volumes) breaking on the symlink layer
//   - blob deduplication subtleties (the SHA in blob names is the git
//     OID, which we'd have to fetch from the HF API per file)
//
// The runtime SIF bind-mounts <cacheDir> at /opt/hf-cache and the runtime
// sets HF_HUB_CACHE=/opt/hf-cache, so hf_hub_download finds the layout
// at /opt/hf-cache/models--<org>--<repo>/snapshots/main/<file>.
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

// MultilingualEmbeddingModel is the multilingual ONNX embedding model
// (Xenova-converted multilingual-e5-small) preloaded alongside the
// default English-centric Nomic model so formations targeting non-
// English content don't stall on a first-run download.
const MultilingualEmbeddingModel = "Xenova/multilingual-e5-small"

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

// multilingualModelFiles is the minimal set for Xenova/multilingual-e5-small.
// Xenova-converted models follow the transformers.js layout — they ship
// only what the ONNX runtime needs (no pytorch_model.bin, no
// sentence-transformers metadata files). The quantized ONNX is preferred
// because it's ~3x smaller (~120MB vs ~470MB) with negligible quality
// loss for retrieval-style use.
var multilingualModelFiles = []string{
	"config.json",
	"tokenizer.json",
	"tokenizer_config.json",
	"special_tokens_map.json",
	"onnx/model_quantized.onnx",
}

// defaultTimeout bounds the total time each file download can take.
// 10 minutes comfortably fits a ~300MB file on a 5Mbps link with
// headroom for CDN hiccups, and short-circuits a wedged connection
// before it blocks init forever.
const defaultTimeout = 10 * time.Minute

// snapshotRevision is the directory name written under snapshots/ and
// the contents of refs/main. HF Hub's standard would be the actual git
// commit SHA fetched from the HF API; we deliberately use a stable
// "main" placeholder because:
//   - hf_hub_download(revision="main") reads refs/main to get the
//     revision name and then looks at snapshots/<that-name>/, so the
//     name itself is opaque to the resolver.
//   - Using a stable name keeps the cache deterministic across runs and
//     avoids an extra HF API round-trip just to learn an opaque string.
//   - If two formations ever pin different commits of the same repo,
//     they'd collide on snapshots/main/ — but the lean download path
//     only ever fetches the default ("main") revision, so the
//     collision is by construction impossible today.
const snapshotRevision = "main"

// EnsureLeanModel downloads the default lean embedding model into cacheDir.
// Wraps EnsureModel with the baked-in repoID + file list so callers
// (currently just cmdInit) don't need to know HuggingFace specifics.
//
// Returns alreadyCached=true when every required file is already present
// with non-zero size, in which case NO HTTP calls are made. The caller
// should switch UX messaging off the flag ("already cached" vs "cached
// after download") so users upgrading a working install don't see a
// misleading "Pre-downloading..." line when nothing is actually fetched.
//
// Best-effort by contract: the caller is expected to log the error and
// continue — the runtime inside the SIF still downloads the model on
// first use if the cache is empty, so a failure here is a slow first
// deploy, not a broken install.
func EnsureLeanModel(cacheDir string, progress io.Writer) (alreadyCached bool, err error) {
	return EnsureModel(cacheDir, LeanEmbeddingModel, leanModelFiles, progress)
}

// EnsureMultilingualModel downloads the multilingual ONNX embedding model
// (Xenova/multilingual-e5-small) into cacheDir. Same best-effort contract
// as EnsureLeanModel — the caller is expected to log on failure and let
// the runtime fetch on first deploy if the cache is empty.
func EnsureMultilingualModel(cacheDir string, progress io.Writer) (alreadyCached bool, err error) {
	return EnsureModel(cacheDir, MultilingualEmbeddingModel, multilingualModelFiles, progress)
}

// IsModelCached reports whether every file in `files` already exists under
// <cacheDir>/models--<org>--<repo>/snapshots/main/ with non-zero size. Used
// as the fast-path check inside EnsureModel, and exported so callers that
// want to render a custom "skipping download, model present" UX can check
// without going through the full EnsureModel codepath.
//
// Non-zero size (rather than exact size match) is deliberate: a strict
// match would require a HEAD request per file, which adds network
// dependency to a check that's supposed to be fast and offline-safe.
// Partial files from a killed init would be left on disk looking
// "cached" under this scheme, but `.tmp` sibling writes + atomic renames
// in downloadFileIfMissing make that effectively unreachable in practice.
func IsModelCached(cacheDir, repoID string, files []string) bool {
	snapshotDir := snapshotPath(cacheDir, repoID)
	for _, rel := range files {
		stat, err := os.Stat(filepath.Join(snapshotDir, rel))
		if err != nil || stat.Size() == 0 {
			return false
		}
	}
	return true
}

// EnsureModel is the generic downloader. Every file in `files` is fetched
// from HF's resolve endpoint for `repoID` and written to
// <cacheDir>/models--<org>--<repo>/snapshots/main/<file>, preserving
// subdirectories (so "onnx/model.onnx" lands at
// <snapshotDir>/onnx/model.onnx). A `refs/main` pointer is also written
// so huggingface_hub.hf_hub_download(revision="main") can resolve the
// snapshot directory.
//
// Returns alreadyCached=true when the fast-path check passes — i.e. no
// HTTP work was needed. When false, at least one file was fetched (this
// includes the partial-cache case where some files existed and others
// didn't). Corruption-recovery is still the operator's responsibility
// (delete the dir and re-run); full checksum verification would require
// an extra HEAD per file and is disproportionate for the cold path.
//
// Progress: if `progress` is non-nil, each file's transfer is tee'd to
// it via io.MultiWriter so callers can show a progress bar. Nil is
// silent, which the tests rely on to stay quiet.
func EnsureModel(cacheDir, repoID string, files []string, progress io.Writer) (alreadyCached bool, err error) {
	// Fast path: skip the entire download machinery if every file is
	// already on disk. Critical for the re-init / upgrade path — no
	// point stat-ing then opening HTTP clients then stat-ing again.
	if IsModelCached(cacheDir, repoID, files) {
		return true, nil
	}

	snapshotDir := snapshotPath(cacheDir, repoID)
	if err := os.MkdirAll(snapshotDir, 0755); err != nil {
		return false, fmt.Errorf("create snapshot dir %q: %w", snapshotDir, err)
	}

	// Write refs/main so hf_hub_download(revision="main") can resolve
	// the snapshot directory name. The contents are the revision name
	// (snapshotRevision); HF Hub treats this as opaque, but writing it
	// idempotently keeps the cache layout self-describing.
	refsDir := filepath.Join(modelDir(cacheDir, repoID), "refs")
	if err := os.MkdirAll(refsDir, 0755); err != nil {
		return false, fmt.Errorf("create refs dir %q: %w", refsDir, err)
	}
	refsMain := filepath.Join(refsDir, "main")
	if err := os.WriteFile(refsMain, []byte(snapshotRevision), 0644); err != nil {
		return false, fmt.Errorf("write refs/main %q: %w", refsMain, err)
	}

	// One client for the whole model so Go's default transport can
	// pool TCP+TLS connections across all 10 files of the lean model.
	// A fresh client per file added a full handshake's latency to each
	// file; shared client cuts that to one handshake total on typical
	// HF CDN responses.
	client := &http.Client{Timeout: defaultTimeout}

	for _, rel := range files {
		dest := filepath.Join(snapshotDir, rel)

		// Create subdirectories lazily so entries like "onnx/model.onnx"
		// or "1_Pooling/config.json" don't fail on a missing parent.
		if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			return false, fmt.Errorf("create subdir for %q: %w", rel, err)
		}

		url := fmt.Sprintf("%s/%s/resolve/main/%s", HFBaseURL, repoID, rel)
		if err := downloadFileIfMissing(client, url, dest, progress); err != nil {
			return false, fmt.Errorf("download %q: %w", rel, err)
		}
	}
	return false, nil
}

// modelDir is the per-repo cache root, e.g.
// <cacheDir>/models--nomic-ai--nomic-embed-text-v1.5.
func modelDir(cacheDir, repoID string) string {
	return filepath.Join(cacheDir, safeModelName(repoID))
}

// snapshotPath is the directory where actual file content lands, e.g.
// <cacheDir>/models--nomic-ai--nomic-embed-text-v1.5/snapshots/main.
// Centralized so IsModelCached and EnsureModel agree on the layout.
func snapshotPath(cacheDir, repoID string) string {
	return filepath.Join(modelDir(cacheDir, repoID), "snapshots", snapshotRevision)
}

// safeModelName converts "nomic-ai/nomic-embed-text-v1.5" to
// "models--nomic-ai--nomic-embed-text-v1.5". Matches HuggingFace Hub's
// own per-repo cache directory naming exactly so hf_hub_download finds
// the cache without translation. Guarantees a path-safe directory name
// on all filesystems — slashes in model IDs would otherwise create an
// unintended nested directory.
func safeModelName(repoID string) string {
	return "models--" + strings.ReplaceAll(repoID, "/", "--")
}

// downloadFileIfMissing fetches url -> dest unless dest already exists
// with non-zero size. Writes to a .tmp sibling and renames on success
// so a partial file from a killed init never gets trusted on re-run.
//
// Takes the http.Client as a parameter so the caller can reuse a
// single client across all files in a model — Go's default transport
// then pools connections to the HF CDN, saving one TLS handshake per
// file on the hot path.
func downloadFileIfMissing(client *http.Client, url, dest string, progress io.Writer) error {
	if stat, err := os.Stat(dest); err == nil && stat.Size() > 0 {
		return nil
	}

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
