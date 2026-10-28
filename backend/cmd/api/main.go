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

	"github.com/tamzrod/telegraf_toolkit/backend/internal/authz"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/httpapi"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/liveprobe"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/metadatastore"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/registryapi"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/registryservice"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/secretstore"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/version"
)

// defaultStorePath is the metadata database location when none is supplied. It
// is a local file, never a network path, matching the single-host ADR-0002
// scope.
const defaultStorePath = "data/registry-metadata.db"

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	storePath := flag.String("store", defaultStorePath, "SQLite metadata store path (ADR-0002)")
	keyPath := flag.String("key-file", "", "path to the externally provisioned master key file (ADR-0003)")
	tokenPath := flag.String("token-file", "", "path to the externally provisioned authentication token file")
	allowTestProbe := flag.Bool("allow-test-probe-policy", false, "allow the loopback/insecure-TLS capability-probe policy (isolated test harness only; never a production default)")
	flag.Parse()

	registry := openRegistry(*storePath, *keyPath)
	api := openRegistryAPI(registry, *tokenPath, *allowTestProbe)

	srv := &http.Server{
		Addr:              *addr,
		Handler:           httpapi.HandlerWithRegistry(registry, api),
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
//
// The secret store is attached only when a master key file is configured and
// loads successfully. Without a key the service still serves metadata but cannot
// store or resolve a credential value. The key is never read from an
// environment variable, a request or a command-line value (only its path is a
// flag), matching ADR-0003.
func openRegistry(storePath, keyPath string) *registryservice.Service {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	store, err := metadatastore.Open(ctx, metadatastore.Options{Path: storePath})
	if err != nil {
		log.Printf("metadata store unavailable (%s): %v", storePath, err)
		return nil
	}
	log.Printf("metadata store ready at %s (schema version %d)", storePath, metadatastore.SchemaVersion())

	service := registryservice.New(store.Profiles(), store.CredentialRefs(), store.Audit(), store).
		WithCredentialBindings(store.CredentialBindings()).
		WithCanonicalSchemas(store.CanonicalSchemas(), store.MappingProfiles(), store.RevisionHistory())
	attachSecretStore(service, store, keyPath)
	return service
}

// attachSecretStore loads the master key and attaches the envelope-encrypted
// secret store. A missing or invalid key is logged and leaves the secret store
// unattached; it never falls back to a generated or default key.
func attachSecretStore(service *registryservice.Service, store *metadatastore.Store, keyPath string) {
	if keyPath == "" {
		log.Printf("no master key file configured; credential storage is unavailable")
		return
	}
	keyring, err := secretstore.LoadKeyring(keyPath)
	if err != nil {
		log.Printf("master key unavailable (%s): %v", keyPath, err)
		return
	}
	secrets, err := secretstore.New(secretstore.Options{
		Keyring:   keyring,
		Envelopes: store.SecretEnvelopes(),
		Audit:     store.Audit(),
	})
	if err != nil {
		log.Printf("secret store unavailable: %v", err)
		return
	}
	service.WithSecretStore(secrets)
	log.Printf("secret store ready (active key %s)", keyring.ActiveKeyID())
}

// openRegistryAPI builds the authorized Registry API. It requires both a
// Registry service and a valid authentication token file; without either it
// returns nil so no data route is registered and there is no anonymously
// reachable registry surface.
//
// allowTestProbePolicy enables the explicit, documented loopback/insecure-TLS
// probe policy for the isolated testing/influxdb harness. It is never a
// production default and must be requested deliberately.
func openRegistryAPI(registry *registryservice.Service, tokenPath string, allowTestProbePolicy bool) *registryapi.API {
	if registry == nil {
		return nil
	}
	if tokenPath == "" {
		log.Printf("no authentication token file configured; registry data routes are disabled")
		return nil
	}
	authenticator, err := authz.LoadTokenFile(tokenPath)
	if err != nil {
		log.Printf("authentication token file unavailable (%s): %v", tokenPath, err)
		return nil
	}

	probePolicy := liveprobe.DefaultPolicy()
	if allowTestProbePolicy {
		probePolicy = liveprobe.TestPolicy()
		log.Printf("capability probing uses the TEST loopback/insecure-TLS policy (not for production)")
	}
	log.Printf("registry API ready (roles: %v)", authz.KnownRoles())
	return registryapi.NewWithOptions(registryapi.Options{
		Service:       registry,
		Authenticator: authenticator,
		Prober:        liveprobe.New(nil),
		ProbePolicy:   probePolicy,
	})
}
