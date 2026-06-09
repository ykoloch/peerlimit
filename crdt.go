package peerlimit

import (
	"sync"
)

// gCounter is an atomic data structure - a grow-only counter that holds,
// for a single key, how many requests each node has consumed (nodeID -> count).
type gCounter map[string]float64

// crdt is a Conflict-free Replicated Data Type that maps a key
// (the value passed to Allow, e.g. "user:123") to its per-node gCounter.
type crdt map[string]gCounter

func (gc gCounter) merge(input gCounter) {
	for nodeID, value := range input {
		gc[nodeID] = max(gc[nodeID], value)
	}
}

func (c crdt) merge(input crdt) {
	for key, value := range input {
		if _, ok := c[key]; !ok {
			c[key] = make(gCounter)
		}
		c[key].merge(value)
	}
}

type store struct {
	nodeID string
	crdt   crdt
	mu     sync.Mutex
}

func newStore(nodeID string) *store {
	return &store{nodeID: nodeID, crdt: make(crdt)}
}

func (s *store) increment(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.crdt[key]; !ok {
		s.crdt[key] = make(gCounter)
	}
	s.crdt[key][s.nodeID]++
}

func (s *store) aggregate(key string) float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	var result float64
	for _, value := range s.crdt[key] {
		result += value
	}
	return result
}

func (s *store) merge(input crdt) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.crdt.merge(input)
}
