package peerlimit

import (
	"context"
	"testing"
	"time"
)

func TestGossip_Converges(t *testing.T) {
	seeds := []string{"localhost:8080"}
	limA, err := New(
		Config{
			Node:         Node1,
			BindPort:     8080,
			Rate:         10,
			Burst:        burst,
			SyncInterval: time.Millisecond * 20,
		},
	)
	if err != nil {
		t.Fatalf("got error: %v", err)
	}

	limB, err := New(
		Config{
			Seeds:        seeds,
			Node:         Node2,
			BindPort:     8081,
			Rate:         10,
			Burst:        burst,
			SyncInterval: time.Millisecond * 20,
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

	gotA :=  limA.store.aggregate(userID)
	if gotA != 8{
		t.Fatalf("limiter A should have aggregate 8, got %v", gotA)
	}
	gotB :=  limB.store.aggregate(userID)
	if gotB != 8{
		t.Fatalf("limiter B should have aggregate 8, got %v", gotB)
	}

	defer limA.Close()
	defer limB.Close()
}
