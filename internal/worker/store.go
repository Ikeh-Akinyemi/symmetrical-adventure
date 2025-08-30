package worker

import "sync"

type IdempotencyStore struct {
	mu    sync.Mutex
	store map[string]bool
}

func NewIdempotencyStore() *IdempotencyStore {
	return &IdempotencyStore{
		store: make(map[string]bool),
	}
}

// Has checks if a key (event UUID) exists in the store.
func (s *IdempotencyStore) Has(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, found := s.store[key]
	return found
}

// Set adds a key (event UUID) to the store.
func (s *IdempotencyStore) Set(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.store[key] = true
}