package registry

import "testing"

func FuzzValidateFormationID(f *testing.F) {
	f.Add("my-formation")
	f.Add("abc")
	f.Add("")
	f.Add("health")
	f.Add("a-very-long-formation-id-that-exceeds-the-maximum")
	f.Fuzz(func(t *testing.T, id string) {
		ValidateFormationID(id) // should not panic
	})
}

func FuzzPortAllocation(f *testing.F) {
	f.Add(19000, 19100, "test-formation")
	f.Fuzz(func(t *testing.T, start, end int, formationID string) {
		if start < 1024 || end > 65535 || start >= end {
			return
		}
		pool, err := NewPortPool(start, end)
		if err != nil {
			return
		}
		pool.Allocate(formationID) // should not panic
	})
}
