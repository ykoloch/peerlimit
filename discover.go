package peerlimit

import (
	"context"
	"errors"
	"net"
)

// PeerDiscoverer returns the current peer addresses ("host:port") to join. It is
// called once at startup and then polled periodically, so the set may change as
// peers come and go.
type PeerDiscoverer interface {
	Discover(context.Context) ([]string, error)
}

type staticDiscoverer struct {
	peers []string
}

// NewStaticDiscoverer returns a PeerDiscoverer that always yields addrs. Use it
// for a fixed peer list known at startup.
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

// NewDNSDiscoverer returns a PeerDiscoverer that resolves host to its addresses
// and appends port to each — e.g. a Kubernetes headless service. A name that
// does not resolve yields no peers rather than an error.
func NewDNSDiscoverer(host, port string) PeerDiscoverer {
	return &dnsDiscoverer{
		host: host,
		port: port,
	}
}

func (dd *dnsDiscoverer) Discover(ctx context.Context) ([]string, error) {
	addrs, err := net.DefaultResolver.LookupHost(ctx, dd.host)
	if err != nil {
		var dnsErr *net.DNSError
		if errors.As(err, &dnsErr) && dnsErr.IsNotFound {
			return nil, nil
		}

		return nil, err
	}

	for i, a := range addrs {
		addrs[i] = net.JoinHostPort(a, dd.port)
	}
	return addrs, nil
}
