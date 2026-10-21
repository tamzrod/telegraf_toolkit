# ADR-0003: Secret-Store Integration and Encryption-Key Management

- **Status:** Accepted
- **Date:** 2026-10-21
- **Scope:** How the Toolkit stores, references, resolves and rotates database credentials and runtime secrets.
- **Deciders:** Human architecture owner (acceptance required).

## Context

[`IDENTITY.md`](../../IDENTITY.md) and [`docs/DATABASE_REGISTRY.md`](../DATABASE_REGISTRY.md) require that connection profiles store **secret references, never plaintext credentials**, that the browser never resolves raw secrets, and that secrets are resolved only server-side or in an authorized host agent/worker. [`docs/ARCHITECTURE.md`](../ARCHITECTURE.md) requires encryption at rest where practical, redaction in logs/exports, and separation of encryption keys from encrypted values.

The metadata store decision ([ADR-0002](0002-metadata-store-selection.md)) deliberately excludes secret material.

## Constraints

- No plaintext secrets in Git, logs, exports, TOML revisions, browser storage, URLs or API responses.
- Secrets are write-only from the UI; full value is never re-displayed after creation.
- Credential references carry read/write/admin role scopes; source reads prefer read-only.
- Rotation must add-and-validate a new credential before revoking the old, without interrupting unrelated runtimes.
- Secret delivery to a Telegraf runtime must use a protected mechanism (restricted files or managed runtime secrets), never command-line flags or world-readable Compose/TOML.
- Encryption keys must be provided independently of the metadata database.
- Must be operable on a single host without requiring an external SaaS dependency by default.

## Considered options

| Option | Summary | Trade-offs |
| --- | --- | --- |
| A. Encrypted secrets in the metadata store | Application-level envelope encryption; keys from env/host secret | Fewest moving parts; key custody and rotation are the Toolkit's responsibility |
| B. External secret manager (e.g. Vault-style) | Dedicated secret backend with its own auth | Strong lifecycle/audit; adds a service to operate and secure; heavier for single host |
| C. Host filesystem secret files | Restricted files delivered to backend/runtime | Simple; weaker audit and rotation ergonomics; file-permission dependent |
| D. Cloud KMS / managed secret service | Provider-managed keys/secrets | Strong custody; introduces external dependency and network/auth surface |

## Trade-offs

- Option A gives the best single-host fit and keeps the Toolkit self-contained, at the cost of implementing careful key custody and rotation.
- Option B is the strongest long-term option for lifecycle, audit and multi-operator environments, but expands the operational and security surface now.
- Options C/D are viable later integrations; D conflicts with a fully self-contained default.

## Security implications

- The secret store must be reachable only by the backend/authorized agent, never the browser.
- Audit records credential **references** and actions, never secret values.
- Encryption keys must not be stored in the same unprotected backup as encrypted values.
- TLS verification is default; insecure verification requires an explicit auditable exception.
- Endpoint validation must be SSRF-aware.

## Validation criteria

- An end-to-end test stores a credential, uses it server-side for a scoped connection test, and confirms the value never appears in API responses, logs or exports.
- Rotation test proves the new credential is validated before the old is revoked, with unrelated runtimes uninterrupted.
- Key-separation test proves ciphertext and key are not co-located in the same backup artifact.

## Recommendation

**Accepted 2026-10-21 by human architecture owner.** Option A is the initial single-host default: application-level envelope encryption behind a pluggable `SecretStore` abstraction. External Vault/KMS implementations are deferred and require separate authorization.

## Accepted initial key-custody and rotation policy

- **Key custody:** the encryption master key is generated outside the application/database and provided to the backend via a restrictive, administrator-provisioned, **read-only host/Compose secret file** outside Git and outside metadata/ciphertext backups; never accept a key from browser/API input, a command line argument, or plaintext application configuration. No default committed or predictable development key. A missing/unreadable key prevents secret resolution (fail closed).
- **Encryption boundary:** use a reviewed authenticated-envelope mechanism with random per-record data-encryption keys/nonces, explicit key identifiers, tamper detection and versioned ciphertext envelope; expose only opaque credential references through Registry APIs. Do not invent a custom cryptographic primitive.
- **Key recovery:** preserve secure separately held recovery copies of the master-key material, with restricted administrator access and a restore drill. A metadata backup alone must never contain the decryption key.
- **Master-key rotation:** stage a new separately provisioned key and key ID, retain the previous key only for an explicitly bounded migration period, rewrap/re-encrypt all affected secret envelopes transactionally with verification, test restart/recovery, then retire the old key after completeness is confirmed. Failure must preserve recoverability; never discard a required old key prematurely.
- **Database token rotation:** provision and validate the replacement scoped token before swapping the active credential reference; audit the reference transition, then revoke the old token, without interrupting unrelated runtimes.
- **External manager integration:** deferred beyond the initial single-host phase. Any production key provisioning/backup environment must be separately reviewed before deployment.
- This ADR acceptance is a design decision; all implementation and testing still require authorized CWAL tasks. No production credential handling is implied.

## References

- [`docs/contracts/README.md`](../contracts/README.md) — credential reference contract and ADR-B candidate
- [`docs/DATABASE_REGISTRY.md`](../DATABASE_REGISTRY.md) — credential lifecycle and safeguards
- [`docs/ARCHITECTURE.md`](../ARCHITECTURE.md) — security and operations
- [ADR-0002](0002-metadata-store-selection.md) — metadata store (excludes secrets)
