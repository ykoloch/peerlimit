package peerlimit

import (
	"context"
	"net"
	"testing"
)

const dPort = "7946"

// TestDiscover_DNSDiscoverer_Happy verifies the core glue: Discover resolves a
// name and attaches the configured port to every returned address.
//
// It uses localhost because that name is guaranteed to exist in /etc/hosts on
// every OS, so it always resolves. Assertions are tolerant on purpose: the
// resolver may return IPv4, IPv6, or both in any order, so we check a property
// (every entry carries dPort) rather than an exact list.
func TestDiscover_DNSDiscoverer_Happy(t *testing.T) {
	d := NewDNSDiscoverer("localhost", dPort)
	seeds, err := d.Discover(context.Background())
	if err != nil {
		t.Fatalf("discover should not return error,  got: %v", err)
	}
	if len(seeds) < 1 {
		t.Fatal("empty set of addresses returned")
	}

	for _, s := range seeds {
		_, p, err := net.SplitHostPort(s)
		if err != nil {
			t.Fatalf("address should be splitted without error,  got: %v", err)
		}
		if p != dPort {
			t.Fatalf("ports don't match. expected %v, got %v", dPort, p)
		}
	}
}

// TestDiscover_DNSDiscoverer_NotFoundIsEmpty verifies that a name with no
// records is treated as "no peers yet" — an empty result with no error, not a
// failure. This is the cold-start case: a headless service with zero ready
// endpoints resolves to nothing.
//
// The .invalid TLD is reserved by RFC 6761 and never resolves, so it yields
// NXDOMAIN, which Go surfaces as *net.DNSError with IsNotFound set. A resolver
// that hijacks NXDOMAIN (some corporate/ISP setups) could return an address and
// make this flaky; accepted as an environment trade-off.
func TestDiscover_DNSDiscoverer_NotFoundIsEmpty(t *testing.T) {
	d := NewDNSDiscoverer("peerlimit-does-not-exist.invalid", dPort)
	seeds, err := d.Discover(context.Background())
	if err != nil {
		t.Fatalf("not-found should not error, got: %v", err)
	}
	if len(seeds) != 0 {
		t.Errorf("not-found should yield no seeds, got: %v", seeds)
	}
}
