package main

import (
	"strings"
	"testing"
)

func TestGetArchString(t *testing.T) {
	arch := getArchString()
	if arch == "" {
		t.Error("expected non-empty arch string")
	}
	valid := map[string]bool{"x86_64": true, "arm64": true, "i386": true}
	if !valid[arch] {
		// Still valid if it's a raw GOARCH value
		t.Logf("arch string: %s (raw GOARCH)", arch)
	}
}

func TestMaskSecret(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"muxi_sk_abcdefghijklmnopqrstuvwxyz1234567890abcdefghijklmnop", "muxi_sk_...mnop"},
		{"short", "***"},
		{"12345678", "***"},
		{"123456789", "12345678...6789"},
	}
	for _, tt := range tests {
		got := maskSecret(tt.input)
		if got != tt.want {
			t.Errorf("maskSecret(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestMaskKey(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"muxi_pk_abcdef123456", "muxi_pk_abcd••••••••"},
		{"short", "muxi_pk_••••••••"},
		{"muxi_pk_abcd", "muxi_pk_••••••••"},
		{"muxi_pk_abcde", "muxi_pk_abcd••••••••"},
	}
	for _, tt := range tests {
		got := maskKey(tt.input)
		if got != tt.want {
			t.Errorf("maskKey(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestGenerateKey(t *testing.T) {
	key, err := generateKey()
	if err != nil {
		t.Fatalf("generateKey() error = %v", err)
	}
	if !strings.HasPrefix(key, "muxi_pk_") {
		t.Errorf("key should start with muxi_pk_, got %s", key)
	}
	if len(key) != 32 { // "muxi_pk_" (8) + 24 hex chars (12 bytes)
		t.Errorf("key length = %d, want 32", len(key))
	}

	// Should generate unique keys
	key2, _ := generateKey()
	if key == key2 {
		t.Error("keys should be unique")
	}
}

func TestGenerateSecret(t *testing.T) {
	secret, err := generateSecret()
	if err != nil {
		t.Fatalf("generateSecret() error = %v", err)
	}
	if !strings.HasPrefix(secret, "muxi_sk_") {
		t.Errorf("secret should start with muxi_sk_, got %s", secret)
	}
	if len(secret) != 64 { // "muxi_sk_" (8) + 56 hex chars (28 bytes)
		t.Errorf("secret length = %d, want 64", len(secret))
	}

	// Should generate unique secrets
	secret2, _ := generateSecret()
	if secret == secret2 {
		t.Error("secrets should be unique")
	}
}

func TestExtractServerName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"server-myhost-abc123", "myhost"},
		{"server-test-deadbeef", "test"},
		{"single", ""},
		{"a-b", "b"},
	}
	for _, tt := range tests {
		got := extractServerName(tt.input)
		if got != tt.want {
			t.Errorf("extractServerName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestGenerateServerIDFromName(t *testing.T) {
	id := generateServerIDFromName("myhost")
	if !strings.HasPrefix(id, "server-myhost-") {
		t.Errorf("server ID should start with server-myhost-, got %s", id)
	}

	// Should generate unique IDs
	id2 := generateServerIDFromName("myhost")
	if id == id2 {
		t.Error("server IDs should be unique (random hash)")
	}
}

func TestIsPortAvailable(t *testing.T) {
	// Port 0 tells the OS to assign a free port - but isPortAvailable checks a specific port
	// Port 19999 is very likely available
	if !isPortAvailable(19999) {
		t.Log("port 19999 not available (something else using it)")
	}

	// Port 1 should not be available (privileged, or in use)
	if isPortAvailable(1) {
		t.Log("port 1 was available (running as root?)")
	}
}

func TestVersion(t *testing.T) {
	if Version == "" {
		t.Error("Version should not be empty (embedded from .version)")
	}
}
