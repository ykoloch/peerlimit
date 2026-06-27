package peerlimit

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"
)

const (
	userID  = "user:1"
	user2ID = "user:2"
	burst   = 50
)

func TestAllow_NewKeyPasses(t *testing.T) {
	config := Config{
		Node:         Node1,
		BindPort:     8080,
		Seeds:        []string{"localhost:8080"},
		Rate:         10,
		Burst:        burst,
		SyncInterval: time.Second,
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
		Node:         Node1,
		BindPort:     8080,
		Seeds:        []string{"localhost:8080"},
		Rate:         5,
		Burst:        burst,
		SyncInterval: time.Second,
	}
	l, err := New(config)
	if err != nil {
		t.Fatalf("got error: %v", err)
	}

	for i := range int(l.config.Burst) {
		if !l.Allow(context.TODO(), userID) {
			t.Fatalf("all requests under bucket capacity should be allowed, rejected: %v", i)
		}
	}
	if l.Allow(context.TODO(), userID) {
		t.Error("request above capacity = true, want false")
	}
}

func TestAllow_Refill(t *testing.T) {
	config := Config{
		Node:         Node1,
		BindPort:     8080,
		Seeds:        []string{"localhost:8080"},
		Rate:         5,
		Burst:        burst,
		SyncInterval: time.Second,
	}
	l, err := New(config)
	if err != nil {
		t.Fatalf("got error: %v", err)
	}

	// exhaust bucket
	for range int(l.config.Burst) {
		l.Allow(context.TODO(), userID)
	}
	if l.Allow(context.TODO(), userID) {
		t.Fatal("bucket should be empty after exhaustion")
	}
	// wait for some tokens to be added
	time.Sleep(time.Millisecond * 300)
	if !l.Allow(context.TODO(), userID) {
		t.Error("request after refill = false, want true")
	}
}

func TestAllow_BucketsIsolated(t *testing.T) {
	config := Config{
		Node:         Node1,
		BindPort:     8080,
		Seeds:        []string{"localhost:8080"},
		Rate:         5,
		Burst:        burst,
		SyncInterval: time.Second,
	}
	l, err := New(config)
	if err != nil {
		t.Fatalf("got error: %v", err)
	}

	// exhaust bucket for user_1
	for range int(l.config.Burst) {
		l.Allow(context.TODO(), userID)
	}
	if l.Allow(context.TODO(), userID) {
		t.Fatal("bucket should be empty after exhaustion")
	}
	// fire request from user_2
	if !l.Allow(context.TODO(), user2ID) {
		t.Error("request for user_2 = false, want true")
	}
}

func TestAllow_RaceSingleKey(t *testing.T) {
	config := Config{
		Node:         Node1,
		BindPort:     8080,
		Seeds:        []string{"localhost:8080"},
		Rate:         5,
		Burst:        burst,
		SyncInterval: time.Second,
	}
	l, err := New(config)
	if err != nil {
		t.Fatalf("got error: %v", err)
	}

	workers := 100
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for range 100 {
				l.Allow(context.TODO(), userID)
			}
		})
	}
	wg.Wait()
}

func TestAllow_RaceMultKey(t *testing.T) {
	config := Config{
		Node:         Node1,
		BindPort:     8080,
		Seeds:        []string{"localhost:8080"},
		Rate:         5,
		Burst:        burst,
		SyncInterval: time.Second,
	}
	l, err := New(config)
	if err != nil {
		t.Fatalf("got error: %v", err)
	}

	workers := 100
	var wg sync.WaitGroup
	for i := range workers {
		wg.Go(func() {
			for range 100 {
				l.Allow(context.TODO(), strconv.Itoa(i))
			}
		})
	}
	wg.Wait()
}
