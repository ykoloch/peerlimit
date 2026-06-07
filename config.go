package peerlimit

import (
	"errors"
	"time"
)

// TokenBucket configures the token-bucket rate limit applied per key.
//
// Each key gets its own bucket that holds up to Burst tokens and refills
// continuously at RefillRate tokens per second. A request consumes one token;
// it is allowed while at least one token is available and rejected otherwise.
// A previously unseen key starts with a full bucket, so it can immediately
// absorb a burst of up to Burst requests.
type TokenBucket struct {
	// RefillRate is the steady-state limit, in tokens (requests) per second,
	// at which a key is allowed once its initial burst has been spent.
	RefillRate float64

	// Burst is the maximum number of tokens a bucket can hold: the largest
	// spike allowed at once (for example after an idle period during which the
	// bucket refilled to full). It also caps refill — tokens never accumulate
	// beyond Burst.
	Burst float64
}

// Config holds the parameters for a Limiter and is passed to New.
type Config struct {
	// Bucket defines the token-bucket limit applied to every key.
	Bucket TokenBucket

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
	if c.Bucket.RefillRate < 1 {
		errs = append(errs, errors.New("token bucket refill rate should be positive"))
	}
	if c.Bucket.Burst < 1 {
		errs = append(errs, errors.New("token bucket burst should be positive"))
	}
	if c.Discovery == nil {
		errs = append(errs, errors.New("discovery is not provided"))
	}
	if c.SyncInterval < 1 {
		errs = append(errs, errors.New("sync interval should be positive"))
	}
	return errors.Join(errs...)
}
