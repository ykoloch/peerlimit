package peerlimit

import (
	"context"
	"testing"
)

const (
	userID = "user:1"
	burst  = 50
)

func TestAllow_NewKeyPasses(t *testing.T) {
	config := Config{
		Bucket: TokenBucket{
			RefillRate: 10,
			Burst:      burst,
		},
		Discovery:    struct{}{},
		SyncInterval: 1,
	}
	l, err := New(config)
	if err != nil {
		t.Fatalf("got error: %v", err)
	}
	result := l.Allow(context.TODO(), userID)
	if !result {
		t.Error("Allow on fresh key=false, want true")
	}
}

func TestAllow_BurstExhaustion(t *testing.T) {
	config := Config{
		Bucket: TokenBucket{
			RefillRate: 5,
			Burst:      burst,
		},
		Discovery:    struct{}{},
		SyncInterval: 1,
	}
	l, err := New(config)
	if err != nil {
		t.Fatalf("got error: %v", err)
	}

	for i := range int(l.config.Bucket.Burst) {
		if !l.Allow(context.TODO(), userID) {
			t.Fatalf("all requests under bucket capacity should be allowed, rejected: %v", i)
		}
	}
	if l.Allow(context.TODO(), userID) {
		t.Error("request above capacity = true, wnat false")
	}
}
