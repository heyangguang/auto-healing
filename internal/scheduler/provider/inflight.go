package provider

import (
	"sync"

	"github.com/google/uuid"
)

type inFlightSet struct {
	mu  sync.Mutex
	ids map[uuid.UUID]int
}

func newInFlightSet() *inFlightSet {
	return &inFlightSet{ids: make(map[uuid.UUID]int)}
}

func (s *inFlightSet) Start(id uuid.UUID) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ids[id] > 0 {
		return false
	}
	s.ids[id] = 1
	return true
}

func (s *inFlightSet) Retain(id uuid.UUID) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ids[id] == 0 {
		return false
	}
	s.ids[id]++
	return true
}

func (s *inFlightSet) Finish(id uuid.UUID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ids[id] <= 1 {
		delete(s.ids, id)
		return
	}
	s.ids[id]--
}
