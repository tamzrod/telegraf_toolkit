// Package httpapi wires the Toolkit HTTP surface.
//
// The health endpoint remains a Phase 0 scaffold. When a Registry service is
// supplied, an additional read-only status endpoint reports whether the
// persistent metadata store is reachable and at which schema version. No
// endpoint here returns registry data, a credential value or a live probe
// result, and none exposes the metadata store directly.
package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/registryservice"
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

// RegistryStatus is the read-only metadata-store status payload.
type RegistryStatus struct {
	Status        string `json:"status"`
	Store         string `json:"store"`
	Connected     bool   `json:"connected"`
	SchemaVersion int    `json:"schema_version"`
	SecretStore   bool   `json:"secret_store"`
	ActiveKeyID   string `json:"active_key_id,omitempty"`
	Description   string `json:"description"`
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
	return HandlerWithRegistry(nil)
}

// HandlerWithRegistry returns the HTTP handler with an optional Registry
// service. Passing nil keeps the scaffold-only surface.
func HandlerWithRegistry(registry *registryservice.Service) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		writeJSON(w, http.StatusOK, NewHealth())
	})

	if registry != nil {
		mux.HandleFunc("/api/v1/registry/status", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				w.Header().Set("Allow", http.MethodGet)
				writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
			defer cancel()
			status, err := registry.Status(ctx)
			if err != nil {
				writeJSON(w, http.StatusServiceUnavailable, RegistryStatus{
					Status:      "unavailable",
					Store:       "sqlite",
					Connected:   false,
					Description: "Metadata store is not reachable.",
				})
				return
			}
			writeJSON(w, http.StatusOK, RegistryStatus{
				Status:        "ok",
				Store:         "sqlite",
				Connected:     status.Connected,
				SchemaVersion: status.SchemaVersion,
				SecretStore:   status.SecretStore,
				ActiveKeyID:   status.ActiveKeyID,
				Description:   "Persistent control-plane metadata store (ADR-0002). Holds credential references and encrypted envelopes; keys are external.",
			})
		})
	}

	return mux
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
