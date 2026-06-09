package peerlimit

import (
	"sync"
	"testing"
)

func TestStore_Increment(t *testing.T) {
	s := newStore(Node1)
	s.increment(userID)
	value := s.crdt[userID][Node1]
	if value != 1 {
		t.Fatalf("new key value should be 1, got %v\n", value)
	}

	s.increment(user2ID)
	value2 := s.crdt[user2ID][Node1]
	if value2 != 1 {
		t.Fatalf("new key value should be 1, got %v\n", value2)
	}

	s.increment(userID)
	value = s.crdt[userID][Node1]
	if value != 2 {
		t.Fatalf("incremented value for existing key should be 2, got %v\n", value)
	}
}

func TestStore_Aggregate(t *testing.T) {
	s := newStore(Node1)
	s.merge(crdt{userID: gCounter{Node2: 5}})
	s.increment(userID)
	aggr := s.aggregate(userID)
	if aggr != 6 {
		t.Fatalf("aggregated value should be 6, got %v\n", aggr)
	}
	if aggr = s.aggregate("user:404"); aggr != 0 {
		t.Fatalf("aggregate value for non existing key should be 0, got %v\n", aggr)
	}
}

func TestStore_Race(t *testing.T) {
	s := newStore(Node1)
	var wg sync.WaitGroup
	workers := 100
	for range workers {
		wg.Go(func() {
			for range 100 {
				s.increment(userID)
			}
		})
	}

	for range 10 {
		wg.Go(func() {
			s.aggregate(userID)
		})
	}

	wg.Wait()
	if got := s.aggregate(userID); got != float64(workers*100) {
		t.Fatalf("want %d, got %v\n", workers*100, got)
	}
}
