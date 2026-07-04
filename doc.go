// Package peerlimit is a distributed rate limiter that runs as an embedded
// library rather than a separate service. Each process enforces the limit from
// its own in-memory state and decides without a network round trip; peers keep
// those states approximately in sync by gossiping their counts. There is no
// shared datastore and no central coordinator, so no single failure disables
// limiting.
//
// The trade-off is accuracy: under partition or gossip lag a limit may be
// briefly exceeded. peerlimit suits abuse protection, noisy-tenant throttling
// and soft API limits — not billing-grade quotas.
package peerlimit
