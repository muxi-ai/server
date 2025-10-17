package auth

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestComputeHMAC(t *testing.T) {
	tests := []struct {
		name      string
		secret    string
		timestamp string
		method    string
		path      string
	}{
		{
			name:      "Valid signature",
			secret:    "test-secret",
			timestamp: "1234567890",
			method:    "POST",
			path:      "/formations/deploy",
		},
		{
			name:      "Empty secret",
			secret:    "",
			timestamp: "1234567890",
			method:    "GET",
			path:      "/formations",
		},
		{
			name:      "Different methods",
			secret:    "test-secret",
			timestamp: "1234567890",
			method:    "DELETE",
			path:      "/formations/test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sig := ComputeHMAC(tt.secret, tt.timestamp, tt.method, tt.path)
			if sig == "" {
				t.Error("ComputeHMAC() returned empty signature")
			}
		})
	}
}

func TestCompareSignatures(t *testing.T) {
	secret := "test-secret-key-12345"
	timestamp := "1234567890"
	method := "POST"
	path := "/formations/deploy"

	// Generate a valid signature
	validSig := ComputeHMAC(secret, timestamp, method, path)

	tests := []struct {
		name string
		sig1 string
		sig2 string
		want bool
	}{
		{
			name: "Identical signatures",
			sig1: validSig,
			sig2: validSig,
			want: true,
		},
		{
			name: "Different signatures",
			sig1: validSig,
			sig2: ComputeHMAC(secret, "9999999999", method, path),
			want: false,
		},
		{
			name: "Empty signatures",
			sig1: "",
			sig2: "",
			want: true,
		},
		{
			name: "One empty signature",
			sig1: validSig,
			sig2: "",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CompareSignatures(tt.sig1, tt.sig2)
			if got != tt.want {
				t.Errorf("CompareSignatures() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseAuthHeader(t *testing.T) {
	tests := []struct {
		name          string
		header        string
		wantKey       string
		wantTimestamp string
		wantSignature string
		wantErr       bool
	}{
		{
			name:          "Valid header",
			header:        "MUXI-HMAC key=test-key, timestamp=1234567890, signature=abc123",
			wantKey:       "test-key",
			wantTimestamp: "1234567890",
			wantSignature: "abc123",
			wantErr:       false,
		},
		{
			name:    "Missing prefix",
			header:  "key=test-key, timestamp=1234567890, signature=abc123",
			wantErr: true,
		},
		{
			name:    "Missing key",
			header:  "MUXI-HMAC timestamp=1234567890, signature=abc123",
			wantErr: true,
		},
		{
			name:    "Missing timestamp",
			header:  "MUXI-HMAC key=test-key, signature=abc123",
			wantErr: true,
		},
		{
			name:    "Missing signature",
			header:  "MUXI-HMAC key=test-key, timestamp=1234567890",
			wantErr: true,
		},
		{
			name:    "Empty header",
			header:  "",
			wantErr: true,
		},
		{
			name:    "Malformed header",
			header:  "MUXI-HMAC invalid format",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, timestamp, signature, err := ParseAuthHeader(tt.header)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseAuthHeader() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if key != tt.wantKey {
					t.Errorf("ParseAuthHeader() Key = %v, want %v", key, tt.wantKey)
				}
				if timestamp != tt.wantTimestamp {
					t.Errorf("ParseAuthHeader() Timestamp = %v, want %v", timestamp, tt.wantTimestamp)
				}
				if signature != tt.wantSignature {
					t.Errorf("ParseAuthHeader() Signature = %v, want %v", signature, tt.wantSignature)
				}
			}
		})
	}
}

func TestSignatureConsistency(t *testing.T) {
	// Test that generating the same signature twice gives the same result
	secret := "test-secret"
	timestamp := "1234567890"
	method := "POST"
	path := "/test"

	sig1 := ComputeHMAC(secret, timestamp, method, path)
	sig2 := ComputeHMAC(secret, timestamp, method, path)

	if sig1 != sig2 {
		t.Errorf("Signatures are not consistent: %s != %s", sig1, sig2)
	}

	// Verify both signatures match
	if !CompareSignatures(sig1, sig2) {
		t.Error("Signature comparison failed for identical signatures")
	}
}

func TestValidateTimestamp(t *testing.T) {
	now := time.Now().Unix()
	tolerance := 300 // 5 minutes

	tests := []struct {
		name      string
		timestamp string
		tolerance int
		wantErr   bool
	}{
		{
			name:      "Current timestamp",
			timestamp: fmt.Sprintf("%d", now),
			tolerance: tolerance,
			wantErr:   false,
		},
		{
			name:      "Recent past (within tolerance)",
			timestamp: fmt.Sprintf("%d", now-100),
			tolerance: tolerance,
			wantErr:   false,
		},
		{
			name:      "Recent future (within tolerance)",
			timestamp: fmt.Sprintf("%d", now+100),
			tolerance: tolerance,
			wantErr:   false,
		},
		{
			name:      "Too old (outside tolerance)",
			timestamp: fmt.Sprintf("%d", now-400),
			tolerance: tolerance,
			wantErr:   true,
		},
		{
			name:      "Too far in future (outside tolerance)",
			timestamp: fmt.Sprintf("%d", now+400),
			tolerance: tolerance,
			wantErr:   true,
		},
		{
			name:      "Edge case: near tolerance boundary (past)",
			timestamp: fmt.Sprintf("%d", now-int64(tolerance)+10),
			tolerance: tolerance,
			wantErr:   false,
		},
		{
			name:      "Edge case: near tolerance boundary (future)",
			timestamp: fmt.Sprintf("%d", now+int64(tolerance)-10),
			tolerance: tolerance,
			wantErr:   false,
		},
		{
			name:      "Invalid timestamp format",
			timestamp: "not-a-number",
			tolerance: tolerance,
			wantErr:   true,
		},
		{
			name:      "Empty timestamp",
			timestamp: "",
			tolerance: tolerance,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTimestamp(tt.timestamp, tt.tolerance)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTimestamp() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGenerateCredentials(t *testing.T) {
	// Test that credentials are generated correctly
	key, secret, err := GenerateCredentials()
	if err != nil {
		t.Fatalf("GenerateCredentials() error = %v", err)
	}

	// Check key format
	if len(key) != 21 { // "MUXI_" + 16 hex chars
		t.Errorf("Key length = %d, want 21", len(key))
	}
	if !strings.HasPrefix(key, "MUXI_") {
		t.Errorf("Key doesn't start with MUXI_: %s", key)
	}

	// Check secret format
	if len(secret) != 67 { // "sk_" + 64 hex chars
		t.Errorf("Secret length = %d, want 67", len(secret))
	}
	if !strings.HasPrefix(secret, "sk_") {
		t.Errorf("Secret doesn't start with sk_: %s", secret)
	}

	// Test that generating twice gives different values
	key2, secret2, err := GenerateCredentials()
	if err != nil {
		t.Fatalf("GenerateCredentials() second call error = %v", err)
	}

	if key == key2 {
		t.Error("Generated same key twice (should be random)")
	}
	if secret == secret2 {
		t.Error("Generated same secret twice (should be random)")
	}
}

func TestGenerateCredentials_Format(t *testing.T) {
	key, secret, err := GenerateCredentials()
	if err != nil {
		t.Fatalf("GenerateCredentials() error = %v", err)
	}

	// Key should start with MUXI_
	if !strings.HasPrefix(key, "MUXI_") {
		t.Errorf("Key = %q, should start with MUXI_", key)
	}

	// Secret should start with sk_
	if !strings.HasPrefix(secret, "sk_") {
		t.Errorf("Secret = %q, should start with sk_", secret)
	}

	// Key should be MUXI_ + 8 chars
	if len(key) != 21 { // MUXI_ (5) + 16 chars
		t.Errorf("Key length = %d, want 21", len(key))
	}

	// Secret should be sk_ + 32 chars
	if len(secret) != 67 { // sk_ (3) + 64 chars
		t.Errorf("Secret length = %d, want 67", len(secret))
	}
}

func TestGenerateCredentials_Uniqueness(t *testing.T) {
	keys := make(map[string]bool)
	secrets := make(map[string]bool)

	// Generate multiple credentials
	for i := 0; i < 100; i++ {
		key, secret, err := GenerateCredentials()
		if err != nil {
			t.Fatalf("GenerateCredentials() error = %v", err)
		}

		if keys[key] {
			t.Errorf("Duplicate key generated: %s", key)
		}
		if secrets[secret] {
			t.Errorf("Duplicate secret generated: %s", secret)
		}

		keys[key] = true
		secrets[secret] = true
	}

	// Should have 100 unique keys and secrets
	if len(keys) != 100 {
		t.Errorf("Generated %d unique keys, want 100", len(keys))
	}
	if len(secrets) != 100 {
		t.Errorf("Generated %d unique secrets, want 100", len(secrets))
	}
}

func TestGenerateCredentials_ValidHexChars(t *testing.T) {
	key, secret, err := GenerateCredentials()
	if err != nil {
		t.Fatalf("GenerateCredentials() error = %v", err)
	}

	// Check key suffix contains only hex chars
	keySuffix := strings.TrimPrefix(key, "MUXI_")
	for _, c := range keySuffix {
		if !strings.ContainsRune("0123456789abcdef", c) {
			t.Errorf("Key contains non-hex char: %c", c)
		}
	}

	// Check secret suffix contains only hex chars
	secretSuffix := strings.TrimPrefix(secret, "sk_")
	for _, c := range secretSuffix {
		if !strings.ContainsRune("0123456789abcdef", c) {
			t.Errorf("Secret contains non-hex char: %c", c)
		}
	}
}

func TestGenerateCredentials_NoError(t *testing.T) {
	// GenerateCredentials should never error in normal circumstances
	for i := 0; i < 10; i++ {
		_, _, err := GenerateCredentials()
		if err != nil {
			t.Errorf("GenerateCredentials() iteration %d error = %v", i, err)
		}
	}
}
