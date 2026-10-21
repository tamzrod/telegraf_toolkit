package metadatastore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/registrycontract"
)

// CredentialBindingRepository persists approved credential-to-profile bindings.
// It satisfies registrycontract.CredentialBindingStore. It stores no secret
// value: only the reference and the profile identity it is approved for.
type CredentialBindingRepository struct {
	db *sql.DB
}

// Put inserts or replaces an approved binding.
func (r *CredentialBindingRepository) Put(binding registrycontract.CredentialBinding) error {
	if binding.CredentialRefID == "" || binding.ConnectionID == "" {
		return registrycontract.NewSanitizedError(registrycontract.CodeReferenceNotFound,
			"A binding requires a credential reference and a connection ID.")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO credential_bindings (
			credential_ref_id, connection_id, endpoint, scope, approved_by, approved_at
		) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(credential_ref_id, connection_id) DO UPDATE SET
			endpoint = excluded.endpoint,
			scope = excluded.scope,
			approved_by = excluded.approved_by,
			approved_at = excluded.approved_at`,
		binding.CredentialRefID, binding.ConnectionID, binding.Endpoint, binding.Scope,
		binding.ApprovedBy, time.Now().UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("upsert credential binding: %w", err)
	}
	return nil
}

// Get returns a binding for a reference and connection.
func (r *CredentialBindingRepository) Get(credentialRefID, connectionID string) (registrycontract.CredentialBinding, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	row := r.db.QueryRowContext(ctx, `
		SELECT credential_ref_id, connection_id, endpoint, scope, approved_by
		FROM credential_bindings WHERE credential_ref_id = ? AND connection_id = ?`,
		credentialRefID, connectionID)

	binding, err := scanBinding(row)
	if errors.Is(err, sql.ErrNoRows) {
		return registrycontract.CredentialBinding{}, false, nil
	}
	if err != nil {
		return registrycontract.CredentialBinding{}, false, err
	}
	return binding, true, nil
}

// List returns every binding ordered by reference and connection.
func (r *CredentialBindingRepository) List() ([]registrycontract.CredentialBinding, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := r.db.QueryContext(ctx, `
		SELECT credential_ref_id, connection_id, endpoint, scope, approved_by
		FROM credential_bindings ORDER BY credential_ref_id, connection_id`)
	if err != nil {
		return nil, fmt.Errorf("list credential bindings: %w", err)
	}
	defer rows.Close()

	out := make([]registrycontract.CredentialBinding, 0)
	for rows.Next() {
		binding, err := scanBinding(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, binding)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate credential bindings: %w", err)
	}
	return out, nil
}

// Delete removes a binding.
func (r *CredentialBindingRepository) Delete(credentialRefID, connectionID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := r.db.ExecContext(ctx,
		`DELETE FROM credential_bindings WHERE credential_ref_id = ? AND connection_id = ?`,
		credentialRefID, connectionID); err != nil {
		return fmt.Errorf("delete credential binding: %w", err)
	}
	return nil
}

func scanBinding(s scanner) (registrycontract.CredentialBinding, error) {
	var binding registrycontract.CredentialBinding
	if err := s.Scan(
		&binding.CredentialRefID, &binding.ConnectionID, &binding.Endpoint,
		&binding.Scope, &binding.ApprovedBy,
	); err != nil {
		return registrycontract.CredentialBinding{}, err
	}
	return binding, nil
}
