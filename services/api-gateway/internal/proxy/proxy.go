package proxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/qaat/api-gateway/internal/middleware"
)

// New returns a reverse-proxy handler that forwards the request to the given
// upstream base URL, preserving the original path. The caller's identity
// (tenant, user, role) — already verified by JWTAuth — is injected as headers
// so downstream services can apply tenant scoping without re-parsing the JWT.
func New(target string) (http.HandlerFunc, error) {
	base, err := url.Parse(target)
	if err != nil {
		return nil, err
	}

	rp := httputil.NewSingleHostReverseProxy(base)

	// Keep the default director (sets scheme/host and joins the path) and add
	// the identity headers + correct Host.
	defaultDirector := rp.Director
	rp.Director = func(req *http.Request) {
		// Capture the host the CLIENT used (the tenant's own address) BEFORE the
		// director rewrites it to the upstream, and forward it so downstream
		// services (e.g. qr-generator) can build tenant-adaptive URLs instead of a
		// single baked host. SaaS: QR/links must follow the tenant's address.
		clientHost := req.Host
		if fh := req.Header.Get("X-Forwarded-Host"); fh != "" {
			clientHost = fh // honour an upstream proxy (Caddy) that already set it
		}
		defaultDirector(req)
		req.Host = base.Host
		if clientHost != "" {
			req.Header.Set("X-Forwarded-Host", clientHost)
		}
		ctx := req.Context()
		// Strip any client-supplied identity headers FIRST so a caller can never
		// spoof tenant/user/role into a downstream service; only the values
		// derived from the gateway-verified JWT are forwarded. Downstream
		// services still verify the JWT independently (defence-in-depth), but the
		// gateway must not be the one that lets a forged header through.
		req.Header.Del("X-Tenant-ID")
		req.Header.Del("X-User-ID")
		req.Header.Del("X-Role")
		if t := middleware.GetTenantID(ctx); t != "" {
			req.Header.Set("X-Tenant-ID", t)
		}
		if u := middleware.GetUserID(ctx); u != "" {
			req.Header.Set("X-User-ID", u)
		}
		if role := middleware.GetRole(ctx); role != "" {
			req.Header.Set("X-Role", role)
		}
	}

	rp.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, _ error) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":"UPSTREAM_UNAVAILABLE","message":"downstream service is unreachable"}`))
	}

	return rp.ServeHTTP, nil
}
