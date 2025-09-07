package worker

import (
	"sync"
	"testing"
)

func TestIdempotencyStore(t *testing.T) {
	t.Run("Set and Has", func(t *testing.T) {
		store := NewIdempotencyStore()
		key := "test-uuid-123"

		// Initially, the key should not exist
		if store.Has(key) {
			t.Errorf("Expected Has(%q) to be false, but got true", key)
		}

		// Set the key
		store.Set(key)

		// Now, the key should exist
		if !store.Has(key) {
			t.Errorf("Expected Has(%q) to be true, but got false", key)
		}
	})

	t.Run("Concurrency Safety", func(t *testing.T) {
		store := NewIdempotencyStore()
		key := "concurrent-key"
		numGoroutines := 100
		var wg sync.WaitGroup

		// Run many goroutines that try to Set the same key simultaneously
		wg.Add(numGoroutines)
		for range numGoroutines {
			go func() {
				defer wg.Done()
				store.Set(key)
			}()
		}
		wg.Wait()

		// After all goroutines complete, the key must exist
		if !store.Has(key) {
			t.Errorf("Expected key to be set after concurrent writes, but it was not")
		}
	})
}