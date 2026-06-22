package peerlimit

import (
	"errors"
	"time"
)

// Config holds the parameters for a Limiter and is passed to New.
type Config struct {
	// TODO: NodeID from memberlist
	Node nodeID

	Rate  float64
	Burst float64
	// Discovery selects how the limiter finds its peer nodes to gossip with.
	// It must be non-nil. The concrete type is not yet finalized (a
	// PeerDiscoverer interface is planned); for now validation only checks
	// that a value is present.
	Discovery any

	// SyncInterval is how often the limiter gossips its local state to peers
	// and merges theirs. It must be positive.
	SyncInterval time.Duration
}

func (c Config) validate() error {
	var errs []error
	if c.Burst < 1 {
		errs = append(errs, errors.New("burst should be positive"))
	}
	if c.Rate < 1 {
		errs = append(errs, errors.New("rate should be positive"))
	}
	if c.Node == "" {
		errs = append(errs, errors.New("node id is not provided"))
	}
	if c.Discovery == nil {
		errs = append(errs, errors.New("discovery is not provided"))
	}
	if c.SyncInterval < 1 {
		errs = append(errs, errors.New("sync interval should be positive"))
	}
	return errors.Join(errs...)
}
