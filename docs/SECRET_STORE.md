# Credential Secret Store

Status: **Implemented (backend-only, envelope-encrypted).**
Decision: [ADR-0003](adr/0003-secret-store-integration.md) (Accepted — Option A, application-level envelope encryption).
Task: P2L-002.

This document describes the backend-only credential secret store that ADR-0003
selects. It covers key custody, the envelope format, the resolver boundary,
rotation, key separation and the honest coverage/limitations.

## Scope

Implemented and verified:

- A pluggable `SecretStore` behind the ADR-0003 boundary, satisfied by the
  envelope-encrypted store (`backend/internal/secretstore`).
- Master key provisioned **outside** the application and read from a restrictive
  file; never from a browser, request, environment variable or committed config.
- Authenticated envelope encryption using AES-256-GCM from the Go standard
  library (no custom cryptography) with per-record random data keys and nonces.
- Versioned envelope format and explicit key IDs.
- Server-only resolver; a write-only ingest path; sanitized audit that records
  references, never values.
- Rotation with stage/rewrap/verify and fail-safe behaviour (the old key is
  never retired by the store).
- Tests proving no plaintext leaks, tamper detection, reference binding,
  fail-closed behaviour and recoverability on rotation failure.

Not in scope:

- External secret manager / KMS integration (deferred per ADR-0003).
- Runtime secret delivery to a Telegraf runtime (a later task).
- Server-authorized HTTP routes that store/resolve credentials (P2L-003). No
  HTTP route resolves a credential value; the API exposes only a status flag.

## Key custody

The master key lives in a JSON file with **owner-only (0600)** permissions:

```json
{
  "active_key_id": "k2",
  "keys": { "k1": "<base64url 32 bytes>", "k2": "<base64url 32 bytes>" }
}
```

- `active_key_id` names the key new envelopes are wrapped with.
- `keys` holds the key material. During a rotation window it may hold the old
  key as well as the new one.

`secretstore.LoadKeyring` enforces:

- the path must exist, be a regular file, and not be readable by group or others
  (`perm & 0o077 == 0`);
- every key must decode to exactly 32 bytes (AES-256);
- the active key ID must be present in `keys`.

Any failure returns an error and **no keyring**, so the store cannot resolve or
write a secret without a valid key. This is the fail-closed requirement.

The running backend only ever **reads** a key file. `secretstore.WriteKeyFile`
exists for operators and tests to provision a key **outside** the application;
it creates the file with `O_EXCL` so a live key is never silently overwritten.

No default, generated or committed development key exists. With no key file
configured the service runs with credential storage unavailable, and
`BlockedSecretResolver`/`BlockedSecretWriter` remain the fail-closed default.

## Envelope format

An envelope is a single opaque string with eight dot-separated fields:

```
<version>.<algorithm>.<key_id>.<wrapped_dek>.<dek_nonce>.<value_nonce>.<value_ct>.<reference_tag>
```

- `version` — envelope format version (`1`).
- `algorithm` — the key-wrapping cipher (`AES256-GCM`).
- `key_id` — the master key that wrapped the data key.
- `wrapped_dek`, `dek_nonce` — the per-record data key and its nonce.
- `value_nonce`, `value_ct` — the encrypted credential value and its nonce.
- `reference_tag` — the credential reference ID the envelope belongs to.

Byte fields are base64url without padding. `wrapped_dek`, `value_ct` and the
nonces are ciphertext or random material; the envelope contains no plaintext.

### Encryption

1. A random 32-byte **data-encryption key (DEK)** and nonces are generated per
   record with `crypto/rand`.
2. The value is sealed with **AES-256-GCM** under the DEK, with the reference ID
   as additional authenticated data (AAD).
3. The DEK is wrapped (sealed) with **AES-256-GCM** under the master key, with a
   fixed `wrapInfo` string as AAD.

This is a standard envelope construction. No custom primitive is invented.

### Integrity and binding

- GCM authenticates every field it protects, so any tampering fails
  authentication (`ErrEnvelopeAuthFailed`).
- Because the reference ID is AAD, a ciphertext moved to a different reference
  fails to decrypt; `open` also compares the `reference_tag` first.
- If the wrapping key is absent, decryption fails with `ErrWrongKeyID` rather
  than returning wrong data.

## Resolver boundary

- `Store.Store`/`StoreWithRef` implement `registrycontract.SecretWriter`: a value
  goes in, an opaque `CredentialRef` (ID, alias, role, fingerprint, timestamps)
  comes out. The value is never returned, logged or echoed.
- `Store.Resolve` is the server-only resolver. It implements
  `registrycontract.SecretResolver`. Callers must use the value only for an
  authorized server-side operation and must never return it to the browser or
  write it to a log.
- The `fingerprint` is a SHA-256 digest used as a non-reversible verification
  token. It is metadata, not a secret.
