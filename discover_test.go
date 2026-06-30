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

// TestDiscover_DNSDiscoverer_Unresolvable verifies that an unresolvable name
// surfaces the resolver error instead of being swallowed into an empty result.
//
// The .invalid TLD is reserved by RFC 6761 and never resolves, so this normally
// yields NXDOMAIN. A resolver that hijacks NXDOMAIN (some corporate/ISP setups)
// could return an address and make this flaky; accepted until we classify DNS
// errors (NotFound vs real failure).
func TestDiscover_DNSDiscoverer_Unresolvable(t *testing.T) {
	d := NewDNSDiscoverer("peerlimit-does-not-exist.invalid", dPort)
	seeds, err := d.Discover(context.Background())
	if err == nil {
		t.Fatalf("discover should error on an unresolvable name, got seeds: %v", seeds)
	}
	if seeds != nil {
		t.Errorf("seeds should be nil on error, got: %v", seeds)
	}
}
