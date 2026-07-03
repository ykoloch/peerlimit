package peerlimit

import (
	"context"
	"sync"
	"time"

	"github.com/hashicorp/memberlist"
)

const dnsResolveTimeout = time.Second * 2

// Limiter is a distributed rate limiter. Build one with New and release it with
// Close. It is safe for concurrent use.
type Limiter struct {
	config Config
	store  *store
	ml     *memberlist.Memberlist
	// cancel stops the background discover and sweep loops.
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// New creates a Limiter, joins the gossip cluster via conf.Discoverer, and
// starts the background discover loop (and, when KeyTTL is set, the eviction
// sweep loop). The loops run until Close is called or ctx is cancelled.
func New(ctx context.Context, conf Config) (*Limiter, error) {
	if err := conf.validate(); err != nil {
		return nil, err
	}

	s := newStore(conf.Node)
	ml, err := startGossip(ctx, s, conf)
	if err != nil {
		return nil, err
	}
	limiter := &Limiter{config: conf, store: s, ml: ml}

	loopCtx, cancel := context.WithCancel(ctx)
	limiter.cancel = cancel

	limiter.wg.Go(func() {
		limiter.discoverLoop(loopCtx)
	})

	if conf.KeyTTL > 0 {
		limiter.wg.Go(func() {
			limiter.sweepLoop(loopCtx)
		})
	}

	return limiter, nil
}

// Close stops the background loops and leaves the gossip cluster.
func (l *Limiter) Close() {
	l.cancel()
	l.wg.Wait()
	l.ml.Leave(time.Millisecond * 500)
	l.ml.Shutdown()
}

// Allow reports whether an event for key k is permitted now, recording it when
// it is. The decision is local, from current state, with no network round trip.
// Each key is an independent bucket, so use k to scope the limit — "user:123"
// per client, "global" for one shared limit.
func (l *Limiter) Allow(_ context.Context, k string) bool {
	return l.store.allow(key(k), l.config.Rate, l.config.Burst)
}

// discoverLoop polls the Discoverer and joins any peers it returns.
func (l *Limiter) discoverLoop(ctx context.Context) {
	t := time.NewTicker(l.config.DiscoverInterval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			cctx, cancel := context.WithTimeout(ctx, dnsResolveTimeout)
			seeds, _ := l.config.Discoverer.Discover(cctx)
			cancel()
			if len(seeds) > 0 {
				// TODO: log error?
				_, _ = l.ml.Join(seeds)
			}
		}
	}
}

// sweepLoop evicts keys idle past KeyTTL on every SweepInterval tick.
func (l *Limiter) sweepLoop(ctx context.Context) {
	t := time.NewTicker(l.config.SweepInterval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			l.store.sweep(l.config.KeyTTL)
		}
	}
}
