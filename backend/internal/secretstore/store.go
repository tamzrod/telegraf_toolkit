package secretstore

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/metadatastore"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/registrycontract"
)

// Compile-time assertions that the envelope-encrypted store satisfies the
// contract's server-side secret boundaries. The contract package's
// BlockedSecretResolver/BlockedSecretWriter remain the fail-closed default for a
// backend with no provisioned master key; this store is the real implementation
// that replaces them when a key is configured.
var (
	_ registrycontract.SecretResolver = (*Store)(nil)
	_ registrycontract.SecretWriter   = (*Store)(nil)
)

// envelopeStore is the persistence boundary for ciphertext envelopes. It is an
// interface so the crypto layer is testable independently and so a future
// backend (for example a managed secret service) can implement it.
type envelopeStore interface {
	Put(env metadatastore.Envelope) error
	Get(credentialRefID string) (metadatastore.Envelope, error)
	ListByKey(keyID string) ([]metadatastore.Envelope, error)
	Rewrap(credentialRefID, expectKeyID string, replacement metadatastore.Envelope) (bool, error)
	Delete(credentialRefID string) error
}

// auditSink records sanitized lifecycle events.
type auditSink interface {
	Record(event registrycontract.AuditEvent) error
}

// Store is the envelope-encrypted credential store. It satisfies
// registrycontract.SecretWriter (write-only ingest) and exposes a server-side
// Resolve. It is constructed only by the backend; the browser has no route to
// it.
type Store struct {
	keyring   *Keyring
	envelopes envelopeStore
	audit     auditSink
	rand      io.Reader
	now       func() time.Time
}

// Options configures a Store.
type Options struct {
	Keyring   *Keyring
	Envelopes envelopeStore
	Audit     auditSink
}

// New builds a secret store. Every dependency is required: without a keyring,
// envelope store or audit sink the store cannot uphold ADR-0003, so it fails
// closed rather than degrading.
func New(opts Options) (*Store, error) {
	if opts.Keyring == nil {
		return nil, errors.New("secret store requires a keyring")
	}
	if opts.Envelopes == nil {
		return nil, errors.New("secret store requires an envelope store")
	}
	if opts.Audit == nil {
		return nil, errors.New("secret store requires an audit sink")
	}
	return &Store{
		keyring:   opts.Keyring,
		envelopes: opts.Envelopes,
		audit:     opts.Audit,
		rand:      rand.Reader,
		now:       func() time.Time { return time.Now().UTC() },
	}, nil
}

// ActiveKeyID reports the key ID new envelopes are wrapped with.
func (s *Store) ActiveKeyID() string { return s.keyring.ActiveKeyID() }

// Store implements registrycontract.SecretWriter. It accepts a write-only
// value, encrypts it into an envelope, persists the envelope and returns an
// opaque reference. The value is never logged, echoed or returned; the returned
// reference carries only non-secret metadata.
//
// The caller supplies the reference ID and role; if the ID is empty a random one
// is generated. A duplicate reference ID is rejected so an existing credential
// is never silently overwritten.
func (s *Store) Store(ctx context.Context, alias string, role registrycontract.Role, value string) (registrycontract.CredentialRef, error) {
	return s.StoreWithRef(ctx, "", alias, role, value)
}

// StoreWithRef is Store with an explicit reference ID, used by callers that
// already minted an identity (and by tests).
func (s *Store) StoreWithRef(ctx context.Context, refID, alias string, role registrycontract.Role, value string) (registrycontract.CredentialRef, error) {
	if err := validateRole(role); err != nil {
		return registrycontract.CredentialRef{}, err
	}
	if value == "" {
		return registrycontract.CredentialRef{}, registrycontract.NewSanitizedError(
			registrycontract.CodeInvalidProfile, "A credential value is required.")
	}
	if refID == "" {
		id, err := s.newReferenceID()
		if err != nil {
			return registrycontract.CredentialRef{}, err
		}
		refID = id
	}
	if _, err := s.envelopes.Get(refID); err == nil {
		return registrycontract.CredentialRef{}, registrycontract.NewSanitizedError(
			registrycontract.CodeDuplicateProfile, "A credential with this reference already exists.")
	} else if !errors.Is(err, metadatastore.ErrEnvelopeNotFound) {
		return registrycontract.CredentialRef{}, err
	}

	enc, err := s.seal(refID, []byte(value))
	if err != nil {
		return registrycontract.CredentialRef{}, err
	}
	if err := s.envelopes.Put(metadatastore.Envelope{
		CredentialRefID: refID,
		KeyID:           enc.keyID,
		Envelope:        enc.encoded,
		UpdatedAt:       s.now(),
	}); err != nil {
		return registrycontract.CredentialRef{}, err
	}

	ref := registrycontract.CredentialRef{
		CredentialRefID: refID,
		Alias:           alias,
		Role:            role,
		Fingerprint:     fingerprint([]byte(value)),
		LastRotated:     s.now(),
		Mock:            false,
	}
	if err := s.recordAudit(ctx, "credential.store", refID); err != nil {
		return registrycontract.CredentialRef{}, err
	}
	return ref, nil
}

