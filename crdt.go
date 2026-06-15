package peerlimit

import (
	"sync"
	"time"
)

type (
	nodeID string
	key    string
)

// gCounter is an atomic data structure - a grow-only counter that holds,
// for a single key, how many requests each node has consumed (nodeID -> count).
type gCounter map[nodeID]float64

// crdt is a Conflict-free Replicated Data Type that maps a key
// (the value passed to Allow, e.g. "user:123") to its per-node gCounter.
type crdt map[key]gCounter

func (gc gCounter) merge(input gCounter) {
	for node, value := range input {
		gc[node] = max(gc[node], value)
	}
}

func (c crdt) merge(input crdt) {
	for k, value := range input {
		if _, ok := c[k]; !ok {
			c[k] = make(gCounter)
		}
		c[k].merge(value)
	}
}

type store struct {
	node    nodeID
	crdt    crdt
	created map[key]time.Time
	mu      sync.Mutex
}

func newStore(node nodeID) *store {
	return &store{node: node, crdt: make(crdt), created: make(map[key]time.Time)}
}

func (s *store) increment(k key) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.crdt[k]; !ok {
		s.crdt[k] = make(gCounter)
	}
	s.crdt[k][s.node]++
}

func (s *store) aggregate(k key) float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	var result float64
	for _, value := range s.crdt[k] {
		result += value
	}
	return result
}

func (s *store) merge(input crdt) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.crdt.merge(input)
}
