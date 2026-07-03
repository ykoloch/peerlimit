package peerlimit_test

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ykoloch/peerlimit"
)

// This example runs a single-node limiter — 100 tokens per second with a burst
// of 50, scoped per client by the key. In a real deployment several processes
// find each other through a Discoverer and keep their counts in sync over
// gossip; the calling code stays exactly the same.
func ExampleLimiter() {
	ctx := context.Background()

	lim, err := peerlimit.New(ctx, peerlimit.Config{
		Node:             "node-1",                        // unique per process
		BindPort:         7946,                            // gossip port
		Discoverer:       peerlimit.NewStaticDiscoverer(), // no peers: lone node
		Rate:             100,                             // tokens per second
		Burst:            50,                              // bucket capacity
		SyncInterval:     time.Second,
		DiscoverInterval: 5 * time.Second,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer lim.Close()

	// The key decides what the limit is scoped to: "user:123" per client,
	// "global" for one shared limit.
	if lim.Allow(ctx, "user:123") {
		fmt.Println("request allowed")
	}
}
