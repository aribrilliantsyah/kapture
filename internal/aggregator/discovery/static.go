package discovery

import (
	"log/slog"
	"sync"
)

// Provider discovers agent endpoints.
type Provider interface {
	// Endpoints returns the current list of agent HTTP base URLs.
	Endpoints() []string
}

// StaticProvider returns a fixed list of endpoints.
type StaticProvider struct {
	endpoints []string
	mu        sync.RWMutex
}

// NewStatic creates a static discovery provider.
func NewStatic(endpoints []string) *StaticProvider {
	return &StaticProvider{endpoints: endpoints}
}

// Endpoints returns the static endpoint list.
func (s *StaticProvider) Endpoints() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]string, len(s.endpoints))
	copy(result, s.endpoints)
	return result
}

// SetEndpoints updates the endpoint list.
func (s *StaticProvider) SetEndpoints(eps []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.endpoints = eps
	slog.Info("updated static endpoints", "count", len(eps))
}
