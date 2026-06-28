package peerlimit

import (
	"context"
	"testing"
	"time"
)

func TestGossip_Converges(t *testing.T) {
	limA, err := New(context.TODO(),
		Config{
			Node:             Node1,
			Discoverer:       NewStaticDiscoverer("localhost:8080", "localhost:8081"),
			BindPort:         8080,
			Rate:             10,
			Burst:            burst,
			SyncInterval:     time.Millisecond * 20,
			DiscoverInterval: time.Second * 5,
		},
	)
	if err != nil {
		t.Fatalf("got error: %v", err)
	}

	limB, err := New(context.TODO(),
		Config{
			Discoverer:       NewStaticDiscoverer("localhost:8080", "localhost:8081"),
			Node:             Node2,
			BindPort:         8081,
			Rate:             10,
			Burst:            burst,
			SyncInterval:     time.Millisecond * 20,
			DiscoverInterval: time.Second * 5,
		},
	)
	if err != nil {
		t.Fatalf("got error: %v", err)
	}

	for range 5 {
		limA.Allow(context.TODO(), userID)
	}

	for range 3 {
		limB.Allow(context.TODO(), userID)
	}

	time.Sleep(time.Millisecond * 300)

	gotA := limA.store.aggregate(userID)
	if gotA != 8 {
		t.Fatalf("limiter A should have aggregate 8, got %v", gotA)
	}
	gotB := limB.store.aggregate(userID)
	if gotB != 8 {
		t.Fatalf("limiter B should have aggregate 8, got %v", gotB)
	}

	defer limA.Close()
	defer limB.Close()
}
