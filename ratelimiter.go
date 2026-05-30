package peerlimit

import (
	"context"
	"sync"
	"time"
)

type Limiter struct {
	buckets map[string]*bucketState
	config  Config
	mu      sync.Mutex
}

type bucketState struct {
	tokens     float64
	lastRefill time.Time
}

func New(conf Config) (*Limiter, error) {
	if err := conf.validate(); err != nil {
		return nil, err
	}
	return &Limiter{buckets: make(map[string]*bucketState), config: conf}, nil
}

// Allow reports whether an event for the given key may proceed under the
// configured token-bucket limit. It refills the key's bucket based on the time
// elapsed since the last call (capped at Burst), then consumes one token if at
// least one is available.
//
// The first request for a previously unseen key starts from a full bucket.
// Allow is safe for concurrent use by multiple goroutines.
//
// The ctx argument is currently unused and reserved for future
// cancellation/deadline support.
func (l *Limiter) Allow(_ context.Context, key string) bool {
	l.mu.Lock()
	bucket, ok := l.buckets[key]
	defer l.mu.Unlock()
	if !ok {
		bucket = &bucketState{
			tokens:     l.config.Bucket.Burst - 1,
			lastRefill: time.Now(),
		}
		l.buckets[key] = bucket
		return true
	}
	refilled := time.Since(bucket.lastRefill).Seconds() * l.config.Bucket.RefillRate
	bucket.tokens = min(bucket.tokens+refilled, l.config.Bucket.Burst)
	bucket.lastRefill = time.Now()
	if bucket.tokens >= 1 {
		bucket.tokens--
		return true
	}
	return false
}
