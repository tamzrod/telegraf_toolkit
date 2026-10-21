package metadatastore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/fixtures"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/registrycontract"
)

// ProfileRepository persists connection profiles. It satisfies
// registrycontract.ProfileStore, so it can replace the in-memory adapter in the
// Registry service without the caller changing.
type ProfileRepository struct {
	db execer
}

// Put inserts a profile. A duplicate connection_id is rejected with the
// contract's duplicate-profile code so identity stays unique across restarts.
func (r *ProfileRepository) Put(profile fixtures.ConnectionProfile) error {
	if err := registrycontract.ValidateProfile(profile); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dialects, err := json.Marshal(profile.Capabilities.QueryDialects)
	if err != nil {
		return fmt.Errorf("encode query dialects: %w", err)
	}
	protocols, err := json.Marshal(profile.Capabilities.WriteProtocols)
	if err != nil {
		return fmt.Errorf("encode write protocols: %w", err)
	}

	_, err = r.db.ExecContext(ctx, `
		INSERT INTO connection_profiles (
			connection_id, display_name, family, major_version, endpoint,
			scope_kind, scope_database, scope_retention_policy,
			scope_organization, scope_bucket,
			query_dialects, write_protocols, schema_discovery,
			credential_ref_id, mock, note
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		profile.ConnectionID, profile.DisplayName, string(profile.Family), profile.MajorVersion, profile.Endpoint,
		string(profile.Scope.Kind), profile.Scope.Database, profile.Scope.RetentionPolicy,
		profile.Scope.Organization, profile.Scope.Bucket,
		string(dialects), string(protocols), boolToInt(profile.Capabilities.SchemaDiscovery),
		profile.CredentialRefID, boolToInt(profile.Mock), profile.Note,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return registrycontract.NewSanitizedError(registrycontract.CodeDuplicateProfile,
				fmt.Sprintf("Profile %q already exists; identities are unique.", profile.ConnectionID))
		}
		return fmt.Errorf("insert profile: %w", err)
	}
	return nil
}

// Get returns a stored profile by connection_id.
func (r *ProfileRepository) Get(connectionID string) (fixtures.ConnectionProfile, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	row := r.db.QueryRowContext(ctx, `
		SELECT connection_id, display_name, family, major_version, endpoint,
		       scope_kind, scope_database, scope_retention_policy,
		       scope_organization, scope_bucket,
		       query_dialects, write_protocols, schema_discovery,
		       credential_ref_id, mock, note
		FROM connection_profiles WHERE connection_id = ?`, connectionID)

	profile, err := scanProfile(row)
	if errors.Is(err, sql.ErrNoRows) {
		return fixtures.ConnectionProfile{}, false, nil
	}
	if err != nil {
		return fixtures.ConnectionProfile{}, false, err
	}
	return profile, true, nil
}

// List returns every profile ordered by connection_id, so the result is
// deterministic across processes.
func (r *ProfileRepository) List() ([]fixtures.ConnectionProfile, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := r.db.QueryContext(ctx, `
		SELECT connection_id, display_name, family, major_version, endpoint,
		       scope_kind, scope_database, scope_retention_policy,
		       scope_organization, scope_bucket,
		       query_dialects, write_protocols, schema_discovery,
		       credential_ref_id, mock, note
		FROM connection_profiles ORDER BY connection_id`)
	if err != nil {
		return nil, fmt.Errorf("list profiles: %w", err)
	}
	defer rows.Close()

	out := make([]fixtures.ConnectionProfile, 0)
	for rows.Next() {
		profile, err := scanProfile(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, profile)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate profiles: %w", err)
	}
	return out, nil
}

// Delete removes a profile by connection_id.
func (r *ProfileRepository) Delete(connectionID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := r.db.ExecContext(ctx, `DELETE FROM connection_profiles WHERE connection_id = ?`, connectionID)
	if err != nil {
		return fmt.Errorf("delete profile: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete profile result: %w", err)
	}
	if affected == 0 {
		return registrycontract.NewSanitizedError(registrycontract.CodeProfileNotFound,
			fmt.Sprintf("Profile %q not found.", connectionID))
	}
	return nil
}

// scanner is satisfied by both *sql.Row and *sql.Rows.
type scanner interface {
	Scan(dest ...any) error
}

func scanProfile(s scanner) (fixtures.ConnectionProfile, error) {
	var (
		profile    fixtures.ConnectionProfile
		family     string
		scopeKind  string
		dialects   string
		protocols  string
		schemaDisc int
		mock       int
	)
	if err := s.Scan(
		&profile.ConnectionID, &profile.DisplayName, &family, &profile.MajorVersion, &profile.Endpoint,
		&scopeKind, &profile.Scope.Database, &profile.Scope.RetentionPolicy,
		&profile.Scope.Organization, &profile.Scope.Bucket,
		&dialects, &protocols, &schemaDisc,
		&profile.CredentialRefID, &mock, &profile.Note,
	); err != nil {
		return fixtures.ConnectionProfile{}, err
	}
	profile.Family = fixtures.Family(family)
	profile.Scope.Kind = fixtures.ScopeKind(scopeKind)
	profile.Mock = mock != 0
	profile.Capabilities.SchemaDiscovery = schemaDisc != 0
	if err := json.Unmarshal([]byte(dialects), &profile.Capabilities.QueryDialects); err != nil {
		return fixtures.ConnectionProfile{}, fmt.Errorf("decode query dialects: %w", err)
	}
	if err := json.Unmarshal([]byte(protocols), &profile.Capabilities.WriteProtocols); err != nil {
		return fixtures.ConnectionProfile{}, fmt.Errorf("decode write protocols: %w", err)
	}
	return profile, nil
}
