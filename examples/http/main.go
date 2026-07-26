// Command http demonstrates using peerlimit as net/http middleware.
//
// It starts a single-node limiter (no peers) and serves one endpoint behind the
// httpmw rate limiter. Run it, then hammer the endpoint to watch it flip to 429
// once the burst is spent:
//
//	go run ./examples/http
//	for i in $(seq 30); do curl -s -o /dev/null -w "%{http_code}\n" localhost:8080/; done
package main

import (
	"context"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/ykoloch/peerlimit"
	"github.com/ykoloch/peerlimit/httpmw"
)

func main() {
	ctx := context.Background()

	lim, err := peerlimit.New(ctx, peerlimit.Config{
		Node:             "example-node",
		BindPort:         7946,
		Discoverer:       peerlimit.NewStaticDiscoverer(), // no peers: a lone node
		Rate:             5,                               // 5 tokens added per second
		Burst:            10,                              // bucket holds at most 10
		SyncInterval:     200 * time.Millisecond,
		DiscoverInterval: 10 * time.Second,
		LogOutput:        os.Stderr,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer lim.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "ok\n")
	})

	// httpmw.New(...) returns the middleware; calling it on mux returns the
	// wrapped handler. Every request now passes the limiter before reaching mux.
	limit := httpmw.New(lim, clientKey)

	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", limit(mux)))
}

// clientKey chooses what the limit is scoped to — this is the policy. Here: per
// API key when the caller sends one, otherwise per client IP.
//
// RemoteAddr is "host:port", so strip the port; otherwise every new connection
// gets a fresh port and thus a fresh bucket, and the limit means nothing. Behind
// a reverse proxy RemoteAddr is the proxy's address — you'd parse a trusted
// X-Forwarded-For instead, which is why httpmw ships no default keyFunc.
func clientKey(r *http.Request) string {
	if k := r.Header.Get("X-API-Key"); k != "" {
		return "key:" + k
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	return "ip:" + host
}
