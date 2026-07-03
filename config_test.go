package peerlimit

import (
	"context"
	"testing"
	"time"
)

// stubDiscoverer is a no-op PeerDiscoverer used to satisfy Config validation
// in tests that are not about discovery itself.
type stubDiscoverer struct{}

func (stubDiscoverer) Discover(context.Context) ([]string, error) { return nil, nil }

type configTest struct {
	name    string
	config  Config
	wantErr bool
}

var configTests = []configTest{
	{
		name: "correct",
		config: Config{
			Node:             Node1,
			BindPort:         8080,
			Discoverer:       stubDiscoverer{},
			Rate:             10,
			Burst:            50,
			SyncInterval:     time.Second,
			DiscoverInterval: time.Second,
		},
		wantErr: false,
	},
	{
		name: "no bind port",
		config: Config{
			Node:         Node1,
			Discoverer:   stubDiscoverer{},
			Rate:         10,
			Burst:        50,
			SyncInterval: time.Second,
		},
		wantErr: true,
	},
	{
		name: "no node",
		config: Config{
			BindPort:     8081,
			Discoverer:   stubDiscoverer{},
			Rate:         10,
			Burst:        50,
			SyncInterval: time.Second,
		},
		wantErr: true,
	},
	{
		name: "no discoverer",
		config: Config{
			Node:         Node1,
			BindPort:     8082,
			Rate:         10,
			Burst:        50,
			SyncInterval: time.Second,
		},
		wantErr: true,
	},
	{
		name: "no rate",
		config: Config{
			Node:         Node1,
			BindPort:     8083,
			Discoverer:   stubDiscoverer{},
			Burst:        50,
			SyncInterval: time.Second,
		},
		wantErr: true,
	},
	{
		name: "no burst",
		config: Config{
			Node:         Node1,
			BindPort:     8084,
			Discoverer:   stubDiscoverer{},
			Rate:         10,
			SyncInterval: time.Second,
		},
		wantErr: true,
	},
	{
		name: "no sync interval",
		config: Config{
			Node:       Node1,
			BindPort:   8085,
			Discoverer: stubDiscoverer{},
			Rate:       10,
			Burst:      50,
		},
		wantErr: true,
	},
	{
		name: "no discovery interval",
		config: Config{
			Node:         Node1,
			BindPort:     8080,
			Discoverer:   stubDiscoverer{},
			Rate:         10,
			Burst:        50,
			SyncInterval: time.Second,
		},
		wantErr: true,
	},
	{
		name: "eviction enabled: key ttl and sweep interval both set",
		config: Config{
			Node:             Node1,
			BindPort:         8086,
			Discoverer:       stubDiscoverer{},
			Rate:             10,
			Burst:            50,
			SyncInterval:     time.Second,
			DiscoverInterval: time.Second,
			KeyTTL:           time.Minute,
			SweepInterval:    time.Second,
		},
		wantErr: false,
	},
	{
		name: "key ttl without sweep interval",
		config: Config{
			Node:             Node1,
			BindPort:         8087,
			Discoverer:       stubDiscoverer{},
			Rate:             10,
			Burst:            50,
			SyncInterval:     time.Second,
			DiscoverInterval: time.Second,
			KeyTTL:           time.Minute,
		},
		wantErr: true,
	},
	{
		name: "sweep interval without key ttl",
		config: Config{
			Node:             Node1,
			BindPort:         8088,
			Discoverer:       stubDiscoverer{},
			Rate:             10,
			Burst:            50,
			SyncInterval:     time.Second,
			DiscoverInterval: time.Second,
			SweepInterval:    time.Second,
		},
		wantErr: true,
	},
	{
		name:    "totally empty",
		wantErr: true,
	},
}

func TestValidate(t *testing.T) {
	for _, tt := range configTests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.validate()
			result := err != nil
			if result != tt.wantErr {
				t.Fatalf("wantErr=%v, got err=%v", tt.wantErr, err)
			}
		})
	}
}
