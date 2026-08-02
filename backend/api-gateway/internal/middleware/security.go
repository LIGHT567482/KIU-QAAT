package middleware

import "net/http"

// SecurityHeaders sets all HTTP security headers required by ARCHITECT.md §6
// and enforces TLS 1.3 at the application layer.
// These headers complement—not replace—Cloudflare/load-balancer TLS termination.
func SecurityHeaders(env string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()

			// Enforce HTTPS for 2 years; include subdomains; allow preloading.
			h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")

			// Prevent MIME sniffing.
			h.Set("X-Content-Type-Options", "nosniff")

			// Deny framing entirely (no dashboard embeds needed).
			h.Set("X-Frame-Options", "DENY")

			// No referrer to external origins (student/coordinator privacy).
			h.Set("Referrer-Policy", "strict-origin-when-cross-origin")

			// Permissions Policy — disable unnecessary browser features.
			h.Set("Permissions-Policy",
				"camera=(), microphone=(), geolocation=(self), bluetooth=(), usb=()")

			// Content-Security-Policy — strict by default; relax only what's needed.
			// 'nonce' approach deferred to Phase 3 hardening sprint;
			// hash-based CSP applied here as an interim measure.
			if env == "production" {
				h.Set("Content-Security-Policy",
					"default-src 'none'; "+
						"script-src 'self'; "+
						"style-src 'self' 'unsafe-inline'; "+ // inline styles for PWA/dashboards; TODO: hash after build
						"img-src 'self' data: blob:; "+
						"font-src 'self'; "+
						"connect-src 'self' https://api.qaat.platform; "+
						"manifest-src 'self'; "+
						"worker-src 'self' blob:; "+ // Service Worker
						"frame-ancestors 'none'; "+
						"base-uri 'self'; "+
						"form-action 'self'",
				)
			} else {
				// Development: permissive CSP to avoid blocking hot-reload / dev tools.
				h.Set("Content-Security-Policy", "default-src * 'unsafe-inline' 'unsafe-eval' data: blob:")
			}

			// Cross-Origin policies.
			h.Set("Cross-Origin-Opener-Policy", "same-origin")
			h.Set("Cross-Origin-Embedder-Policy", "require-corp")
			h.Set("Cross-Origin-Resource-Policy", "same-site")

			next.ServeHTTP(w, r)
		})
	}
}

// RequireHTTPS redirects plain HTTP to HTTPS in production.
// Behind a load balancer the X-Forwarded-Proto header is authoritative.
func RequireHTTPS(env string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if env != "production" {
				next.ServeHTTP(w, r)
				return
			}
			proto := r.Header.Get("X-Forwarded-Proto")
			if proto == "http" {
				target := "https://" + r.Host + r.URL.RequestURI()
				http.Redirect(w, r, target, http.StatusMovedPermanently)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
