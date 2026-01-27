package auth

import "testing"

func FuzzComputeHMAC(f *testing.F) {
	f.Add("secret", "1234567890", "GET", "/test")
	f.Add("", "", "", "")
	f.Fuzz(func(t *testing.T, secret, timestamp, method, path string) {
		ComputeHMAC(secret, timestamp, method, path) // should not panic
	})
}

func FuzzCompareSignatures(f *testing.F) {
	f.Add("sig1", "sig2")
	f.Add("", "")
	f.Add("abc", "abc")
	f.Fuzz(func(t *testing.T, sig1, sig2 string) {
		CompareSignatures(sig1, sig2) // should not panic
	})
}
