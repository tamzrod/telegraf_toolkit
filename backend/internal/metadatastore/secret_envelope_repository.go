package metadatastore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Envelope is a stored credential ciphertext envelope. It holds no key and no
// plaintext: `Envelope` is authenticated ciphertext produced by the secret
// store's cryptographic layer.
type Envelope struct {
	CredentialRefID string
	KeyID           string
	Envelope        string
	UpdatedAt       time.Time
}

// ErrEnvelopeNotFound is returned when no envelope is stored for a reference.
var ErrEnvelopeNotFound = errors.New("secret envelope not found")

// SecretEnvelopeRepository persists ciphertext envelopes. It never sees a
// plaintext value or a key; it stores and returns opaque strings only.
type SecretEnvelopeRepository struct {
	db execer
}

// Put upserts an envelope for a reference.
func (r *SecretEnvelopeRepository) Put(env Envelope) error {
	if env.CredentialRefID == "" || env.KeyID == "" || env.Envelope == "" {
		return errors.New("envelope requires a reference ID, key ID and ciphertext")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO secret_envelopes (credential_ref_id, key_id, envelope, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(credential_ref_id) DO UPDATE SET
			key_id = excluded.key_id,
			envelope = excluded.envelope,
			updated_at = excluded.updated_at`,
		env.CredentialRefID, env.KeyID, env.Envelope, env.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("upsert secret envelope: %w", err)
	}
	return nil
}

// Get returns the envelope for a reference.
func (r *SecretEnvelopeRepository) Get(credentialRefID string) (Envelope, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var (
		env       Envelope
		updatedAt string
	)
	row := r.db.QueryRowContext(ctx, `
		SELECT credential_ref_id, key_id, envelope, updated_at
		FROM secret_envelopes WHERE credential_ref_id = ?`, credentialRefID)
	if err := row.Scan(&env.CredentialRefID, &env.KeyID, &env.Envelope, &updatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Envelope{}, ErrEnvelopeNotFound
		}
		return Envelope{}, fmt.Errorf("read secret envelope: %w", err)
	}
	parsed, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return Envelope{}, fmt.Errorf("parse envelope timestamp: %w", err)
	}
	env.UpdatedAt = parsed
	return env, nil
}

// ListByKey returns every envelope wrapped by a given key ID. Rotation uses it
// to find envelopes that still need rewrapping.
func (r *SecretEnvelopeRepository) ListByKey(keyID string) ([]Envelope, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := r.db.QueryContext(ctx, `
		SELECT credential_ref_id, key_id, envelope, updated_at
		FROM secret_envelopes WHERE key_id = ? ORDER BY credential_ref_id`, keyID)
	if err != nil {
		return nil, fmt.Errorf("list envelopes by key: %w", err)
	}
	defer rows.Close()

	out := make([]Envelope, 0)
	for rows.Next() {
		var (
			env       Envelope
			updatedAt string
		)
		if err := rows.Scan(&env.CredentialRefID, &env.KeyID, &env.Envelope, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan envelope: %w", err)
		}
		parsed, err := time.Parse(time.RFC3339Nano, updatedAt)
		if err != nil {
			return nil, fmt.Errorf("parse envelope timestamp: %w", err)
		}
		env.UpdatedAt = parsed
		out = append(out, env)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate envelopes: %w", err)
	}
	return out, nil
}

// Rewrap atomically replaces the ciphertext of an envelope only if it still
// carries the expected current key ID. The conditional update makes rewrapping
// safe under concurrency: a concurrent writer that already rewrapped the row
// causes this call to be a no-op rather than clobbering newer ciphertext.
func (r *SecretEnvelopeRepository) Rewrap(credentialRefID, expectKeyID string, replacement Envelope) (bool, error) {
	if replacement.CredentialRefID == "" || replacement.KeyID == "" || replacement.Envelope == "" {
		return false, errors.New("replacement envelope requires a reference ID, key ID and ciphertext")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := r.db.ExecContext(ctx, `
		UPDATE secret_envelopes
		SET key_id = ?, envelope = ?, updated_at = ?
		WHERE credential_ref_id = ? AND key_id = ?`,
		replacement.KeyID, replacement.Envelope,
		replacement.UpdatedAt.UTC().Format(time.RFC3339Nano),
		credentialRefID, expectKeyID,
	)
	if err != nil {
		return false, fmt.Errorf("rewrap envelope: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("rewrap result: %w", err)
	}
	return affected == 1, nil
}

// ReplaceIf atomically replaces the ciphertext of an envelope only if the
// stored envelope is still exactly the expected one.
//
// The conditional update makes a credential replacement safe under concurrency
// (P2C-007): two attempts that both observed the same current envelope cannot
// both swap - the first wins, the second is a no-op that reports applied=false
// instead of overwriting the first attempt's approved value. Unlike Rewrap,
// which conditions on the key ID alone, this conditions on the full envelope
// ciphertext, so it detects ANY intervening change of the stored value, not
// only a key rotation.
func (r *SecretEnvelopeRepository) ReplaceIf(credentialRefID, expectEnvelope string, replacement Envelope) (bool, error) {
	if replacement.CredentialRefID == "" || replacement.KeyID == "" || replacement.Envelope == "" {
		return false, errors.New("replacement envelope requires a reference ID, key ID and ciphertext")
	}
	if strings.TrimSpace(credentialRefID) == "" || strings.TrimSpace(expectEnvelope) == "" {
		return false, errors.New("conditional replace requires a reference ID and the expected current envelope")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := r.db.ExecContext(ctx, `
		UPDATE secret_envelopes
		SET key_id = ?, envelope = ?, updated_at = ?
		WHERE credential_ref_id = ? AND envelope = ?`,
		replacement.KeyID, replacement.Envelope,
		replacement.UpdatedAt.UTC().Format(time.RFC3339Nano),
		credentialRefID, expectEnvelope,
	)
	if err != nil {
		return false, fmt.Errorf("conditional replace envelope: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("conditional replace result: %w", err)
	}
	return affected == 1, nil
}

// Delete removes an envelope. It is used only by explicit revocation, never by
// rotation.
func (r *SecretEnvelopeRepository) Delete(credentialRefID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := r.db.ExecContext(ctx, `DELETE FROM secret_envelopes WHERE credential_ref_id = ?`, credentialRefID); err != nil {
		return fmt.Errorf("delete secret envelope: %w", err)
	}
	return nil
}
