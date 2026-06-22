package peerlimit

import (
	"context"
)

type Limiter struct {
	config Config
	store  *store
}

func New(conf Config) (*Limiter, error) {
	if err := conf.validate(); err != nil {
		return nil, err
	}
	return &Limiter{config: conf, store: newStore(conf.Node)}, nil
}

func (l *Limiter) Allow(_ context.Context, k string) bool {
	return l.store.allow(key(k), l.config.Rate, l.config.Burst)
}
