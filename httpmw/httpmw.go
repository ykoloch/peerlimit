// Package httpmw adapts a peerlimit.Limiter to standard net/http middleware.
//
// It deliberately lives outside the core package: the core never imports
// httpmw, so callers who only need Allow never pull in any HTTP wiring. The
// middleware is a plain func(http.Handler) http.Handler and works with
// net/http, chi, and any router built on the standard http.Handler interface.
package httpmw

import (
	"net/http"

	"github.com/ykoloch/peerlimit"
)

// KeyFunc derives the rate-limit key from a request. The key selects the
// limiting model: "user:"+id limits per client, "global" applies one shared
// limit. There is no default on purpose — a silent per-IP default breaks behind
// a proxy, where every client shares the load balancer's address and would be
// limited as one. The caller must choose.
type KeyFunc func(r *http.Request) string

// New wraps lim as net/http middleware that rejects requests over the limit
// with 429 Too Many Requests and passes the rest to the next handler.
//
// key is required; New panics if it is nil so the mistake surfaces at startup
// rather than as a nil dereference on the first request.
func New(lim *peerlimit.Limiter, key KeyFunc) func(http.Handler) http.Handler {
	if key == nil {
		panic("httpmw: KeyFunc must be provided")
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if lim.Allow(r.Context(), key(r)) {
				next.ServeHTTP(w, r)
				return
			}
			http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
		})
	}
}
