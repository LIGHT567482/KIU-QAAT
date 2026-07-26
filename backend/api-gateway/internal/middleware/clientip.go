package middleware

import (
	"net/http"
	"strings"
)

// ClientIP returns the caller's source IP, preferring the first hop in
// X-Forwarded-For (set by the Caddy reverse proxy) and falling back to
// RemoteAddr with the port stripped. Used for LAN-proximity checks and audit.
func ClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if first := strings.TrimSpace(strings.Split(xff, ",")[0]); first != "" {
			return first
		}
	}
	ip := r.RemoteAddr
	if i := strings.LastIndex(ip, ":"); i != -1 {
		ip = ip[:i]
	}
	return strings.Trim(ip, "[]") // strip IPv6 brackets
}
