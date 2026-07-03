package peerlimit

import (
	"errors"
	"io"
	"time"
)

// Config holds the parameters for a Limiter and is passed to New.
type Config struct {
	// Node is this process's identity in the cluster. It must be unique and
	// stable for the process's lifetime: it names this node's own G-Counter cell.
	Node nodeID
	// BindPort is the port memberlist listens on for gossip.
	BindPort int
	// Discoverer supplies the peer addresses to join, at startup and on every
	// DiscoverInterval tick.
	Discoverer PeerDiscoverer

	// Rate is the sustained refill rate, in tokens per second.
	Rate float64
	// Burst is the bucket capacity: the most tokens available at once.
	Burst float64

	// KeyTTL is how long a key may sit idle before it is evicted from local
	// state. Zero disables eviction. Must be set together with SweepInterval.
	KeyTTL time.Duration
	// SweepInterval is how often idle keys are checked against KeyTTL. Zero
	// disables eviction. Must be set together with KeyTTL.
	SweepInterval time.Duration

	// SyncInterval is how often this node gossips its state to peers and merges
	// theirs. Must be positive.
	SyncInterval time.Duration
	// DiscoverInterval is how often Discoverer is polled for peers. Must be positive.
	DiscoverInterval time.Duration

	// LogOutput receives memberlist's internal logs. Nil discards them.
	LogOutput io.Writer
}

func (c Config) validate() error {
	var errs []error
	if c.Node == "" {
		errs = append(errs, errors.New("node id is not provided"))
	}
	if c.BindPort == 0 {
		errs = append(errs, errors.New("bind port is not provided"))
	}
	if c.Rate < 1 {
		errs = append(errs, errors.New("rate should be positive"))
	}
	if c.Burst < 1 {
		errs = append(errs, errors.New("burst should be positive"))
	}
	if c.SyncInterval < 1 {
		errs = append(errs, errors.New("sync interval should be positive"))
	}
	if c.DiscoverInterval < 1 {
		errs = append(errs, errors.New("discover interval should be positive"))
	}
	if (c.KeyTTL > 0) != (c.SweepInterval > 0) {
		errs = append(errs, errors.New("key TTL and sweep interval must be set together or left unset"))
	}
	if c.Discoverer == nil {
		errs = append(errs, errors.New("discoverer can not be empty"))
	}
	return errors.Join(errs...)
}