- Errors are sanitized: an unknown reference yields the contract's
  `credential-reference-not-found` code with a secret-free message.

The API exposes only `secret_store: true/false` and the active key ID through the
read-only status endpoint. No route stores, resolves or returns a value.

## Rotation

`Store.Rotate(ctx, fromKeyID)` implements the ADR-0003 stage/rewrap/verify/retire
procedure for the rewrap step:

1. **Stage** (operator): provision a key file that contains the new key as
   `active_key_id` **and** the old key still present.
2. **Rewrap** (store): for each envelope on the old key, decrypt with the old
   key, re-seal under the active key, **verify** the new envelope decrypts to the
   original value, then replace it with a conditional update keyed on the old
   key ID.
3. **Retire** (operator): once no envelope references the old key, remove it from
   the key file.

Properties:

- The store **never** retires a key. It requires the old key to remain available
  and returns `ErrWrongKeyID` if it is absent, so a rotation can never make data
  unrecoverable.
- The conditional update (`WHERE key_id = ?`) means a concurrent writer that
  already rewrapped a row is not clobbered; a second rotation finds nothing to
  do.
- A failure at any point leaves the affected envelope on the old key; the value
  remains resolvable. Verified by
  `TestRotationFailurePreservesRecoverability`.

Database **token** rotation (add → validate → reassign → revoke) is the
credential-lifecycle concern documented in [`DATABASE_REGISTRY.md`](DATABASE_REGISTRY.md);
the store supports it by storing a new reference and deleting the old one only
after the new one is validated.

## Key separation

- Ciphertext envelopes live in the metadata database (`secret_envelopes`).
- Keys live in a separate file outside the database and outside Git.
- A metadata backup (`VACUUM INTO`) therefore contains ciphertext only and can
  never decrypt anything without the separately held key. See
  [`METADATA_STORE.md`](METADATA_STORE.md).

## Coverage and unsupported editions

- **Coverage:** credential value storage, server-side resolution, rotation and
  revocation for the initial single-host deployment.
- **Unsupported:** external secret manager/KMS backends; multi-host key
  distribution; hardware-backed keys. These require separate authorization.
- The store is edition-independent: it is not specific to any InfluxDB version
  and performs no historian read or write.

## Verification evidence (recorded 2026-10-21)

**Environment:** branch `phase-2/live-registry-implementation`; Go 1.27.3.
No new dependency: envelope encryption uses only the Go standard library.

**Deliverables:**
- `backend/internal/secretstore/` — keyring (`keyring.go`), envelope format
  (`envelope.go`) and the envelope-encrypted store (`store.go`).
- `backend/internal/metadatastore/secret_envelope_repository.go` + migration v3
  (`secret_envelopes` table).
- `backend/internal/registryservice` — service wiring for store/resolve/rotate.
- `backend/cmd/api/main.go` — `-key-file` flag; attaches the secret store only
  when a valid key loads.
- Tests: `secretstore/store_test.go`, `secretstore/keyring_test.go`,
  `secretstore/envelope_test.go`, `registryservice/secret_service_test.go`.

**Commands and actual outcomes:**
- `gofmt -l .` → clean; `go vet ./...` → pass; `go build ./...` → pass.
- `go test ./... -count=1` → all packages `ok`.
- `go test -race ./internal/secretstore/ ./internal/registryservice/ -count=1`
  → `ok`.
- `make check` → exit 0 (backend + frontend + policy).
- Process-level: `api` with `-key-file` served
  `{"schema_version":3,"secret_store":true,"active_key_id":"k1"}`; with no key
  file it reported `secret_store:false` and logged that credential storage is
  unavailable.

**Acceptance mapping (P2L-002):**
- Secure externally provisioned master key file → `LoadKeyring` rejects missing,
  world-readable, wrong-size and missing-active-key files.
- Authenticated envelope encryption via an established library → AES-256-GCM;
  tamper and wrong-key tests fail closed.
- Versioned key IDs → envelopes carry a `key_id`; the key file has
  `active_key_id`.
- Server-only resolver → resolver is internal; the API exposes only a status
  flag.
- Rotation/rewrap rollback and recovery tests → `TestRotationRewrapsAndVerifies`,
  `TestRotationFailurePreservesRecoverability`,
  `TestRotationDoesNotClobberConcurrentRewrap`.
- Inaccessible browser/token logs → audit records references only; no value in
  any stored or returned field.
- Keys outside database and git → key file is separate and `.gitignore` excludes
  `*.key`; a test asserts the database file contains no plaintext.

## Limitations

- Key files are protected by filesystem permissions only; OS keyrings/HSMs are
  not integrated.
- `VACUUM INTO` backups copy ciphertext in full; they are not incremental.
- The store cannot detect a lost key file until a resolve fails; operators must
  hold a separate recovery copy (ADR-0003).
- Only Linux amd64 is verified.
- No external secret manager, KMS or runtime secret-delivery integration exists.
