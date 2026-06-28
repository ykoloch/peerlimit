package peerlimit

import (
	"errors"
	"io"
	"time"
)

// Config holds the parameters for a Limiter and is passed to New.
type Config struct {
	Node       nodeID
	BindPort   int
	Discoverer PeerDiscoverer

	Rate  float64
	Burst float64

	// SyncInterval is how often the limiter gossips its local state to peers
	// and merges theirs. It must be positive.
	SyncInterval time.Duration
	LogOutput    io.Writer
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
	if c.Discoverer == nil {
		errs = append(errs, errors.New("discoverer can not be empty"))
	}
	return errors.Join(errs...)
}
