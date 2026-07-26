package httpmw_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ykoloch/peerlimit"
	"github.com/ykoloch/peerlimit/httpmw"
)

// portSeq hands out a unique BindPort per limiter so several memberlist
// instances in one test binary never collide on a port.
var portSeq atomic.Int32

// newLimiter builds a lone-node limiter for tests. A static discoverer with no
// peers means it bootstraps without joining anyone, and t.Cleanup tears down
// the memberlist listeners when the test ends.
func newLimiter(t *testing.T, rate, burst float64) *peerlimit.Limiter {
	t.Helper()
	lim, err := peerlimit.New(context.Background(), peerlimit.Config{
		Node:             "test-node",
		BindPort:         7100 + int(portSeq.Add(1)),
		Discoverer:       peerlimit.NewStaticDiscoverer(),
		Rate:             rate,
		Burst:            burst,
		SyncInterval:     time.Second,
		DiscoverInterval: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("peerlimit.New: %v", err)
	}
	t.Cleanup(lim.Close)
	return lim
}

// serve runs one GET request through h and returns the recorded response.
func serve(h http.Handler) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	h.ServeHTTP(rec, req)
	return rec
}

func TestNew_AllowsUnderLimit(t *testing.T) {
	lim := newLimiter(t, 100, 10)

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})
	key := func(r *http.Request) string { return "user:1" }

	rec := serve(httpmw.New(lim, key)(next))

	if !nextCalled {
		t.Error("next handler was not called for an allowed request")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestNew_RejectsOverLimit(t *testing.T) {
	lim := newLimiter(t, 1, 1)

	const k = "user:1"
	// Drain the bucket so the middleware's decision must be a rejection.
	// Rate is 1 token/s, far slower than this loop, so it empties and stops.
	for lim.Allow(context.Background(), k) {
	}

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	})
	key := func(r *http.Request) string { return k }

	rec := serve(httpmw.New(lim, key)(next))

	if nextCalled {
		t.Error("next handler was called for a rejected request")
	}
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
}

func TestNew_PanicsOnNilKeyFunc(t *testing.T) {
	// The nil check runs before lim is touched, so a nil limiter is fine here.
	defer func() {
		if recover() == nil {
			t.Error("New with a nil KeyFunc did not panic")
		}
	}()
	httpmw.New(nil, nil)
}
