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
			Rate:         10,
			Burst:        50,
			Discovery:    struct{}{},
			SyncInterval: time.Second,
		},
		wantErr: false,
	},
	{
		name: "no node",
		config: Config{
			Rate:         10,
			Burst:        50,
			Discovery:    struct{}{},
			SyncInterval: time.Second,
		},
		wantErr: true,
	},
	{
		name: "no rate",
		config: Config{
			Node:         Node1,
			Burst:        50,
			Discovery:    struct{}{},
			SyncInterval: time.Second,
		},
		wantErr: true,
	},
	{
		name: "no burst",
		config: Config{
			Node:         Node1,
			Rate:         10,
			Discovery:    struct{}{},
			SyncInterval: time.Second,
		},
		wantErr: true,
	},
	{
		name: "no discovery",
		config: Config{
			Node:         Node1,
			Rate:         10,
			Burst:        50,
			SyncInterval: time.Second,
		},
		wantErr: true,
	},
	{
		name: "no sync interval",
		config: Config{
			Node:      Node1,
			Rate:      10,
			Burst:     50,
			Discovery: struct{}{},
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
