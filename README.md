# peerlimit

peerlimit is a distributed rate limiter for Go, implemented as an embedded
library rather than a standalone service. Each replica enforces the limit from
its own in-memory state and decides immediately, without a network round trip;
the replicas keep that state approximately consistent by exchanging their counts
over a gossip protocol. Because coordination happens directly between peers, the
limiter depends on no shared datastore and no central coordinator, and no single
component's failure can disable it.

## How it compares

Rate limiting in Go generally takes one of three forms, each with a cost.
Single-process limiters such as `golang.org/x/time/rate` and `uber-go/ratelimit`
are simple but do not coordinate: every replica limits in isolation, so N
replicas admit up to N times the intended rate. Redis-backed limiters such as
`go-redis/redis_rate` coordinate accurately, but place a network round trip on
every decision and turn the datastore into both a hard dependency and a single
point of failure. Gossip-based services such as `mailgun/gubernator` avoid the
shared store, yet remain a separate service that still has to be deployed and
operated.

peerlimit occupies the position none of these do: distributed coordination
offered as a library rather than as infrastructure. Its accuracy is weaker than
that of a strongly consistent store, but in exchange it removes the operational
dependency and the single point of failure such a store introduces — a trade-off
that suits the workloads described under
[Consistency & failure model](#consistency--failure-model).

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

### Choosing the key

There are no separate modes; the key you pass sets the scope of the limit, and
the scope in turn determines how accurate that limit can be. A key such as
`"user:123"` creates an independent bucket per user, and distributing traffic
across many such keys is the favourable case: each key changes slowly enough
that gossip keeps the nodes closely aligned. Concentrating all traffic onto a
single key — `"global"`, say — has the opposite effect. It becomes a hot key
that changes faster than gossip can propagate it, and so exhibits the largest
over-allow. peerlimit is not intended to enforce a hard global ceiling; see the
failure model below.

## How it works

The algorithm is a token bucket: `Rate` tokens accrue per second up to a
capacity of `Burst`, and each allowed request spends one. Rather than share a
single counter, every node owns its own cell and only ever increments it, since
consumption is monotonic. Other nodes observe that cell but never write to it,
which removes write contention and the need for any cross-cluster lock.

Those cells form a G-Counter CRDT. Merging them takes the maximum value per
node, an operation that is commutative, associative and idempotent, so gossip
messages that arrive reordered or more than once cannot corrupt the result. The
merged value represents total consumption across the cluster, which each node
subtracts from its local bucket.

Transport is handled by
[hashicorp/memberlist](https://github.com/hashicorp/memberlist), which already
solves membership, failure detection and the gossip wire; peerlimit supplies
only the payload and the merge logic. memberlist uses UDP for frequent small
messages and TCP for the occasional full state sync. Every node answers `Allow`
from its own merged view, so convergence is eventual and consistency is not
strict — as the next section makes precise.

## Consistency & failure model

In CAP terms peerlimit is an AP system: it remains available and
partition-tolerant and gives up strict consistency. The practical consequence is
*over-allow* — the cluster may briefly permit more than the configured limit.
The situations that cause it are:

1. **Partition** — separated halves cannot observe each other's consumption,
   which is unavoidable for an AP system.
2. **Gossip lag** — a spike is admitted locally before peers learn of it.
3. **Cold start** — a fresh node has not yet received any state.
4. **Hot key** — a key changes faster than gossip can spread it.
5. **Dead peer** — a node's most recent increments may be partially lost.
6. **Clock skew** — refill timing differs slightly between nodes.

In steady state peerlimit does not *under*-allow, and it reconverges once gossip
catches up. What it does not provide is a hard global ceiling at any single
instant.

### Suitable uses

Abuse protection, throttling noisy tenants, soft API limits, and shielding
internal services from one another — workloads where briefly allowing slightly
more than the limit does no harm.

### Unsuitable uses

Billing-grade quotas, or anything where exceeding the limit costs money or
breaks correctness. When a hard guarantee is required, a strongly consistent
store such as Redis with a Lua script is the appropriate tool. The limitation is
a deliberate trade-off, not a defect.

## Discovery

A node needs at least one peer address to bootstrap, after which gossip spreads
the rest. How that first contact is supplied is pluggable through the
`PeerDiscoverer` interface:

```go
type PeerDiscoverer interface {
	Discover(ctx context.Context) ([]string, error)
}
```

Two implementations are provided. The static discoverer takes an explicit list
of `host:port` addresses and suits fixed topologies — bare VMs with known
addresses, docker-compose, or local testing:

```go
peerlimit.NewStaticDiscoverer("10.0.0.2:7946", "10.0.0.3:7946")
```

The DNS discoverer resolves a name to the current set of peer IPs and attaches
the configured port to each. It is meant for a Kubernetes headless Service or a
docker-compose service name, where a single name maps to all of the backing
pods or containers:

```go
peerlimit.NewDNSDiscoverer("peerlimit.default.svc.cluster.local", "7946")
```

A self-healing loop re-runs discovery on `DiscoverInterval` and re-joins, so a
node recovers from cold start, a full partition, or seed churn without manual
intervention. An empty result is treated as "no peers yet": the node runs solo
and keeps retrying. Discovery only ever *adds* join targets — pruning dead
members is memberlist's failure detection, not the discoverer's concern.

## Deployment environments

peerlimit suits environments where processes have stable network identities and
can maintain long-lived gossip connections, including Kubernetes (through a
headless Service), docker-compose, virtual machines, and ECS. It is a poor fit
for scale-to-zero and FaaS platforms such as Cloud Run or Lambda, where peers
have no stable address, instances are too short-lived to converge, and churn
outpaces gossip.

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
| `LogOutput` | io.Writer | Optional; where memberlist logs go (silent by default) |

Every field except `LogOutput` is required.

## Known limitations

The per-key bucket map currently grows without bound, as keys are never
reclaimed. This is acceptable for a bounded key space but becomes a problem for
high-cardinality per-client keys over a long uptime; distributed eviction is
inherently delicate, because max-merge can resurrect a key that one node has
evicted. State is also held entirely in memory and is lost on restart, which is
intentional — for rate limiting it is acceptable, and persistence is not
planned. Finally, the consistency model is eventual by design, as detailed in
[Consistency & failure model](#consistency--failure-model).

## License

MIT — see [LICENSE](LICENSE).
