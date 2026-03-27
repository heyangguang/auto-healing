package httpapi

import (
	"errors"
	"fmt"
	"sync"
)

type concurrentSectionState struct {
	mu     sync.Mutex
	result map[string]interface{}
	errs   []error
}

func newConcurrentSectionState(size int) *concurrentSectionState {
	return &concurrentSectionState{
		result: make(map[string]interface{}, size),
	}
}

func (s *concurrentSectionState) addResult(section string, data interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.result[section] = data
}

func (s *concurrentSectionState) addError(section string, err error) {
	if err == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.errs = append(s.errs, fmt.Errorf("section %s: %w", section, err))
}

func (s *concurrentSectionState) resultAndError() (map[string]interface{}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.result, errors.Join(s.errs...)
}

func safeSectionLoad(load func() (interface{}, error)) (data interface{}, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			err = fmt.Errorf("panic: %v", rec)
		}
	}()
	return load()
}
