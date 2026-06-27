package peerlimit

import (
	"context"
	"time"

	"github.com/hashicorp/memberlist"
)

type Limiter struct {
	config Config
	store  *store
	ml     *memberlist.Memberlist
}

func New(conf Config) (*Limiter, error) {
	if err := conf.validate(); err != nil {
		return nil, err
	}

	s := newStore(conf.Node)
	ml, err := startGossip(s, conf)
	if err != nil {
		return nil, err
	}
	return &Limiter{config: conf, store: s, ml: ml}, nil
}

func (l *Limiter) Close(){
	l.ml.Leave(time.Millisecond * 500)
	l.ml.Shutdown()
}

func (l *Limiter) Allow(_ context.Context, k string) bool {
	return l.store.allow(key(k), l.config.Rate, l.config.Burst)
}
