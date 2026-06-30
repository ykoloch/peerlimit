package peerlimit

import (
	"context"
	"net"
)

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

type dnsDiscoverer struct {
	host string
	port string
}

func NewDNSDiscoverer(host, port string) PeerDiscoverer {
	return &dnsDiscoverer{
		host: host,
		port: port,
	}
}

func (dd *dnsDiscoverer) Discover(ctx context.Context) ([]string, error) {
	addrs, err := net.DefaultResolver.LookupHost(ctx, dd.host)
	if err != nil {
		return nil, err
	}

	for i, a := range addrs {
		addrs[i] = net.JoinHostPort(a, dd.port)
	}
	return addrs, nil
}
