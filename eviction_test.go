package peerlimit

import (
	"context"
	"sync"
	"testing"
	"time"
)

// A key idle longer than the TTL is removed from all three maps the store keeps
// per key: crdt, baseline and lastSeen. Missing any one of them would either
// leak memory or leave the store's state inconsistent.
func TestStore_SweepEvictsStaleKey(t *testing.T) {
	s := newStore(Node1)
	s.allow(userID, 10, 100) // populates crdt, baseline and lastSeen
	// backdate lastSeen so the key looks idle for an hour
	s.lastSeen[userID] = time.Now().Add(-time.Hour)

	s.sweep(time.Minute)

	if _, ok := s.crdt[userID]; ok {
		t.Fatalf("stale key should be evicted from crdt")
	}
	if _, ok := s.baseline[userID]; ok {
		t.Fatalf("stale key should be evicted from baseline")
	}
	if _, ok := s.lastSeen[userID]; ok {
		t.Fatalf("stale key should be evicted from lastSeen")
	}
}

// A single sweep evicts the idle key while leaving a recently-touched key fully
// intact — eviction is per-key, not all-or-nothing.
func TestStore_SweepKeepsFreshKey(t *testing.T) {
	s := newStore(Node1)
	s.allow(userID, 10, 100)  // will be aged out
	s.allow(user2ID, 10, 100) // stays fresh
	s.lastSeen[userID] = time.Now().Add(-time.Hour)

	s.sweep(time.Minute)

	if _, ok := s.crdt[userID]; ok {
		t.Fatalf("stale key should be gone")
	}
	if _, ok := s.crdt[user2ID]; !ok {
		t.Fatalf("fresh key should survive in crdt")
	}
	if _, ok := s.baseline[user2ID]; !ok {
		t.Fatalf("fresh key should survive in baseline")
	}
	if _, ok := s.lastSeen[user2ID]; !ok {
		t.Fatalf("fresh key should survive in lastSeen")
	}
}

// Eviction must wipe the consumed count, not just free memory: an exhausted
// bucket that gets evicted has to come back as a brand-new key, otherwise the
// stale consumed value would keep denying the returning client.
func TestStore_SweepEvictedKeyRestartsFresh(t *testing.T) {
	s := newStore(Node1)
	rate, burst := 10.0, 5.0

	// drain the bucket to empty
	for s.allow(userID, rate, burst) {
	}
	if s.allow(userID, rate, burst) {
		t.Fatalf("bucket should be exhausted before eviction")
	}

	// the key goes idle and is swept away
	s.lastSeen[userID] = time.Now().Add(-time.Hour)
	s.sweep(time.Minute)

	if got := s.aggregate(userID); got != 0 {
		t.Fatalf("evicted key should have zero consumed, got %v", got)
	}
	// a fresh request is allowed again — the key restarted from scratch
	if !s.allow(userID, rate, burst) {
		t.Fatalf("evicted key should behave as fresh and be allowed")
	}
}

// A sweep that finds nothing to evict must be a clean no-op. Regression guard
// for the earlier bug where the mutex was unlocked without ever being locked
// when no key was stale, crashing with "unlock of unlocked mutex".
func TestStore_SweepNoStaleIsNoop(t *testing.T) {
	s := newStore(Node1)
	s.allow(userID, 10, 100)

	s.sweep(time.Hour) // nothing is older than an hour

	if _, ok := s.crdt[userID]; !ok {
		t.Fatalf("fresh key must survive a no-op sweep")
	}
}

// Sweeping concurrently with allow must not race on the shared maps. TTL is a
// nanosecond so the sweeper aggressively evicts while writers keep recreating
// the key — run under -race to catch any unguarded access.
func TestStore_SweepRace(t *testing.T) {
	s := newStore(Node1)
	var wg sync.WaitGroup

	for range 50 {
		wg.Go(func() {
			for range 100 {
				s.allow(userID, 100, 100)
			}
		})
	}
	wg.Go(func() {
		for range 500 {
			s.sweep(time.Nanosecond)
		}
	})

	wg.Wait()
}

// present reports whether the store still tracks k, reading the crdt map under
// the store lock so the check never races the background sweeper.
func present(s *store, k key) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.crdt[k]
	return ok
}

// End-to-end wiring: with KeyTTL/SweepInterval set, New starts the sweep loop
// and a key left idle past its TTL is evicted by that background loop without
// any further calls. Exercises sweepLoop and the eviction branch in New.
func TestLimiter_EvictsIdleKeyViaLoop(t *testing.T) {
	l, err := New(context.Background(), Config{
		Node:             Node1,
		BindPort:         9100 + int(portSeq.Add(1)),
		Discoverer:       NewStaticDiscoverer(),
		Rate:             10,
		Burst:            burst,
		SyncInterval:     time.Second,
		DiscoverInterval: time.Second * 5,
		KeyTTL:           50 * time.Millisecond,
		SweepInterval:    20 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(l.Close)

	l.Allow(context.TODO(), userID)
	if !present(l.store, userID) {
		t.Fatal("key should exist right after Allow")
	}

	// leave the key idle for well past TTL + a few sweep ticks
	time.Sleep(200 * time.Millisecond)

	if present(l.store, userID) {
		t.Fatal("idle key should be evicted by the sweep loop")
	}
}
