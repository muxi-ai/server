package runtime

import (
	"fmt"
	"strings"
	"testing"
)

func TestSIFFilename_LeanIsSuffixFree(t *testing.T) {
	// Lean is the default variant and must produce a filename with NO
	// "-lean-" segment. This is the hinge of the back-compat contract:
	// existing release artifacts on GitHub and existing SIFs on operator
	// disks all use the variant-less name, so lean must reproduce it
	// byte-for-byte.
	got := sifFilename("0.20260422.0", "lean")

	if strings.Contains(got, "-lean-") {
		t.Errorf("lean filename must be suffix-free, got %q", got)
	}
	if !strings.HasPrefix(got, "muxi-runtime-0.20260422.0-") {
		t.Errorf("filename missing expected prefix, got %q", got)
	}
	if !strings.HasSuffix(got, ".sif") {
		t.Errorf("filename missing .sif extension, got %q", got)
	}
}

func TestSIFFilename_EmptyVariantEqualsLean(t *testing.T) {
	// Defensive: even though callers route through ParseMuxiRuntime which
	// never emits an empty variant, sifFilename must still produce the
	// lean filename for an empty variant. This guards against any caller
	// that constructs paths manually and skips the parser.
	lean := sifFilename("0.20260422.0", "lean")
	empty := sifFilename("0.20260422.0", "")
	if lean != empty {
		t.Errorf("empty variant must equal lean, got lean=%q empty=%q", lean, empty)
	}
}

func TestSIFFilename_NonDefaultVariantsPrecedePlatform(t *testing.T) {
	// Variant-before-platform is the naming convention decided in the
	// interface contract — it groups variants together alphabetically on
	// disk and reads naturally (version, then build variant, then target).
	// Every live non-default variant must honor this ordering, otherwise
	// mirror URLs and on-disk layout drift between variants.
	cases := []struct {
		variant     string
		mustContain string
	}{
		{variant: "pytorch", mustContain: "-pytorch-"},
		{variant: "cuda", mustContain: "-cuda-"},
	}
	for _, tc := range cases {
		t.Run(tc.variant, func(t *testing.T) {
			got := sifFilename("0.20260422.0", tc.variant)

			vIdx := strings.Index(got, tc.mustContain)
			linuxIdx := strings.Index(got, "-linux-")
			if vIdx < 0 {
				t.Fatalf("missing %s segment in %q", tc.mustContain, got)
			}
			if linuxIdx < 0 {
				t.Fatalf("missing -linux- segment in %q", got)
			}
			if vIdx > linuxIdx {
				t.Errorf("variant must precede platform, got %q", got)
			}
		})
	}
}

func TestSIFFilename_FutureVariantsFollowSameShape(t *testing.T) {
	// Reserved-but-not-yet-shipped variants (NOT in the allowlist, so
	// ParseMuxiRuntime rejects them at the edge). The filename helper is
	// still checked here so that when these variants DO ship, the naming
	// convention is already correct — no special-casing needed later.
	cases := []struct {
		variant        string
		mustContain    string
		mustNotContain string
	}{
		{variant: "pytorch-gpu", mustContain: "-pytorch-gpu-", mustNotContain: "-lean-"},
	}
	for _, tc := range cases {
		t.Run(tc.variant, func(t *testing.T) {
			got := sifFilename("0.20260422.0", tc.variant)
			if !strings.Contains(got, tc.mustContain) {
				t.Errorf("filename missing %q, got %q", tc.mustContain, got)
			}
			if strings.Contains(got, tc.mustNotContain) {
				t.Errorf("filename contains unexpected %q, got %q", tc.mustNotContain, got)
			}
			if !strings.HasSuffix(got, ".sif") {
				t.Errorf("filename missing .sif extension, got %q", got)
			}
		})
	}
}

func TestSIFFilename_MatchesExistingResolverConvention(t *testing.T) {
	// Regression guard: the lean filename MUST byte-exactly match the
	// historical resolver convention (muxi-runtime-<version>-<platform>.sif).
	// This test pins that invariant so any future edits to sifFilename
	// that break back-compat fail loudly.
	version := "0.20260422.0"
	got := sifFilename(version, "lean")
	want := fmt.Sprintf("muxi-runtime-%s-%s.sif", version, getPlatform())
	if got != want {
		t.Errorf("lean filename drift: got %q, want %q", got, want)
	}
}

func TestSIFFilename_SemverVersionsStillWork(t *testing.T) {
	// Back-compat: semver-style versions (which some formations still
	// pin) must format identically to calver versions — the helper is
	// agnostic to version shape and just substitutes.
	cases := []string{"1.2.3", "0.2025.0", "1.0.0", "10.20.30"}
	for _, v := range cases {
		t.Run(v, func(t *testing.T) {
			got := sifFilename(v, "lean")
			if !strings.Contains(got, v) {
				t.Errorf("filename missing version %q, got %q", v, got)
			}
			if !strings.HasPrefix(got, "muxi-runtime-"+v+"-") {
				t.Errorf("filename missing expected prefix, got %q", got)
			}
		})
	}
}
