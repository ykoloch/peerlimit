package peerlimit

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// startNode brings up a limiter on port, seeded with seeds. Refill is slow (one
// token per second) so a bucket does not meaningfully refill within a test and
// the counts we assert on stay stable.
func startNode(t *testing.T, name nodeID, port int, seeds ...string) *Limiter {
	t.Helper()
	lim, err := New(context.Background(), Config{
		Node:             name,
		BindPort:         port,
		Discoverer:       NewStaticDiscoverer(seeds...),
		Rate:             1,
		Burst:            burst,
		SyncInterval:     30 * time.Millisecond,
		DiscoverInterval: 50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("start %s: %v", name, err)
	}
	return lim
}

// crash simulates an ungraceful death: the node's loops stop and its transport
// is torn down without the graceful Leave that Close would broadcast, so peers
// have to notice the failure on their own.
func crash(l *Limiter) {
	l.cancel()
	l.ml.Shutdown()
}

// drain calls Allow until it is refused, returning how many were admitted.
func drain(l *Limiter, k string) int {
	n := 0
	for l.Allow(context.Background(), k) {
		n++
		if n > int(burst)*4 { // safety valve, should never trip
			break
		}
	}
	return n
}

// eventually polls cond until it holds or timeout elapses. Gossip is
// asynchronous, so cluster-wide assertions have to wait for convergence rather
// than read once.
func eventually(t *testing.T, timeout time.Duration, cond func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return cond()
}

func ports(n int) []int {
	base := 7000 + int(portSeq.Add(1))*10
	ps := make([]int, n)
	for i := range ps {
		ps[i] = base + i
	}
	return ps
}

func addr(port int) string { return fmt.Sprintf("localhost:%d", port) }

// A crashing peer must not take the cluster down. The survivors keep answering
// Allow, keep converging with one another, and retain the dead node's last
// observed consumption — its G-Counter cell is grow-only and was already
// gossiped, so max-merge holds onto it.
func TestChaos_PeerCrashKeepsClusterAvailable(t *testing.T) {
	p := ports(3)
	seeds := []string{addr(p[0]), addr(p[1]), addr(p[2])}

	a := startNode(t, "A", p[0], seeds...)
	b := startNode(t, "B", p[1], seeds...)
	c := startNode(t, "C", p[2], seeds...)
	t.Cleanup(a.Close)
	t.Cleanup(b.Close)
	// c is crashed mid-test, so it is not registered for a graceful Close.

	ctx := context.Background()
	for range 5 {
		a.Allow(ctx, userID)
	}
	for range 3 {
		b.Allow(ctx, userID)
	}
	for range 7 {
		c.Allow(ctx, userID)
	}

	// the three converge on 5+3+7 = 15
	if !eventually(t, 2*time.Second, func() bool {
		return a.store.aggregate(userID) == 15 && b.store.aggregate(userID) == 15
	}) {
		t.Fatalf("cluster did not converge before crash: A=%v B=%v",
			a.store.aggregate(userID), b.store.aggregate(userID))
	}

	crash(c)

	// a survivor still admits requests — availability is unaffected
	if !a.Allow(ctx, userID) {
		t.Fatal("survivor A should still allow after the peer crash")
	}
	b.Allow(ctx, userID)

	// survivors reconverge among themselves, still carrying C's 7:
	// A did 5+1, B did 3+1, C's 7 is retained -> 17
	if !eventually(t, 2*time.Second, func() bool {
		return a.store.aggregate(userID) == 17 && b.store.aggregate(userID) == 17
	}) {
		t.Fatalf("survivors did not reconverge after crash: A=%v B=%v",
			a.store.aggregate(userID), b.store.aggregate(userID))
	}
}

// A partition is simulated by running two nodes in isolation: neither sees the
// other's consumption, so each independently admits up to its own burst and the
// cluster over-allows, admitting close to 2*burst for a single key. When the
// partition heals the disjoint G-Counter cells merge by max, both sides converge
// on the true combined total, and further requests are then correctly denied.
func TestChaos_PartitionOverAllowsThenConverges(t *testing.T) {
	p := ports(2)

	// isolated: each node's discoverer knows only itself
	a := startNode(t, "A", p[0], addr(p[0]))
	b := startNode(t, "B", p[1], addr(p[1]))
	t.Cleanup(a.Close)
	t.Cleanup(b.Close)

	admitA := drain(a, userID)
	admitB := drain(b, userID)

	// each side drained its own bucket independently — that is the over-allow
	if admitA < burst || admitB < burst {
		t.Fatalf("each isolated side should admit at least burst: A=%d B=%d burst=%d",
			admitA, admitB, int(burst))
	}

	// heal the partition: reconnect the two nodes
	if _, err := a.ml.Join([]string{addr(p[1])}); err != nil {
		t.Fatalf("heal join failed: %v", err)
	}

	// disjoint cells, so the merged total is the sum of the two — not double
	// counted, and idempotent under continued gossip
	want := float64(admitA + admitB)
	if !eventually(t, 2*time.Second, func() bool {
		return a.store.aggregate(userID) == want && b.store.aggregate(userID) == want
	}) {
		t.Fatalf("did not converge after heal: A=%v B=%v want=%v",
			a.store.aggregate(userID), b.store.aggregate(userID), want)
	}

	// the cluster is now over its limit, so a fresh request is refused
	if a.Allow(context.Background(), userID) {
		t.Fatal("after heal A should deny: cluster is over the combined limit")
	}
}
