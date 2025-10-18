package registry

import "testing"

func TestValidateFormationID(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{"valid lowercase", "my-api", false},
		{"valid with numbers", "api-v2", false},
		{"valid long", "my-long-formation-name-here", false},
		{"valid alphanumeric", "api123", false},
		{"reserved: health", "health", true},
		{"reserved: ping", "ping", true},
		{"reserved: rpc", "rpc", true},
		{"reserved: api", "api", true},
		{"reserved: server", "server", true},
		{"reserved: admin", "admin", true},
		{"reserved: metrics", "metrics", true},
		{"too short", "ab", true},
		{"too long", "this-is-a-very-long-formation-id-that-exceeds-fifty-characters", true},
		{"uppercase", "My-API", true},
		{"spaces", "my api", true},
		{"underscore", "my_api", true},
		{"empty", "", true},
		{"starts with hyphen", "-myapi", true},
		{"ends with hyphen", "myapi-", true},
		{"special chars", "my@api", true},
		{"dots", "my.api", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFormationID(tt.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFormationID(%q) error = %v, wantErr %v", tt.id, err, tt.wantErr)
			}
		})
	}
}

func TestValidateFormationID_EdgeCases(t *testing.T) {
	// Test exact boundaries
	t.Run("exactly 3 chars", func(t *testing.T) {
		if err := ValidateFormationID("a-b"); err != nil {
			t.Errorf("3-char ID should be valid: %v", err)
		}
	})

	t.Run("exactly 50 chars", func(t *testing.T) {
		// 50 chars: a + 48 middle chars + b
		id := "a" + "012345678901234567890123456789012345678901234567" + "b"
		if len(id) != 50 {
			t.Fatalf("Test ID should be 50 chars, got %d", len(id))
		}
		if err := ValidateFormationID(id); err != nil {
			t.Errorf("50-char ID should be valid: %v", err)
		}
	})

	t.Run("51 chars", func(t *testing.T) {
		// 51 chars: a + 49 middle chars + b
		id := "a" + "0123456789012345678901234567890123456789012345678" + "b"
		if len(id) != 51 {
			t.Fatalf("Test ID should be 51 chars, got %d", len(id))
		}
		if err := ValidateFormationID(id); err == nil {
			t.Error("51-char ID should be invalid")
		}
	})
}
