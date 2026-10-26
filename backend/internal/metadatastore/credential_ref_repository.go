package metadatastore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/registrycontract"
)

// CredentialRefRepository persists credential references. It satisfies
// registrycontract.CredentialRefStore. No method here reads, returns or stores
// a credential value: the schema has no value column, so a value cannot be
// persisted even by mistake (ADR-0002/ADR-0003).
type CredentialRefRepository struct {
	db execer
}

// Put inserts or replaces a credential reference.
func (r *CredentialRefRepository) Put(ref registrycontract.CredentialRef) error {
	if err := registrycontract.ValidateCredentialRef(ref); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO credential_refs (
			credential_ref_id, alias, role, fingerprint, last_rotated, expires_at, mock
		) VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(credential_ref_id) DO UPDATE SET
			alias = excluded.alias,
			role = excluded.role,
			fingerprint = excluded.fingerprint,
			last_rotated = excluded.last_rotated,
			expires_at = excluded.expires_at,
			mock = excluded.mock`,
		ref.CredentialRefID, ref.Alias, string(ref.Role), ref.Fingerprint,
		nullableTime(ref.LastRotated), nullableTime(ref.ExpiresAt), boolToInt(ref.Mock),
	)
	if err != nil {
		return fmt.Errorf("upsert credential reference: %w", err)
	}
	return nil
}

// Get returns a stored reference by ID.
func (r *CredentialRefRepository) Get(credentialRefID string) (registrycontract.CredentialRef, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	row := r.db.QueryRowContext(ctx, `
		SELECT credential_ref_id, alias, role, fingerprint, last_rotated, expires_at, mock
		FROM credential_refs WHERE credential_ref_id = ?`, credentialRefID)

	ref, err := scanCredentialRef(row)
	if errors.Is(err, sql.ErrNoRows) {
		return registrycontract.CredentialRef{}, false, nil
	}
	if err != nil {
		return registrycontract.CredentialRef{}, false, err
	}
	return ref, true, nil
}

// List returns every reference ordered by ID.
func (r *CredentialRefRepository) List() ([]registrycontract.CredentialRef, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := r.db.QueryContext(ctx, `
		SELECT credential_ref_id, alias, role, fingerprint, last_rotated, expires_at, mock
		FROM credential_refs ORDER BY credential_ref_id`)
	if err != nil {
		return nil, fmt.Errorf("list credential references: %w", err)
	}
	defer rows.Close()

	out := make([]registrycontract.CredentialRef, 0)
	for rows.Next() {
		ref, err := scanCredentialRef(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, ref)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate credential references: %w", err)
	}
	return out, nil
}

// Delete removes a stored reference. It is used when a credential is revoked,
// so that no profile is left pointing at a reference that no longer exists.
func (r *CredentialRefRepository) Delete(credentialRefID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := r.db.ExecContext(ctx, `DELETE FROM credential_refs WHERE credential_ref_id = ?`, credentialRefID)
	if err != nil {
		return fmt.Errorf("delete credential reference: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete credential reference result: %w", err)
	}
	if affected == 0 {
		return registrycontract.NewSanitizedError(registrycontract.CodeReferenceNotFound,
			fmt.Sprintf("No credential reference matches %q.", credentialRefID))
	}
	return nil
}

func scanCredentialRef(s scanner) (registrycontract.CredentialRef, error) {
	var (
		ref         registrycontract.CredentialRef
		role        string
		lastRotated sql.NullString
		expiresAt   sql.NullString
		mock        int
	)
	if err := s.Scan(
		&ref.CredentialRefID, &ref.Alias, &role, &ref.Fingerprint,
		&lastRotated, &expiresAt, &mock,
	); err != nil {
		return registrycontract.CredentialRef{}, err
	}
	ref.Role = registrycontract.Role(role)
	ref.Mock = mock != 0
	var err error
	if ref.LastRotated, err = parseNullableTime(lastRotated); err != nil {
		return registrycontract.CredentialRef{}, err
	}
	if ref.ExpiresAt, err = parseNullableTime(expiresAt); err != nil {
		return registrycontract.CredentialRef{}, err
	}
	return ref, nil
}

// nullableTime stores a zero time as NULL so "never rotated" and "no expiry"
// stay distinct from a real timestamp.
func nullableTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func parseNullableTime(v sql.NullString) (time.Time, error) {
	if !v.Valid || v.String == "" {
		return time.Time{}, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, v.String)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse stored timestamp %q: %w", v.String, err)
	}
	return parsed, nil
}
