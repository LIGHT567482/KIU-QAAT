package handlers

import (
	"encoding/json"
	"net/http"
)

// NotImplemented returns a handler stub for routes that exist in the router
// but whose service implementations are built in later phases.
func NotImplemented(route string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotImplemented)
		json.NewEncoder(w).Encode(map[string]string{ //nolint:errcheck
			"error":   "NOT_IMPLEMENTED",
			"message": route + " is not yet implemented",
		})
	}
}
