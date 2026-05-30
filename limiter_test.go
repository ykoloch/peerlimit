package peerlimit

import (
	"context"
	"testing"
)

func TestAllow_NewKeyPasses(t *testing.T) {
	config := Config{
		Bucket: TokenBucket{
			RefillRate: 10,
			Burst:      50,
		},
		Discovery:    struct{}{},
		SyncInterval: 1,
	}
	l, err := New(config)
	if err != nil {
		t.Fatalf("got error: %v", err)
	}
	result := l.Allow(context.TODO(), "user:1")
	if !result {
		t.Error("Allow on fresh key=false, want true")
	}
}
