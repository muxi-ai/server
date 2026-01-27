package telemetry

import (
	"testing"
)

func TestNormalizeArch(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"amd64", "x86_64"},
		{"386", "i386"},
		{"arm64", "arm64"},
		{"mips", "mips"},
	}
	for _, tt := range tests {
		got := normalizeArch(tt.input)
		if got != tt.want {
			t.Errorf("normalizeArch(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestGetRAMGB(t *testing.T) {
	ram := getRAMGB()
	// On macOS/Linux should return > 0, on other platforms may return 0
	t.Logf("RAM: %d GB", ram)
}
