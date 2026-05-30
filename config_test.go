package peerlimit

import (
	"testing"
	"time"
)

type test struct {
	name    string
	config  Config
	wantErr bool
}

var tests = []test{
	{
		name: "correct",
		config: Config{Bucket: TokenBucket{
			RefillRate: 10,
			Burst:      50,
		},
			Discovery:    struct{}{},
			SyncInterval: time.Second,
		},
		wantErr: false,
	},
	{
		name: "no bucket",
		config: Config{
			Discovery:    struct{}{},
			SyncInterval: time.Second,
		},
		wantErr: true,
	},
	{
		name: "no bucket refill rate",
		config: Config{Bucket: TokenBucket{
			Burst: 50,
		},
			Discovery:    struct{}{},
			SyncInterval: time.Second,
		},
		wantErr: true,
	},
	{
		name: "no bucket burst",
		config: Config{Bucket: TokenBucket{
			RefillRate: 10,
		},
			Discovery:    struct{}{},
			SyncInterval: time.Second,
		},
		wantErr: true,
	},
	{
		name: "no discovery",
		config: Config{Bucket: TokenBucket{
			Burst:      50,
			RefillRate: 10,
		},
			SyncInterval: time.Second,
		},
		wantErr: true,
	},
	{
		name: "no sync interval",
		config: Config{Bucket: TokenBucket{
			Burst:      50,
			RefillRate: 10,
		},
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
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.validate()
			result := err != nil
			if result != tt.wantErr {
				t.Fatalf("wantErr=%v, got err=%v", tt.wantErr, err)
			}
		})
	}
}
