package middleware

import (
	"encoding/json"
	"net/http"
	"sync"

	"golang.org/x/time/rate"
)

// CoordinatorRateLimit enforces 50 req/s per coordinator (burst 100) as
// specified in technicaldoc.md §5.3.
// Non-coordinator roles use a global per-IP limiter (200 req/s, burst 400).
func CoordinatorRateLimit() func(http.Handler) http.Handler {
	var (
		mu           sync.Mutex
		coordinators = make(map[string]*rate.Limiter)
		global       = make(map[string]*rate.Limiter)
	)

	getCoordinatorLimiter := func(id string) *rate.Limiter {
		mu.Lock()
		defer mu.Unlock()
		l, ok := coordinators[id]
		if !ok {
			l = rate.NewLimiter(50, 100)
			coordinators[id] = l
		}
		return l
	}

	getGlobalLimiter := func(ip string) *rate.Limiter {
		mu.Lock()
		defer mu.Unlock()
		l, ok := global[ip]
		if !ok {
			l = rate.NewLimiter(200, 400)
			global[ip] = l
		}
		return l
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var limiter *rate.Limiter
			if GetRole(r.Context()) == RoleCoordinator {
				limiter = getCoordinatorLimiter(GetUserID(r.Context()))
			} else {
				limiter = getGlobalLimiter(r.RemoteAddr)
			}

			if !limiter.Allow() {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				json.NewEncoder(w).Encode(map[string]string{ //nolint:errcheck
					"error":   "RATE_LIMIT_EXCEEDED",
					"message": "too many requests",
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// PublicIPRateLimit throttles an unauthenticated public endpoint per client IP.
// Used for the student check-in route, which has no JWT to key on. Check-in is a
// human action, so a few requests/second per IP is generous; the cap exists only
// to blunt scripted abuse / room-code brute force (the signed-QR gate already
// restricts attempts to holders of a real, enrolled QR).
func PublicIPRateLimit(perSec rate.Limit, burst int) func(http.Handler) http.Handler {
	var (
		mu  sync.Mutex
		ips = make(map[string]*rate.Limiter)
	)
	get := func(ip string) *rate.Limiter {
		mu.Lock()
		defer mu.Unlock()
		l, ok := ips[ip]
		if !ok {
			l = rate.NewLimiter(perSec, burst)
			ips[ip] = l
		}
		return l
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !get(r.RemoteAddr).Allow() {
				// Retry-After lets a client back off intelligently instead of hammering or
				// giving up. This matters most on sign-in: a whole cohort opening the app in a
				// lecture hall arrives from ONE public IP (the campus NAT), so they share a
				// bucket and the ones at the back are refused through no fault of their own.
				// Without this header the app had nothing to distinguish "wrong password" from
				// "wait two seconds", and showed the student a credentials error for neither.
				w.Header().Set("Retry-After", "2")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				json.NewEncoder(w).Encode(map[string]string{ //nolint:errcheck
					"error":   "RATE_LIMIT_EXCEEDED",
					"message": "Too many people are signing in at once. Wait a moment and try again.",
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
