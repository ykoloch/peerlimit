# peerlimit — design

This document covers the architecture, the consistency guarantees, and the
failure modes. For usage see the [README](README.md) and the
[API docs](https://pkg.go.dev/github.com/ykoloch/peerlimit).

## Goals

- Distributed rate limiting as an **embeddable library**, not a separate service.
- **Local decisions**: every node answers `Allow` from its own state, with no
  per-request network round trip.
- **No shared datastore and no central coordinator**, so no single component's
  failure can disable limiting.
- **Eventual convergence** between nodes over gossip.

## Non-goals

- Strong consistency or a hard global ceiling at any single instant.
- Billing-grade quotas, or anything where exceeding the limit costs money.
- Persistence: state is in-memory and lost on restart, by design.

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
dependency and the single point of failure such a store introduces.

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
strict.

### Choosing the key

There are no separate modes; the key you pass sets the scope of the limit, and
the scope in turn determines how accurate that limit can be. A key such as
`"user:123"` creates an independent bucket per user, and distributing traffic
across many such keys is the favourable case: each key changes slowly enough
that gossip keeps the nodes closely aligned. Concentrating all traffic onto a
single key — `"global"`, say — has the opposite effect. It becomes a hot key
that changes faster than gossip can propagate it, and so exhibits the largest
over-allow.

## Eviction

By default per-key state is kept for the lifetime of the process. Setting
`KeyTTL` together with `SweepInterval` turns on eviction: a key left idle for
longer than `KeyTTL` is dropped, and a background sweep reclaims such keys every
`SweepInterval`. The two fields are enabled together or left unset together.

Eviction is coordinated across the cluster rather than purely local. Alongside
the consumption counts, each node gossips the last time it saw activity for a
key, merged as a maximum — so a key stays alive while it is active on *any* node
and is reclaimed only once it has gone idle across the whole cluster. Without
this, a node that only ever learns of a key through gossip and never serves it
locally would never reclaim it.

One rough edge remains, by design. Consumption is replicated but the local
refill baseline is not, so a key that a node has just evicted and then relearns
from a lagging peer comes back with its consumed count intact but its baseline
reset to now. Until the baseline catches up, that node may briefly reject
requests it should have allowed — a short *under*-allow, the mirror of the
over-allow the system is otherwise prone to. It is bounded, self-correcting, and
occurs only while eviction is enabled.

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

In steady state peerlimit does not *under*-allow — the one exception is the
eviction resurrection window, and only when eviction is enabled (see
[Eviction](#eviction)) — and it reconverges once gossip catches up. What it does
not provide is a hard global ceiling at any single instant.

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
`PeerDiscoverer` interface. Two implementations are provided: a static one for a
fixed peer list, and a DNS one that resolves a name (e.g. a Kubernetes headless
Service) to the current set of peer IPs.

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
