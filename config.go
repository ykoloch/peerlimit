package peerlimit

import (
	"errors"
	"time"
)

type TokenBucket struct {
	RefillRate float64
	Burst      float64
}

type Config struct {
	Bucket       TokenBucket
	Discovery    any
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
