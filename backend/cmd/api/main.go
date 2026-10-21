// Command api runs the Telegraf Toolkit backend.
//
// It serves the health endpoint and, when a metadata store path is configured,
// a read-only Registry status endpoint backed by the persistent SQLite
// control-plane metadata store selected by ADR-0002. It performs no database
// historian access, no Docker control and no Telegraf runtime management, and
// it never resolves a credential value.
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/httpapi"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/metadatastore"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/registryservice"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/version"
)

// defaultStorePath is the metadata database location when none is supplied. It
// is a local file, never a network path, matching the single-host ADR-0002
// scope.
const defaultStorePath = "data/registry-metadata.db"

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	storePath := flag.String("store", defaultStorePath, "SQLite metadata store path (ADR-0002)")
	flag.Parse()

	registry := openRegistry(*storePath)

	srv := &http.Server{
		Addr:              *addr,
		Handler:           httpapi.HandlerWithRegistry(registry),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("telegraf-toolkit api %s listening on %s", version.Version, *addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}

// openRegistry opens the metadata store and builds the Registry service. A
// store that cannot be opened fails closed: the process logs the failure and
// serves without a registry rather than serving against an unverified schema.
func openRegistry(storePath string) *registryservice.Service {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	store, err := metadatastore.Open(ctx, metadatastore.Options{Path: storePath})
	if err != nil {
		log.Printf("metadata store unavailable (%s): %v", storePath, err)
		return nil
	}
	log.Printf("metadata store ready at %s (schema version %d)", storePath, metadatastore.SchemaVersion())
	return registryservice.New(store.Profiles(), store.CredentialRefs(), store.Audit(), store)
}
