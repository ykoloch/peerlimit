package peerlimit

import (
	"context"
	"sync"
	"time"

	"github.com/hashicorp/memberlist"
)

const dnsResolveTimeout = time.Second * 2

type Limiter struct {
	config Config
	store  *store
	ml     *memberlist.Memberlist
	// cancel is for returning from the discovery loop
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

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

func (l *Limiter) Close() {
	l.cancel()
	l.wg.Wait()
	l.ml.Leave(time.Millisecond * 500)
	l.ml.Shutdown()
}

func (l *Limiter) Allow(_ context.Context, k string) bool {
	return l.store.allow(key(k), l.config.Rate, l.config.Burst)
}

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
