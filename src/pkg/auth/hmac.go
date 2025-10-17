package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ComputeHMAC computes HMAC-SHA256 signature
// Format: HMAC-SHA256(secret, "{timestamp};{method};{path}")
func ComputeHMAC(secret, timestamp, method, path string) string {
	message := fmt.Sprintf("%s;%s;%s", timestamp, method, path)
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(message))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// ParseAuthHeader parses MUXI-HMAC authorization header
// Format: MUXI-HMAC key=XXX, timestamp=YYY, signature=ZZZ
func ParseAuthHeader(header string) (key, timestamp, signature string, err error) {
	if !strings.HasPrefix(header, "MUXI-HMAC ") {
		return "", "", "", fmt.Errorf("invalid auth header format: must start with 'MUXI-HMAC '")
	}

	// Remove prefix
	params := strings.TrimPrefix(header, "MUXI-HMAC ")

	// Split by comma
	parts := strings.Split(params, ", ")

	// Parse key=value pairs
	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}

		switch kv[0] {
		case "key":
			key = kv[1]
		case "timestamp":
			timestamp = kv[1]
		case "signature":
			signature = kv[1]
		}
	}

	// Validate all required parameters present
	if key == "" || timestamp == "" || signature == "" {
		return "", "", "", fmt.Errorf("missing required parameters (key, timestamp, or signature)")
	}

	return key, timestamp, signature, nil
}

// ValidateTimestamp checks if timestamp is within tolerance
func ValidateTimestamp(timestampStr string, toleranceSeconds int) error {
	ts, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid timestamp format: %w", err)
	}

	timestamp := time.Unix(ts, 0)
	now := time.Now()
	diff := now.Sub(timestamp)

	// Handle negative diff (future timestamp)
	if diff < 0 {
		diff = -diff
	}

	tolerance := time.Duration(toleranceSeconds) * time.Second
	if diff > tolerance {
		return fmt.Errorf("timestamp expired (diff: %v, tolerance: %v)", diff, tolerance)
	}

	return nil
}

// CompareSignatures performs constant-time signature comparison
// Prevents timing attacks
func CompareSignatures(sig1, sig2 string) bool {
	return subtle.ConstantTimeCompare([]byte(sig1), []byte(sig2)) == 1
}
