package monitoring

import (
	"sync"
	"time"
)

type Hub struct {
	mu          sync.RWMutex
	snapshot    Snapshot
	subscribers map[chan Snapshot]struct{}
}

func NewHub(initial Snapshot) *Hub {
	return &Hub{snapshot: initial, subscribers: make(map[chan Snapshot]struct{})}
}

func (h *Hub) Current() Snapshot {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return cloneSnapshot(h.snapshot)
}

func (h *Hub) Publish(snapshot Snapshot) Snapshot {
	h.mu.Lock()
	snapshot.Revision = h.snapshot.Revision + 1
	if snapshot.ObservedAt.IsZero() {
		snapshot.ObservedAt = time.Now().UTC()
	}
	h.snapshot = cloneSnapshot(snapshot)
	for subscriber := range h.subscribers {
		select {
		case subscriber <- cloneSnapshot(snapshot):
		default:
			select {
			case <-subscriber:
			default:
			}
			select {
			case subscriber <- cloneSnapshot(snapshot):
			default:
			}
		}
	}
	h.mu.Unlock()
	return snapshot
}

func (h *Hub) Subscribe() (<-chan Snapshot, func()) {
	ch := make(chan Snapshot, 1)
	h.mu.Lock()
	h.subscribers[ch] = struct{}{}
	ch <- cloneSnapshot(h.snapshot)
	h.mu.Unlock()

	var once sync.Once
	return ch, func() {
		once.Do(func() {
			h.mu.Lock()
			delete(h.subscribers, ch)
			close(ch)
			h.mu.Unlock()
		})
	}
}

func cloneSnapshot(snapshot Snapshot) Snapshot {
	if snapshot.Timeline == nil {
		snapshot.Timeline = []Segment{}
	} else {
		snapshot.Timeline = append([]Segment{}, snapshot.Timeline...)
	}
	return snapshot
}
