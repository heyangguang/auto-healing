package schedulerx

import (
	"sync"

	"github.com/google/uuid"
)

type InFlightSet struct {
	mu  sync.Mutex
	ids map[uuid.UUID]int
}

func NewInFlightSet() *InFlightSet {
	return &InFlightSet{ids: make(map[uuid.UUID]int)}
}

func (s *InFlightSet) Start(id uuid.UUID) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ids[id] > 0 {
		return false
	}
	s.ids[id] = 1
	return true
}

func (s *InFlightSet) Retain(id uuid.UUID) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ids[id] == 0 {
		return false
	}
	s.ids[id]++
	return true
}

func (s *InFlightSet) Finish(id uuid.UUID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ids[id] <= 1 {
		delete(s.ids, id)
		return
	}
	s.ids[id]--
}
