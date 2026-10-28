package registrycontract

import "context"

// Repositories bundles the registry repositories that take part in one atomic
// operation.
//
// A transactional store hands the unit of work a bundle whose repositories are
// bound to the same transaction, so a write made through one of them is
// committed or rolled back together with the others. A caller that decorates
// a repository in this bundle (for example to inject a fault) still operates
// inside the transaction, which is what makes fault injection meaningful.
type Repositories struct {
	// Profiles stores connection profiles.
	Profiles ProfileStore
	// References stores credential references. It never holds a value.
	References CredentialRefStore
	// Bindings stores approved credential-to-profile bindings.
	Bindings CredentialBindingStore
	// Audit records sanitized audit events.
	Audit AuditSink
	// CanonicalSchemas stores versioned canonical schemas.
	CanonicalSchemas CanonicalSchemaStore
	// MappingProfiles stores versioned mapping profiles.
	MappingProfiles MappingProfileStore
	// RevisionHistory records immutable revision history.
	RevisionHistory RevisionHistoryStore
}

// TransactionalStore is the optional persistence capability that applies a set
// of registry writes atomically.
//
// Implementing it means a multi-write registry operation - a profile, its
// approved binding and the audit events - is applied all or nothing: if any
// write fails, every write of that operation is rolled back, so no orphaned
// profile, binding or audit gap can survive a failure (P2C-002).
type TransactionalStore interface {
	// WithinTransaction runs fn with repositories bound to a single
	// transaction. If fn returns an error, every write it performed is rolled
	// back and the error is returned unchanged. If fn succeeds, the writes are
	// committed together.
	WithinTransaction(ctx context.Context, fn func(Repositories) error) error
}