// Resolve decrypts the envelope for a reference and returns the value. It is
// server-side only. The value must never be logged, returned to the browser or
// embedded in an error; callers that do so violate ADR-0003.
func (s *Store) Resolve(ctx context.Context, credentialRefID string) (string, error) {
	if strings.TrimSpace(credentialRefID) == "" {
		return "", registrycontract.NewSanitizedError(
			registrycontract.CodeReferenceNotFound, "A credential reference is required.")
	}
	stored, err := s.envelopes.Get(credentialRefID)
	if errors.Is(err, metadatastore.ErrEnvelopeNotFound) {
		return "", registrycontract.NewSanitizedError(
			registrycontract.CodeReferenceNotFound, "No credential is stored for this reference.")
	}
	if err != nil {
		return "", err
	}
	value, err := s.open(stored.Envelope, credentialRefID)
	if err != nil {
		return "", err
	}
	return string(value), nil
}

// sealedEnvelope is the result of sealing a value.
type sealedEnvelope struct {
	keyID   string
	encoded string
}

// seal encrypts a value into an envelope under the active key.
//
// Envelope construction (no custom primitive is invented):
//  1. A random 32-byte data-encryption key (DEK) and nonces are generated.
//  2. The value is encrypted with AES-256-GCM under the DEK, with the reference
//     ID as additional authenticated data, so ciphertext cannot be moved to a
//     different reference.
//  3. The DEK is wrapped (encrypted) with AES-256-GCM under the master key,
//     with wrapInfo as additional authenticated data.
func (s *Store) seal(referenceID string, value []byte) (sealedEnvelope, error) {
	keyID := s.keyring.ActiveKeyID()
	masterKey, ok := s.keyring.key(keyID)
	if !ok {
		return sealedEnvelope{}, ErrWrongKeyID
	}

	dek := make([]byte, 32)
	if _, err := io.ReadFull(s.rand, dek); err != nil {
		return sealedEnvelope{}, fmt.Errorf("generate data key: %w", err)
	}
	dekNonce := make([]byte, gcmNonceSize)
	if _, err := io.ReadFull(s.rand, dekNonce); err != nil {
		return sealedEnvelope{}, fmt.Errorf("generate key nonce: %w", err)
	}
	valueNonce := make([]byte, gcmNonceSize)
	if _, err := io.ReadFull(s.rand, valueNonce); err != nil {
		return sealedEnvelope{}, fmt.Errorf("generate value nonce: %w", err)
	}

	valueGCM, err := newGCM(dek)
	if err != nil {
		return sealedEnvelope{}, err
	}
	valueCT := valueGCM.Seal(nil, valueNonce, value, aadFor(referenceID))

	masterGCM, err := newGCM(masterKey)
	if err != nil {
		return sealedEnvelope{}, err
	}
	wrappedDEK := masterGCM.Seal(nil, dekNonce, dek, []byte(wrapInfo))

	enc := envelope{
		version:      envelopeVersion,
		algorithm:    algorithmID,
		keyID:        keyID,
		wrappedDEK:   wrappedDEK,
		dekNonce:     dekNonce,
		valueNonce:   valueNonce,
		valueCT:      valueCT,
		referenceTag: referenceID,
	}
	encoded, err := enc.encode()
	if err != nil {
		return sealedEnvelope{}, err
	}
	return sealedEnvelope{keyID: keyID, encoded: encoded}, nil
}

// open decrypts a stored envelope for a reference. It fails with a generic
// authentication error when the envelope was tampered with or is bound to a
// different reference, and with ErrWrongKeyID when the wrapping key is absent.
func (s *Store) open(raw, referenceID string) ([]byte, error) {
	enc, err := parseEnvelope(raw)
	if err != nil {
		return nil, err
	}
	if enc.referenceTag != referenceID {
		return nil, ErrEnvelopeAuthFailed
	}
	masterKey, ok := s.keyring.key(enc.keyID)
	if !ok {
		return nil, ErrWrongKeyID
	}
	masterGCM, err := newGCM(masterKey)
	if err != nil {
		return nil, err
	}
	dek, err := masterGCM.Open(nil, enc.dekNonce, enc.wrappedDEK, []byte(wrapInfo))
	if err != nil {
		return nil, ErrEnvelopeAuthFailed
	}
	valueGCM, err := newGCM(dek)
	if err != nil {
		return nil, err
	}
	value, err := valueGCM.Open(nil, enc.valueNonce, enc.valueCT, aadFor(referenceID))
	if err != nil {
		return nil, ErrEnvelopeAuthFailed
	}
	return value, nil
}

