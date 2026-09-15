package handler

import (
	"net"
	"net/http"
	"sync"

	"golang.org/x/time/rate"
)

// IPRateLimiter hands out one rate.Limiter per client IP instead of a single
// shared limiter for the whole server. A single global *rate.Limiter (as this
// used to be) means one busy or malicious client can exhaust the entire
// bucket and lock every other user out of login/register for everyone else,
// which is exactly the kind of "login sometimes just stops working" bug this
// was causing. Keying by IP scopes the limit to where it belongs.
type IPRateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
	r        rate.Limit
	b        int
}

func NewIPRateLimiter(r rate.Limit, b int) *IPRateLimiter {
	return &IPRateLimiter{
		limiters: make(map[string]*rate.Limiter),
		r:        r,
		b:        b,
	}
}

func (l *IPRateLimiter) getLimiter(key string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()

	lim, ok := l.limiters[key]
	if !ok {
		lim = rate.NewLimiter(l.r, l.b)
		l.limiters[key] = lim
	}
	return lim
}

// Allow reports whether a request from r's client IP may proceed.
func (l *IPRateLimiter) Allow(r *http.Request) bool {
	return l.getLimiter(clientKey(r)).Allow()
}

func clientKey(r *http.Request) string {
	// chimw.RealIP (registered ahead of this in the middleware chain) already
	// rewrites RemoteAddr from X-Forwarded-For/X-Real-IP when present.
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
