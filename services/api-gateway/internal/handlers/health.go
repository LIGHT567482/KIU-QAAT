package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/qaat/api-gateway/internal/version"
)

type healthResponse struct {
	Status    string `json:"status"`
	Service   string `json:"service"`
	Version   string `json:"version"`
	Timestamp string `json:"timestamp"`
}

func Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(healthResponse{ //nolint:errcheck
		Status:    "ok",
		Service:   "api-gateway",
		Version:   version.Version,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}
