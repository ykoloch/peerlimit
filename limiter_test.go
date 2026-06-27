package peerlimit

import (
	"context"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const (
	userID  = "user:1"
	user2ID = "user:2"
	burst   = 50
)

// portSeq hands out a unique BindPort per test limiter so that several
// memberlist instances in the same test binary never collide on one port.
var portSeq atomic.Int32

// newTestLimiter builds a single-node limiter on its own port and registers
// Close via t.Cleanup, so the memberlist listeners and goroutines are torn
// down when the test ends. Seeds is empty: a lone node bootstraps without Join.
func newTestLimiter(t *testing.T, rate float64) *Limiter {
	t.Helper()
	l, err := New(Config{
		Node:         Node1,
		BindPort:     8080 + int(portSeq.Add(1)),
		Rate:         rate,
		Burst:        burst,
		SyncInterval: time.Second,
	})
	if err != nil {
		t.Fatalf("got error: %v", err)
	}
	t.Cleanup(l.Close)
	return l
}

func TestAllow_NewKeyPasses(t *testing.T) {
	l := newTestLimiter(t, 10)
	if !l.Allow(context.TODO(), userID) {
		t.Error("Allow on fresh key=false, want true")
	}
}

func TestAllow_BurstExhaustion(t *testing.T) {
	l := newTestLimiter(t, 5)
	for i := range int(l.config.Burst) {
		if !l.Allow(context.TODO(), userID) {
			t.Fatalf("all requests under bucket capacity should be allowed, rejected: %v", i)
		}
	}
	if l.Allow(context.TODO(), userID) {
		t.Error("request above capacity = true, want false")
	}
}

func TestAllow_Refill(t *testing.T) {
	l := newTestLimiter(t, 5)

	// exhaust bucket
	for range int(l.config.Burst) {
		l.Allow(context.TODO(), userID)
	}
	if l.Allow(context.TODO(), userID) {
		t.Fatal("bucket should be empty after exhaustion")
	}
	// wait for some tokens to be added
	time.Sleep(time.Millisecond * 300)
	if !l.Allow(context.TODO(), userID) {
		t.Error("request after refill = false, want true")
	}
}

func TestAllow_BucketsIsolated(t *testing.T) {
	l := newTestLimiter(t, 5)

	// exhaust bucket for user_1
	for range int(l.config.Burst) {
		l.Allow(context.TODO(), userID)
	}
	if l.Allow(context.TODO(), userID) {
		t.Fatal("bucket should be empty after exhaustion")
	}
	// fire request from user_2
	if !l.Allow(context.TODO(), user2ID) {
		t.Error("request for user_2 = false, want true")
	}
}

func TestAllow_RaceSingleKey(t *testing.T) {
	l := newTestLimiter(t, 5)

	workers := 100
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for range 100 {
				l.Allow(context.TODO(), userID)
			}
		})
	}
	wg.Wait()
}

func TestAllow_RaceMultKey(t *testing.T) {
	l := newTestLimiter(t, 5)

	workers := 100
	var wg sync.WaitGroup
	for i := range workers {
		wg.Go(func() {
			for range 100 {
				l.Allow(context.TODO(), strconv.Itoa(i))
			}
		})
	}
	wg.Wait()
}
