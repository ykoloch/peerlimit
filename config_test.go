package peerlimit

import (
	"testing"
	"time"
)

type configTest struct {
	name    string
	config  Config
	wantErr bool
}

var configTests = []configTest{
	{
		name: "correct",
		config: Config{
			Node:         Node1,
			BindPort:     8080,
			Rate:         10,
			Burst:        50,
			SyncInterval: time.Second,
		},
		wantErr: false,
	},
	{
		name: "no bind port",
		config: Config{
			Node:         Node1,
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
			BindPort:     8082,
			Burst:        50,
			SyncInterval: time.Second,
		},
		wantErr: true,
	},
	{
		name: "no burst",
		config: Config{
			Node:         Node1,
			BindPort:     8083,
			Rate:         10,
			SyncInterval: time.Second,
		},
		wantErr: true,
	},
	{
		name: "no sync interval",
		config: Config{
			Node:     Node1,
			BindPort: 8084,
			Rate:     10,
			Burst:    50,
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
