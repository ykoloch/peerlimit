package peerlimit

import (
	"context"
	"testing"
	"time"
)

func TestGossip_Converges(t *testing.T) {
	limA, err := New(context.TODO(),
		Config{
			Node:             Node1,
			Discoverer:       NewStaticDiscoverer("localhost:8080", "localhost:8081"),
			BindPort:         8080,
			Rate:             10,
			Burst:            burst,
			SyncInterval:     time.Millisecond * 20,
			DiscoverInterval: time.Second * 5,
		},
	)
	if err != nil {
		t.Fatalf("got error: %v", err)
	}

	limB, err := New(context.TODO(),
		Config{
			Discoverer:       NewStaticDiscoverer("localhost:8080", "localhost:8081"),
			Node:             Node2,
			BindPort:         8081,
			Rate:             10,
			Burst:            burst,
			SyncInterval:     time.Millisecond * 20,
			DiscoverInterval: time.Second * 5,
		},
	)
	if err != nil {
		t.Fatalf("got error: %v", err)
	}

	for range 5 {
		limA.Allow(context.TODO(), userID)
	}

	for range 3 {
		limB.Allow(context.TODO(), userID)
	}

	time.Sleep(time.Millisecond * 300)

	gotA := limA.store.aggregate(userID)
	if gotA != 8 {
		t.Fatalf("limiter A should have aggregate 8, got %v", gotA)
	}
	gotB := limB.store.aggregate(userID)
	if gotB != 8 {
		t.Fatalf("limiter B should have aggregate 8, got %v", gotB)
	}

	defer limA.Close()
	defer limB.Close()
}

// A valid remote snapshot is merged into the local store: the incoming node's
// cell is added on top of the local one.
func TestDelegate_MergeRemoteState(t *testing.T) {
	s := newStore(Node1)
	s.increment(userID) // local Node1:1
	d := &delegate{store: s}

	incoming := crdt{userID: gCounter{Node2: 5}}
	buf, err := incoming.marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	d.MergeRemoteState(buf, false)

	if got := s.aggregate(userID); got != 6 {
		t.Fatalf("want aggregate 6 after merge, got %v", got)
	}
}

// A corrupt gossip payload must be dropped, not panic or clobber local state.
// Peers can send garbage (version skew, truncated frames) and the node has to
// survive it.
func TestDelegate_MergeRemoteState_BadDataIgnored(t *testing.T) {
	s := newStore(Node1)
	s.increment(userID)
	d := &delegate{store: s}

	d.MergeRemoteState([]byte("{ not valid json"), false)

	if got := s.aggregate(userID); got != 1 {
		t.Fatalf("bad payload must leave state intact, want 1 got %v", got)
	}
}

// LocalState emits a snapshot that unmarshals back into the same counts — the
// wire format round-trips, which is what lets a peer merge it on the far side.
func TestDelegate_LocalState_RoundTrips(t *testing.T) {
	s := newStore(Node1)
	s.increment(userID)
	s.increment(userID)
	d := &delegate{store: s}

	buf := d.LocalState(false)
	got, err := unmarshalCRDT(buf)
	if err != nil {
		t.Fatalf("local state should unmarshal, got %v", err)
	}
	if got[userID][Node1] != 2 {
		t.Fatalf("round-trip lost data: %v", got)
	}
}
