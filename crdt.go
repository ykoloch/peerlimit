package peerlimit

import (
	"sync"
	"time"

	"github.com/vmihailenco/msgpack/v5"
)

type (
	nodeID string
	key    string
)

// gCounter is a grow-only counter (G-Counter CRDT): for a single key it holds
// how many requests each node has consumed (nodeID -> count). Counts only ever
// increase, so merging cell-by-cell with max can never lose consumption.
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

// store holds one node's rate-limiter state, all guarded by mu: the replicated
// crdt (per-key consumption, gossiped) plus two local-only maps — baseline, the
// token-bucket refill anchor, and lastSeen, the eviction clock.
type store struct {
	node     nodeID
	crdt     crdt
	baseline map[key]time.Time
	mu       sync.Mutex
	lastSeen map[key]time.Time
}

// allow makes the token-bucket decision for k over the aggregated G-Counter,
// recording consumption when it returns true.
func (s *store) allow(k key, rate, burst float64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.lastSeen[k] = time.Now()

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
	return &store{node: node, crdt: make(crdt), baseline: make(map[key]time.Time), lastSeen: make(map[key]time.Time)}
}

func (s *store) increment(k key) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.incrementLocked(k)
}

// incrementLocked bumps this node's cell for k. The caller must hold mu, as
// with every *Locked helper.
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

// merge folds a peer's payload into local state: G-Counter cells by max, and
// lastSeen by max so a key stays alive while active on any node and is evicted
// only once idle across the cluster. The incoming timestamp is taken as-is,
// never time.Now() — otherwise a mere mention of a stale key would keep it
// immortal and eviction would never fire.
func (s *store) merge(input payload) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.crdt.merge(input.Crdt)
	for k, v := range input.LastSeen {
		s.lastSeen[k] = maxTime(s.lastSeen[k], v)
	}
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

// snapshot serialises the current state (counts + lastSeen) into a payload for
// a peer to merge.
func (s *store) snapshot() ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	pl := payload{
		Crdt:     s.crdt,
		LastSeen: s.lastSeen,
	}
	return pl.marshal()
}

// sweep evicts every key idle longer than ttl, dropping it from all local maps.
// The whole pass is held under mu.
func (s *store) sweep(ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k := range s.lastSeen {
		passed := time.Since(s.lastSeen[k])
		if passed > ttl {
			delete(s.baseline, k)
			delete(s.lastSeen, k)
			delete(s.crdt, k)
		}
	}
}

// payload is the gossip wire format: the G-Counter counts and the lastSeen
// timestamps, the two pieces of state peers must exchange. Fields are exported
// so msgpack can encode them.
type payload struct {
	Crdt     crdt              `msgpack:"c"`
	LastSeen map[key]time.Time `msgpack:"l"`
}

func (p payload) marshal() ([]byte, error) {
	return msgpack.Marshal(p)
}

func unmarshalPayload(data []byte) (payload, error) {
	p := new(payload)
	err := msgpack.Unmarshal(data, p)
	return *p, err
}
