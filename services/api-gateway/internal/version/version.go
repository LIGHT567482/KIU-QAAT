// Package version is the single source of truth for the running platform version.
// Keep this in sync with the repo-root /VERSION file (and frontend VITE_APP_VERSION
// baked at build time). Surfaced by GET /health and GET /api/v1/health.
package version

// Version is the QAAT platform release. Bump on every release.
const Version = "1.0.0"
