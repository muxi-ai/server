package runtime

import (
	"path/filepath"
	"testing"
)

func TestGetPlatform(t *testing.T) {
	platform := getPlatform()
	if platform != "linux-amd64" && platform != "linux-arm64" {
		t.Errorf("unexpected platform: %s", platform)
	}
}

func TestResolver_Resolve(t *testing.T) {
	tests := []struct {
		name       string
		available  []string
		constraint string
		want       string
		wantErr    bool
	}{
		{
			name:       "exact version",
			available:  []string{"1.0.0", "1.1.0", "2.0.0"},
			constraint: "1.1.0",
			want:       "1.1.0",
			wantErr:    false,
		},
		{
			name:       "latest",
			available:  []string{"1.0.0", "1.1.0", "2.0.0"},
			constraint: "latest",
			want:       "latest",
			wantErr:    false,
		},
		{
			name:       "empty constraint",
			available:  []string{"1.0.0", "1.1.0", "2.0.0"},
			constraint: "",
			want:       "latest",
			wantErr:    false,
		},
		{
			name:       "minor constraint",
			available:  []string{"1.0.0", "1.1.0", "1.1.5", "2.0.0"},
			constraint: "1.1",
			want:       "1.1.5",
			wantErr:    false,
		},
		{
			name:       "major constraint",
			available:  []string{"1.0.0", "1.5.0", "2.0.0"},
			constraint: "1",
			want:       "1.5.0",
			wantErr:    false,
		},
		{
			name:       "version not found",
			available:  []string{"1.0.0", "2.0.0"},
			constraint: "3.0.0",
			want:       "3.0.0",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewResolver(tt.available, "/tmp/runtimes")
			got, err := r.Resolve(tt.constraint)
			if (err != nil) != tt.wantErr {
				t.Errorf("Resolve() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Resolve() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResolver_GetSIFPath(t *testing.T) {
	r := NewResolver([]string{}, "/tmp/runtimes")
	path := r.GetSIFPath("1.0.0")

	// Should contain the version and platform
	if path == "" {
		t.Error("GetSIFPath() returned empty string")
	}
}

func TestResolver_SIFPathForVariant_LeanMatchesGetSIFPath(t *testing.T) {
	// Back-compat contract: the new variant-aware method, when called with
	// DefaultVariant, must produce a path byte-identical to the legacy
	// GetSIFPath. Any divergence here would silently break every deploy
	// script or registry entry that pins the historical filename.
	r := NewResolver([]string{}, "/tmp/runtimes")

	legacy := r.GetSIFPath("1.0.0")
	variantAware := r.SIFPathForVariant("1.0.0", DefaultVariant)
	if legacy != variantAware {
		t.Errorf("lean drift: GetSIFPath=%q SIFPathForVariant(..., lean)=%q",
			legacy, variantAware)
	}
}

func TestResolver_SIFPathForVariant_PytorchInsertsVariant(t *testing.T) {
	r := NewResolver([]string{}, "/tmp/runtimes")
	lean := r.SIFPathForVariant("1.0.0", "lean")
	pytorch := r.SIFPathForVariant("1.0.0", "pytorch")

	if lean == pytorch {
		t.Errorf("lean and pytorch paths must differ, both = %q", lean)
	}
	// Variant-aware path must live under the same runtimesDir — only the
	// filename differs, so callers can still rely on directory layout.
	if filepath.Dir(lean) != filepath.Dir(pytorch) {
		t.Errorf("variant paths diverged from runtimesDir: lean=%q pytorch=%q",
			lean, pytorch)
	}
}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		version string
		major   int
		minor   int
		patch   int
		wantErr bool
	}{
		{"1.2.3", 1, 2, 3, false},
		{"0.0.0", 0, 0, 0, false},
		{"10.20.30", 10, 20, 30, false},
		{"invalid", 0, 0, 0, true},
		{"1.2", 0, 0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			major, minor, patch, err := parseVersion(tt.version)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseVersion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if major != tt.major || minor != tt.minor || patch != tt.patch {
					t.Errorf("parseVersion() = %d.%d.%d, want %d.%d.%d", major, minor, patch, tt.major, tt.minor, tt.patch)
				}
			}
		})
	}
}

func TestIsNewer(t *testing.T) {
	tests := []struct {
		name string
		a    [3]int
		b    [3]int
		want bool
	}{
		{"major greater", [3]int{2, 0, 0}, [3]int{1, 9, 9}, true},
		{"major less", [3]int{1, 0, 0}, [3]int{2, 0, 0}, false},
		{"minor greater", [3]int{1, 2, 0}, [3]int{1, 1, 9}, true},
		{"minor less", [3]int{1, 1, 0}, [3]int{1, 2, 0}, false},
		{"patch greater", [3]int{1, 1, 2}, [3]int{1, 1, 1}, true},
		{"patch less", [3]int{1, 1, 1}, [3]int{1, 1, 2}, false},
		{"equal", [3]int{1, 1, 1}, [3]int{1, 1, 1}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isNewer(tt.a[0], tt.a[1], tt.a[2], tt.b[0], tt.b[1], tt.b[2])
			if got != tt.want {
				t.Errorf("isNewer() = %v, want %v", got, tt.want)
			}
		})
	}
}
