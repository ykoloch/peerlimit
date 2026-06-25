package peerlimit

import (
	"encoding/json"
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
	node     nodeID
	crdt     crdt
	baseline map[key]time.Time
	mu       sync.Mutex
}

func (s *store) allow(k key, rate, burst float64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.baseline[k]; !ok {
		s.baseline[k] = time.Now()
	}

	// total tokens budget accumulated since baseline
	budget := time.Since(s.baseline[k]).Seconds()*rate + burst
	consumed := s.aggregateLocked(k)
	available := budget - consumed
	if available < 1.0 {
		return false
	}

	// overflow - number of tokens that exceeds burst
	if overflow := available - burst; overflow > 0 {
		// shift: how long it took to over-accrue these surplus tokens —
		// rewind baseline by exactly that, so the surplus is "un-earned"
		shift := time.Duration((overflow / rate) * float64(time.Second))
		s.baseline[k] = s.baseline[k].Add(shift)
	}

	s.incrementLocked(k)
	return true
}

func newStore(node nodeID) *store {
	return &store{node: node, crdt: make(crdt), baseline: make(map[key]time.Time)}
}

func (s *store) increment(k key) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.incrementLocked(k)
}

func (s *store) incrementLocked(k key) {
	if _, ok := s.crdt[k]; !ok {
		s.crdt[k] = make(gCounter)
	}
	s.crdt[k][s.node]++
}

func (s *store) aggregate(k key) float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.aggregateLocked(k)
}

func (s *store) aggregateLocked(k key) float64 {
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

func (c crdt) marshal() ([]byte, error) {
	return json.Marshal(c)
}

func unmarshalCRDT(data []byte) (crdt, error) {
	c := make(crdt)
	err := json.Unmarshal(data, &c)
	return c, err
}
