package dockerutil

import (
	"bytes"
	"strings"
	"testing"
)

// TestRenderPullProgress_CollapsesLayerLifecycleToSingleLine is the
// happy-path guard: given a transcript with N "Pulling fs layer" events
// and N "Pull complete" events, the output must contain a progress line
// ending in "Layers N/N (100%)" regardless of the verbose noise in
// between (Verifying Checksum, Download complete, Extracting…).
//
// The renderer prefixes each progress line with a spinner frame from
// SpinnerFrames. The specific frame at any point is
// scheduling-dependent (the ticker fires independently of event
// arrival), so we assert on substrings rather than exact equality.
func TestRenderPullProgress_CollapsesLayerLifecycleToSingleLine(t *testing.T) {
	transcript := strings.Join([]string{
		"latest: Pulling from muxi-ai/runtime-runner",
		"abc123: Pulling fs layer",
		"def456: Pulling fs layer",
		"ghi789: Pulling fs layer",
		"abc123: Verifying Checksum",
		"abc123: Download complete",
		"abc123: Extracting [=>] 10MB/100MB",
		"abc123: Pull complete",
		"def456: Verifying Checksum",
		"def456: Download complete",
		"def456: Pull complete",
		"ghi789: Verifying Checksum",
		"ghi789: Download complete",
		"ghi789: Pull complete",
		"Digest: sha256:deadbeef",
		"Status: Downloaded newer image for ghcr.io/muxi-ai/runtime-runner:latest",
		"ghcr.io/muxi-ai/runtime-runner:latest",
	}, "\n")

	var out bytes.Buffer
	RenderPullProgress(strings.NewReader(transcript), &out)

	got := out.String()
	if !strings.Contains(got, "Layers 3/3 (100%)") {
		t.Errorf("expected final 3/3 reading in output, got %q", got)
	}
	if !strings.HasSuffix(got, "\n") {
		t.Errorf("output must terminate with newline; got %q", got)
	}
	if !containsAnySpinnerFrame(got) {
		t.Errorf("expected at least one spinner frame in output, got %q", got)
	}
}

// TestRenderPullProgress_StaggeredLayerAnnouncements guards the case
// where Docker announces a layer only after prior layers have already
// completed — the running total grows and the percentage naturally
// drops. This is honest progress, not a regression.
func TestRenderPullProgress_StaggeredLayerAnnouncements(t *testing.T) {
	transcript := strings.Join([]string{
		"a: Pulling fs layer", // total=1, pulled=0 -> 0/1 (0%)
		"a: Pull complete",    // total=1, pulled=1 -> 1/1 (100%)
		"b: Pulling fs layer", // total=2, pulled=1 -> 1/2 (50%)
		"c: Pulling fs layer", // total=3, pulled=1 -> 1/3 (33%)
		"b: Pull complete",    // total=3, pulled=2 -> 2/3 (66%)
		"c: Pull complete",    // total=3, pulled=3 -> 3/3 (100%)
	}, "\n")

	var out bytes.Buffer
	RenderPullProgress(strings.NewReader(transcript), &out)

	got := out.String()
	if !strings.Contains(got, "Layers 1/1 (100%)") {
		t.Errorf("expected interim 1/1 reading, got %q", got)
	}
	if !strings.Contains(got, "Layers 1/3 (33%)") {
		t.Errorf("expected interim 1/3 reading after third layer announced, got %q", got)
	}
	if !strings.Contains(got, "Layers 3/3 (100%)") {
		t.Errorf("expected final 3/3 reading, got %q", got)
	}
}

// TestRenderPullProgress_ImageUpToDatePrintsNothing — when Docker finds
// the image already cached, there are no "Pulling fs layer" events, so
// we must print nothing and let the caller's success message own the
// line. Guards against a stray newline or partial progress line
// appearing in the transcript.
func TestRenderPullProgress_ImageUpToDatePrintsNothing(t *testing.T) {
	transcript := strings.Join([]string{
		"latest: Pulling from muxi-ai/runtime-runner",
		"Digest: sha256:deadbeef",
		"Status: Image is up to date for ghcr.io/muxi-ai/runtime-runner:latest",
		"ghcr.io/muxi-ai/runtime-runner:latest",
	}, "\n")

	var out bytes.Buffer
	RenderPullProgress(strings.NewReader(transcript), &out)

	if out.Len() != 0 {
		t.Errorf("expected empty output for cached image, got %q", out.String())
	}
}

// containsAnySpinnerFrame returns true if any of the defined spinner
// frames appears anywhere in s. Used to verify a spinner character was
// painted without pinning the test to a specific frame (which depends
// on ticker scheduling).
func containsAnySpinnerFrame(s string) bool {
	for _, f := range SpinnerFrames {
		if strings.Contains(s, f) {
			return true
		}
	}
	return false
}
