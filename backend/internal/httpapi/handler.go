// Package httpapi wires the Toolkit HTTP surface.
//
// This is a Phase 0 scaffold. It exposes only a liveness/health endpoint and a
// build-identity endpoint. It does not connect to databases, Docker, Telegraf
// runtimes or any real system.
package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/version"
)

// Health is the liveness payload returned by the scaffold API.
type Health struct {
	Status      string `json:"status"`
	Service     string `json:"service"`
	Version     string `json:"version"`
	Scaffold    bool   `json:"scaffold"`
	Connected   bool   `json:"connected"`
	Description string `json:"description"`
}

// NewHealth returns the fixed scaffold health payload.
func NewHealth() Health {
	return Health{
		Status:      "ok",
		Service:     "telegraf-toolkit-api",
		Version:     version.Version,
		Scaffold:    true,
		Connected:   false,
		Description: "Phase 0 scaffold. Not connected to databases, Telegraf runtimes or Docker.",
	}
}

// Handler returns the HTTP handler for the scaffold API.
func Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		writeJSON(w, http.StatusOK, NewHealth())
	})

	return mux
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