// Rotate rewraps every envelope under the keyring's active key.
//
// Procedure (ADR-0003 stage/rewrap/verify/retire):
//  1. It decrypts and re-encrypts each envelope under the new active key.
//  2. Each rewrap is a conditional update keyed on the envelope's current key
//     ID, so a concurrent writer is never clobbered.
//  3. It verifies every rewrapped envelope decrypts and matches the original
//     value before reporting success.
//  4. It does not retire any key: the old key must remain available to unwrap
//     any envelope that failed, so failure always preserves recoverability.
//
// It returns the number of envelopes rewrapped.
func (s *Store) Rotate(ctx context.Context, fromKeyID string) (int, error) {
	if fromKeyID == "" {
		return 0, errors.New("rotation requires the previous key ID")
	}
	active := s.keyring.ActiveKeyID()
	if fromKeyID == active {
		return 0, errors.New("rotation requires a new active key distinct from the previous one")
	}
	if !s.keyring.has(fromKeyID) {
		return 0, fmt.Errorf("rotation requires the previous key %q to remain available: %w", fromKeyID, ErrWrongKeyID)
	}

	envelopes, err := s.envelopes.ListByKey(fromKeyID)
	if err != nil {
		return 0, err
	}

	rewrapped := 0
	for _, stored := range envelopes {
		// Recover the plaintext, re-seal under the active key, verify, then
		// conditionally replace. Failure leaves the old envelope untouched.
		value, err := s.open(stored.Envelope, stored.CredentialRefID)
		if err != nil {
			return rewrapped, fmt.Errorf("rewrap %q: cannot open existing envelope: %w", stored.CredentialRefID, err)
		}
		sealed, err := s.seal(stored.CredentialRefID, value)
		if err != nil {
			return rewrapped, fmt.Errorf("rewrap %q: %w", stored.CredentialRefID, err)
		}
		// Verify the new envelope before persisting it.
		check, err := s.open(sealed.encoded, stored.CredentialRefID)
		if err != nil || string(check) != string(value) {
			return rewrapped, fmt.Errorf("rewrap %q: verification failed", stored.CredentialRefID)
		}
		ok, err := s.envelopes.Rewrap(stored.CredentialRefID, fromKeyID, metadatastore.Envelope{
			CredentialRefID: stored.CredentialRefID,
			KeyID:           sealed.keyID,
			Envelope:        sealed.encoded,
			UpdatedAt:       s.now(),
		})
		if err != nil {
			return rewrapped, fmt.Errorf("rewrap %q: %w", stored.CredentialRefID, err)
		}
		if ok {
			rewrapped++
		}
	}

	if err := s.recordAudit(ctx, "credential.rotate", fromKeyID); err != nil {
		return rewrapped, err
	}
	return rewrapped, nil
}

// Delete removes an envelope (revocation). Rotation never calls it.
func (s *Store) Delete(ctx context.Context, credentialRefID string) error {
	if err := s.envelopes.Delete(credentialRefID); err != nil {
		return err
	}
	return s.recordAudit(ctx, "credential.revoke", credentialRefID)
}

func (s *Store) recordAudit(ctx context.Context, action, targetID string) error {
	return s.audit.Record(registrycontract.AuditEvent{
		Actor:      "system",
		Action:     action,
		TargetKind: registrycontract.TargetCredentialRef,
		TargetID:   targetID,
		Outcome:    registrycontract.OutcomeSucceeded,
		At:         s.now(),
		Mock:       false,
	})
}

func (s *Store) newReferenceID() (string, error) {
	buf := make([]byte, 16)
	if _, err := io.ReadFull(s.rand, buf); err != nil {
		return "", fmt.Errorf("generate reference ID: %w", err)
	}
	return "cred_" + b64(buf), nil
}

const gcmNonceSize = 12

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, ErrInvalidKey
	}
	return cipher.NewGCM(block)
}

// aadFor binds a value ciphertext to its reference ID.
func aadFor(referenceID string) []byte {
	return []byte("telegraf-toolkit/secretstore/v1/value:" + referenceID)
}

// fingerprint returns a non-reversible verification token for a value. It is
// SHA-256 based and reveals nothing about the value; it is metadata, not a
// secret.
func fingerprint(value []byte) string {
	sum := sha256.Sum256(value)
	return "sha256:" + b64(sum[:])
}

func validateRole(role registrycontract.Role) error {
	switch role {
	case registrycontract.RoleRead, registrycontract.RoleWrite, registrycontract.RoleAdmin:
		return nil
	default:
		return registrycontract.NewSanitizedError(
			registrycontract.CodeRoleMismatch, "Unknown credential role.")
	}
}
