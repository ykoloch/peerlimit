# peerlimit

peerlimit is a distributed rate limiter for Go, implemented as an embedded
library rather than a standalone service. Each replica decides immediately from
its own in-memory state — no network round trip — and the replicas keep that
state approximately consistent by gossiping their counts. There is no shared
datastore and no central coordinator, so no single failure can disable limiting.

The cost is accuracy: under partition or gossip lag the cluster may briefly allow
slightly more than the limit. That suits abuse protection, noisy-tenant
throttling, soft API limits, and shielding internal services — **not**
billing-grade quotas. See [DESIGN.md](DESIGN.md) for the full trade-off.

## Install

```
go get github.com/ykoloch/peerlimit
```

## Quick start

```go
ctx := context.Background()

lim, err := peerlimit.New(ctx, peerlimit.Config{
	Node:             "node-1",                       // unique per process
	BindPort:         7946,                           // gossip port
	Discoverer:       peerlimit.NewStaticDiscoverer("10.0.0.2:7946", "10.0.0.3:7946"),
	Rate:             100,                            // tokens added per second
	Burst:            50,                             // bucket capacity
	SyncInterval:     200 * time.Millisecond,         // how often state is gossiped
	DiscoverInterval: 10 * time.Second,               // how often peers are re-discovered
})
if err != nil {
	log.Fatal(err)
}
defer lim.Close()

if lim.Allow(ctx, "user:123") {
	// serve the request
} else {
	// reject: over the limit
}
```

`Allow` returns immediately from local state and never blocks on the network.
The key sets the scope: `"user:123"` is an independent bucket per client,
`"global"` one shared limit. Many slow-changing keys converge well; a single hot
key shows the largest over-allow (see [DESIGN.md](DESIGN.md#choosing-the-key)).

## Configuration

| Field | Type | Meaning |
| --- | --- | --- |
| `Node` | string | Unique identity for this process (its G-Counter cell) |
| `BindPort` | int | Port memberlist binds for gossip |
| `Discoverer` | `PeerDiscoverer` | How peers are found (static or DNS) |
| `Rate` | float64 | Tokens added per second |
| `Burst` | float64 | Bucket capacity (maximum burst) |
| `SyncInterval` | time.Duration | How often local state is gossiped and merged |
| `DiscoverInterval` | time.Duration | How often peers are re-discovered |
| `KeyTTL` | time.Duration | Optional; evict a key after this much idle time (requires `SweepInterval`) |
| `SweepInterval` | time.Duration | Optional; how often the eviction sweep runs (requires `KeyTTL`) |
| `LogOutput` | io.Writer | Optional; where memberlist logs go (silent by default) |

Every field is required except `LogOutput` and the `KeyTTL`/`SweepInterval`
eviction pair, which is opt-in and must be set together or left unset.

## Discovery

A node needs one peer address to bootstrap; gossip spreads the rest. Two
`PeerDiscoverer` implementations ship:

```go
peerlimit.NewStaticDiscoverer("10.0.0.2:7946", "10.0.0.3:7946")     // fixed list
peerlimit.NewDNSDiscoverer("peerlimit.default.svc.cluster.local", "7946") // k8s headless service
```

Discovery re-runs on `DiscoverInterval` and re-joins, so a node self-heals from
cold start, partition, or seed churn. See [DESIGN.md](DESIGN.md#discovery).

## Design & failure model

peerlimit is an AP system with an eventual-consistency model, and it evicts idle
keys through a cluster-coordinated TTL. The architecture, the exact failure
modes (over-allow and the eviction resurrection window), and the suitable
deployment environments are all in [DESIGN.md](DESIGN.md).

## License

MIT — see [LICENSE](LICENSE).
