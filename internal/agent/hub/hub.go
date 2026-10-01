// Package hub fans freshly written log entries out to live-tail subscribers.
package hub

import (
	"sync"
	"sync/atomic"

	"github.com/aribrilliantsyah/kapture/internal/logfilter"
	"github.com/aribrilliantsyah/kapture/internal/model"
)

const subBuffer = 2048

// Sub is one live-tail subscription.
type Sub struct {
	C       chan model.LogEntry
	filter  *logfilter.Filter
	Dropped atomic.Int64 // entries skipped because the reader was too slow
}

// Hub distributes entries to subscribers without ever blocking the writer.
type Hub struct {
	mu   sync.RWMutex
	subs map[*Sub]struct{}
}

// New creates an empty hub.
func New() *Hub {
	return &Hub{subs: map[*Sub]struct{}{}}
}

// Subscribe registers a subscriber for entries matching f.
func (h *Hub) Subscribe(f *logfilter.Filter) *Sub {
	s := &Sub{C: make(chan model.LogEntry, subBuffer), filter: f}
	h.mu.Lock()
	h.subs[s] = struct{}{}
	h.mu.Unlock()
	return s
}

// Unsubscribe removes a subscriber.
func (h *Hub) Unsubscribe(s *Sub) {
	h.mu.Lock()
	delete(h.subs, s)
	h.mu.Unlock()
}

// Publish offers entries to every matching subscriber; full buffers drop.
func (h *Hub) Publish(entries []model.LogEntry) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for s := range h.subs {
		for i := range entries {
			if !s.filter.Match(&entries[i]) {
				continue
			}
			select {
			case s.C <- entries[i]:
			default:
				s.Dropped.Add(1)
			}
		}
	}
}

// Len returns the number of subscribers.
func (h *Hub) Len() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.subs)
}
