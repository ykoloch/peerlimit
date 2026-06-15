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
	consumed  float64
	createdAt time.Time
}

func (b *bucketState) allow(now time.Time, rate, burst float64) bool {
	allowance := burst + now.Sub(b.createdAt).Seconds()*rate
	// at least one whole token should be available
	if b.consumed+1 <= allowance {
		b.consumed++
		return true
	}
	return false
}

func New(conf Config) (*Limiter, error) {
	if err := conf.validate(); err != nil {
		return nil, err
	}
	return &Limiter{buckets: make(map[string]*bucketState), config: conf}, nil
}

func (l *Limiter) Allow(_ context.Context, key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	bucket, ok := l.buckets[key]
	if !ok {
		bucket = &bucketState{createdAt: time.Now()}
		l.buckets[key] = bucket
	}
	return bucket.allow(time.Now(), l.config.Bucket.RefillRate, l.config.Bucket.Burst)
}
