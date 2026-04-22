package runtime

import (
	"strings"
	"testing"
)

func TestParseMuxiRuntime(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantVersion string
		wantVariant string
		wantErr     bool
		errContains string
	}{
		// --- Normalization table from S1 ---
		{
			name:        "empty defaults to latest lean",
			input:       "",
			wantVersion: "latest",
			wantVariant: "lean",
		},
		{
			name:        "bare latest defaults to lean",
			input:       "latest",
			wantVersion: "latest",
			wantVariant: "lean",
		},
		{
			name:        "bare calver defaults to lean",
			input:       "0.20260422.0",
			wantVersion: "0.20260422.0",
			wantVariant: "lean",
		},
		{
			name:        "latest with pytorch variant",
			input:       "latest:pytorch",
			wantVersion: "latest",
			wantVariant: "pytorch",
		},
		{
			name:        "calver with pytorch variant",
			input:       "0.20260422.0:pytorch",
			wantVersion: "0.20260422.0",
			wantVariant: "pytorch",
		},
		{
			name:        "latest with unknown variant rejected with allowlist",
			input:       "latest:unknown",
			wantErr:     true,
			errContains: "allowed variants",
		},

		// --- Back-compat: existing formations keep working ---
		{
			name:        "semver three-part passes through",
			input:       "1.2.3",
			wantVersion: "1.2.3",
			wantVariant: "lean",
		},
		{
			name:        "semver two-part passes through",
			input:       "1.2",
			wantVersion: "1.2",
			wantVariant: "lean",
		},
		{
			name:        "single digit version passes through",
			input:       "1",
			wantVersion: "1",
			wantVariant: "lean",
		},
		{
			name:        "zero-prefixed version passes through",
			input:       "0.2025.0",
			wantVersion: "0.2025.0",
			wantVariant: "lean",
		},
		{
			name:        "semver with explicit lean",
			input:       "1.2.3:lean",
			wantVersion: "1.2.3",
			wantVariant: "lean",
		},
		{
			name:        "semver with pytorch",
			input:       "1.2.3:pytorch",
			wantVersion: "1.2.3",
			wantVariant: "pytorch",
		},

		// --- Explicit default variant ---
		{
			name:        "latest with explicit lean variant",
			input:       "latest:lean",
			wantVersion: "latest",
			wantVariant: "lean",
		},

		// --- Reserved-but-not-built variants must fail at allowlist ---
		{
			name:        "reserved gpu variant rejected until SIF ships",
			input:       "latest:gpu",
			wantErr:     true,
			errContains: "allowed variants",
		},
		{
			name:        "reserved pytorch-gpu variant rejected until SIF ships",
			input:       "latest:pytorch-gpu",
			wantErr:     true,
			errContains: "allowed variants",
		},

		// --- Shape errors (regex mismatch) ---
		{
			name:        "empty variant after colon",
			input:       "latest:",
			wantErr:     true,
			errContains: "expected",
		},
		{
			name:        "uppercase in variant rejected",
			input:       "latest:Pytorch",
			wantErr:     true,
			errContains: "expected",
		},
		{
			name:        "variant starting with digit rejected",
			input:       "latest:1pytorch",
			wantErr:     true,
			errContains: "expected",
		},
		{
			name:        "variant starting with hyphen rejected",
			input:       "latest:-pytorch",
			wantErr:     true,
			errContains: "expected",
		},
		{
			name:        "multiple colons rejected",
			input:       "latest:pytorch:extra",
			wantErr:     true,
			errContains: "expected",
		},
		{
			name:        "leading whitespace rejected",
			input:       " latest",
			wantErr:     true,
			errContains: "expected",
		},
		{
			name:        "trailing whitespace rejected",
			input:       "latest ",
			wantErr:     true,
			errContains: "expected",
		},
		{
			name:        "embedded whitespace rejected",
			input:       "latest :pytorch",
			wantErr:     true,
			errContains: "expected",
		},
		{
			name:        "garbage rejected",
			input:       "garbage",
			wantErr:     true,
			errContains: "expected",
		},
		{
			name:        "alphabetic version rejected",
			input:       "v1.2.3",
			wantErr:     true,
			errContains: "expected",
		},
		{
			name:        "trailing dot rejected",
			input:       "1.2.3.",
			wantErr:     true,
			errContains: "expected",
		},
		{
			name:        "double dot rejected",
			input:       "1..3",
			wantErr:     true,
			errContains: "expected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotVersion, gotVariant, err := ParseMuxiRuntime(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseMuxiRuntime(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if tt.wantErr {
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("ParseMuxiRuntime(%q) error = %q, want substring %q",
						tt.input, err.Error(), tt.errContains)
				}
				// Error path must return zero values for version/variant to
				// prevent callers from accidentally using partial parses.
				if gotVersion != "" || gotVariant != "" {
					t.Errorf("ParseMuxiRuntime(%q) on error returned non-empty (version=%q, variant=%q)",
						tt.input, gotVersion, gotVariant)
				}
				return
			}
			if gotVersion != tt.wantVersion {
				t.Errorf("ParseMuxiRuntime(%q) version = %q, want %q",
					tt.input, gotVersion, tt.wantVersion)
			}
			if gotVariant != tt.wantVariant {
				t.Errorf("ParseMuxiRuntime(%q) variant = %q, want %q",
					tt.input, gotVariant, tt.wantVariant)
			}
		})
	}
}

// TestParseMuxiRuntime_DefaultVariantInAllowlist is a regression guard: the
// default variant MUST be present in the allowlist, otherwise every call to
// ParseMuxiRuntime with no variant (the common case) would fail the final
// allowlist check.
func TestParseMuxiRuntime_DefaultVariantInAllowlist(t *testing.T) {
	if _, ok := ValidVariants[DefaultVariant]; !ok {
		t.Fatalf("DefaultVariant %q missing from ValidVariants %v",
			DefaultVariant, ValidVariants)
	}
}

// TestAllowedVariantsList_Stable locks in the error-message format so that
// adding a variant to the allowlist in the future forces an explicit update
// to this test — catches accidental vs intentional changes to user-facing
// error text.
func TestAllowedVariantsList_Stable(t *testing.T) {
	got := allowedVariantsList()
	want := `"lean", "pytorch"`
	if got != want {
		t.Errorf("allowedVariantsList() = %q, want %q", got, want)
	}
}

// FuzzParseMuxiRuntime exercises the regex with random input. The function
// must never panic, and any success must be round-trippable against the
// normalization contract (version + variant reconstructs a valid form).
func FuzzParseMuxiRuntime(f *testing.F) {
	seeds := []string{
		"", "latest", "latest:pytorch", "0.20260422.0", "0.20260422.0:pytorch",
		"1.2.3", "1.2.3:lean", "latest:unknown", "latest:", "garbage",
		" latest", "latest ", "latest:Pytorch", "a:b:c", "::::",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		v, variant, err := ParseMuxiRuntime(s)
		if err != nil {
			return
		}
		// Successful parses must have both fields populated and variant must
		// be in the allowlist (never a raw pattern match without vetting).
		if v == "" {
			t.Errorf("ParseMuxiRuntime(%q) succeeded with empty version", s)
		}
		if _, ok := ValidVariants[variant]; !ok {
			t.Errorf("ParseMuxiRuntime(%q) succeeded with unvetted variant %q", s, variant)
		}
	})
}
