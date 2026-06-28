package peerlimit

import "context"

type PeerDiscoverer interface {
	Discover(context.Context) ([]string, error)
}

type staticDiscoverer struct {
	peers []string
}

func NewStaticDiscoverer(addrs ...string) PeerDiscoverer {
	return &staticDiscoverer{
		peers: addrs,
	}
}

func (sd *staticDiscoverer) Discover(_ context.Context) ([]string, error) {
	return sd.peers, nil
}
