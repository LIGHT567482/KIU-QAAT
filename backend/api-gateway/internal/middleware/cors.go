package middleware

import (
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
)

// isLocalOrigin reports whether an Origin lives on the local machine or a private
// LAN — loopback, a private IPv4 range (10/8, 172.16/12, 192.168/16), or a
// ".local" mDNS host. QAAT is a self-hosted LAN tool whose front-door IP is DHCP
// and changes between networks; allowing local origins lets it run on any local
// network (and fully offline via localhost) without re-pointing CORS each time.
func isLocalOrigin(origin string) bool {
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	host := u.Hostname()
	if host == "localhost" || strings.HasSuffix(host, ".local") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback() || ip.IsPrivate()
	}
	return false
}

// CORS returns a middleware that enforces an allowlist of origins, plus any
// local/LAN origin (see isLocalOrigin) so the tool works offline and on any
// local network. The allowlist still governs non-local (e.g. public) origins.
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && (slices.Contains(allowedOrigins, origin) || isLocalOrigin(origin)) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Vary", "Origin")
			}

			if r.Method == http.MethodOptions {
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers",
					strings.Join([]string{
						"Authorization",
						"Content-Type",
						"X-Correlation-ID",
						"X-Device-Fingerprint",
						"X-Coordinator-ID",
					}, ", "),
				)
				w.Header().Set("Access-Control-Max-Age", "86400")
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
