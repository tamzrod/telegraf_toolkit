package metadatastore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/registrycontract"
)

// CanonicalSchemaRepository persists versioned canonical schemas.
type CanonicalSchemaRepository struct {
	db execer
}

func (r *CanonicalSchemaRepository) beginTx(ctx context.Context) (*sql.Tx, error) {
	type txer interface {
		BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error)
	}
	if t, ok := r.db.(txer); ok {
		return t.BeginTx(ctx, nil)
	}
	return nil, fmt.Errorf("transactions not supported")
}

func (r *CanonicalSchemaRepository) Put(schema registrycontract.CanonicalSchemaRevision) error {
	if strings.TrimSpace(schema.SchemaID) == "" || strings.TrimSpace(schema.LogicalKey) == "" {
		return registrycontract.NewSanitizedError(registrycontract.CodeInvalidProfile, "A canonical schema requires an id and logical key.")
	}
	if strings.TrimSpace(schema.ContentJSON) == "" {
		return registrycontract.NewSanitizedError(registrycontract.CodeInvalidProfile, "A canonical schema requires content.")
	}
	if err := validateSafeJSONNoSecrets(schema.ContentJSON); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tx, err := r.beginTx(ctx)
	if err != nil {
		// BeginTx failed - propagate error to caller instead of silent fallback
		return fmt.Errorf("BeginTx failed: %w", err)
	}
	if err != nil {
		// BeginTx failed - propagate error to caller instead of silent fallback
		return fmt.Errorf("BeginTx failed: %w", err)
	}
	if tx != nil {
		// If here, beginTx succeeded with a valid transaction
		defer tx.Rollback()
		// Try to update if draft
		res, err := tx.ExecContext(ctx, `
			UPDATE canonical_schemas SET
				logical_key = ?, major_version = ?, minor_version = ?, revision = ?,
				state = ?, content_json = ?, content_hash = ?, created_by = ?, created_at = ?,
				approved_by = ?, approved_at = ?, notes = ?
			WHERE schema_id = ? AND state = 'draft'`,
			schema.LogicalKey, schema.MajorVersion, schema.MinorVersion, schema.Revision,
			string(schema.State), schema.ContentJSON, schema.ContentHash, schema.CreatedBy,
			formatTime(schema.CreatedAt), schema.ApprovedBy, formatTime(schema.ApprovedAt), schema.Notes,
			schema.SchemaID,
		)
		if err != nil {
			return fmt.Errorf("update canonical schema: %w", err)
		}
		n, _ := res.RowsAffected()
		if n > 0 {
			if string(schema.State) == string(registrycontract.CanonicalStateApproved) {
				tx.Rollback()
				return registrycontract.NewSanitizedError(registrycontract.CodeSecretMaterial, "use approval transition for approved state")
			}
			if err := tx.Commit(); err != nil {
				return fmt.Errorf("commit canonical schema: %w", err)
			}
			return nil
		}
		// Check if exists and approved
		var existingState string
		err = tx.QueryRowContext(ctx, `SELECT state FROM canonical_schemas WHERE schema_id = ?`, schema.SchemaID).Scan(&existingState)
		if err == nil {
			if existingState == string(registrycontract.CanonicalStateApproved) {
				tx.Rollback()
				return registrycontract.NewSanitizedError(registrycontract.CodeSecretMaterial, "approved canonical schema revisions are immutable")
			}
			tx.Rollback()
			return registrycontract.NewSanitizedError(registrycontract.CodeSecretMaterial, "approved canonical schema revisions are immutable")
		}
		if !errors.Is(err, sql.ErrNoRows) {
			tx.Rollback()
			return fmt.Errorf("read existing canonical schema: %w", err)
		}
		// insert new
		if string(schema.State) == string(registrycontract.CanonicalStateApproved) {
			tx.Rollback()
			return registrycontract.NewSanitizedError(registrycontract.CodeSecretMaterial, "cannot insert approved revision")
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO canonical_schemas (
				schema_id, logical_key, major_version, minor_version, revision,
				state, content_json, content_hash, created_by, created_at,
				approved_by, approved_at, notes
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			schema.SchemaID, schema.LogicalKey, schema.MajorVersion, schema.MinorVersion, schema.Revision,
			string(schema.State), schema.ContentJSON, schema.ContentHash, schema.CreatedBy,
			formatTime(schema.CreatedAt), schema.ApprovedBy, formatTime(schema.ApprovedAt), schema.Notes,
		)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("insert canonical schema: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit canonical schema: %w", err)
		}
		return nil
	}

	// Non-tx fallback
	var existingState string
	err = r.db.QueryRowContext(ctx, `SELECT state FROM canonical_schemas WHERE schema_id = ?`, schema.SchemaID).Scan(&existingState)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("read existing canonical schema: %w", err)
	}
	if err == nil {
		if existingState == string(registrycontract.CanonicalStateApproved) {
			return registrycontract.NewSanitizedError(registrycontract.CodeSecretMaterial, "approved canonical schema revisions are immutable")
		}
		if string(schema.State) == string(registrycontract.CanonicalStateApproved) {
			return registrycontract.NewSanitizedError(registrycontract.CodeSecretMaterial, "use approval transition for approved state")
		}
		res, err := r.db.ExecContext(ctx, `
			UPDATE canonical_schemas SET
				logical_key = ?, major_version = ?, minor_version = ?, revision = ?,
				state = ?, content_json = ?, content_hash = ?, created_by = ?, created_at = ?,
				approved_by = ?, approved_at = ?, notes = ?
			WHERE schema_id = ? AND state = 'draft'`,
			schema.LogicalKey, schema.MajorVersion, schema.MinorVersion, schema.Revision,
			string(schema.State), schema.ContentJSON, schema.ContentHash, schema.CreatedBy,
			formatTime(schema.CreatedAt), schema.ApprovedBy, formatTime(schema.ApprovedAt), schema.Notes,
			schema.SchemaID,
		)
		if err != nil {
			return fmt.Errorf("update canonical schema: %w", err)
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			return registrycontract.NewSanitizedError(registrycontract.CodeSecretMaterial, "approved canonical schema revisions are immutable")
		}
		return nil
	}

	if string(schema.State) == string(registrycontract.CanonicalStateApproved) {
		return registrycontract.NewSanitizedError(registrycontract.CodeSecretMaterial, "cannot insert approved revision")
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO canonical_schemas (
			schema_id, logical_key, major_version, minor_version, revision,
			state, content_json, content_hash, created_by, created_at,
			approved_by, approved_at, notes
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		schema.SchemaID, schema.LogicalKey, schema.MajorVersion, schema.MinorVersion, schema.Revision,
		string(schema.State), schema.ContentJSON, schema.ContentHash, schema.CreatedBy,
		formatTime(schema.CreatedAt), schema.ApprovedBy, formatTime(schema.ApprovedAt), schema.Notes,
	)
	if err != nil {
		return fmt.Errorf("insert canonical schema: %w", err)
	}
	return nil
}

func (r *CanonicalSchemaRepository) Get(schemaID string) (registrycontract.CanonicalSchemaRevision, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	row := r.db.QueryRowContext(ctx, `
		SELECT schema_id, logical_key, major_version, minor_version, revision,
			state, content_json, content_hash, created_by, created_at,
			approved_by, approved_at, notes
		FROM canonical_schemas WHERE schema_id = ?`, schemaID)
	s, err := scanCanonical(row)
	if errors.Is(err, sql.ErrNoRows) {
		return registrycontract.CanonicalSchemaRevision{}, false, nil
	}
	if err != nil {
		return registrycontract.CanonicalSchemaRevision{}, false, err
	}
	return s, true, nil
}

func (r *CanonicalSchemaRepository) GetByLogical(logicalKey string, revision int) (registrycontract.CanonicalSchemaRevision, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	row := r.db.QueryRowContext(ctx, `
		SELECT schema_id, logical_key, major_version, minor_version, revision,
			state, content_json, content_hash, created_by, created_at,
			approved_by, approved_at, notes
		FROM canonical_schemas WHERE logical_key = ? AND revision = ?`, logicalKey, revision)
	s, err := scanCanonical(row)
	if errors.Is(err, sql.ErrNoRows) {
		return registrycontract.CanonicalSchemaRevision{}, false, nil
	}
	if err != nil {
		return registrycontract.CanonicalSchemaRevision{}, false, err
	}
	return s, true, nil
}

func (r *CanonicalSchemaRepository) ListByLogical(logicalKey string) ([]registrycontract.CanonicalSchemaRevision, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := r.db.QueryContext(ctx, `
		SELECT schema_id, logical_key, major_version, minor_version, revision,
			state, content_json, content_hash, created_by, created_at,
			approved_by, approved_at, notes
		FROM canonical_schemas WHERE logical_key = ? ORDER BY revision ASC`, logicalKey)
	if err != nil {
		return nil, fmt.Errorf("list canonical schemas: %w", err)
	}
	defer rows.Close()
	return scanCanonicals(rows)
}

func (r *CanonicalSchemaRepository) ListAll() ([]registrycontract.CanonicalSchemaRevision, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := r.db.QueryContext(ctx, `
		SELECT schema_id, logical_key, major_version, minor_version, revision,
			state, content_json, content_hash, created_by, created_at,
			approved_by, approved_at, notes
		FROM canonical_schemas ORDER BY logical_key ASC, revision ASC`)
	if err != nil {
		return nil, fmt.Errorf("list all canonical schemas: %w", err)
	}
	defer rows.Close()
	return scanCanonicals(rows)
}

func scanCanonicals(rows *sql.Rows) ([]registrycontract.CanonicalSchemaRevision, error) {
	out := make([]registrycontract.CanonicalSchemaRevision, 0)
	for rows.Next() {
		s, err := scanCanonical(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate canonical schemas: %w", err)
	}
	return out, nil
}

func scanCanonical(s scanner) (registrycontract.CanonicalSchemaRevision, error) {
	var (
		sch      registrycontract.CanonicalSchemaRevision
		state    string
		created  string
		approved string
	)
	if err := s.Scan(
		&sch.SchemaID, &sch.LogicalKey, &sch.MajorVersion, &sch.MinorVersion, &sch.Revision,
		&state, &sch.ContentJSON, &sch.ContentHash, &sch.CreatedBy, &created,
		&sch.ApprovedBy, &approved, &sch.Notes,
	); err != nil {
		return registrycontract.CanonicalSchemaRevision{}, err
	}
	sch.State = registrycontract.CanonicalRevisionState(state)
	var err error
	if sch.CreatedAt, err = parseTime(created); err != nil {
		return registrycontract.CanonicalSchemaRevision{}, err
	}
	if approved != "" {
		if sch.ApprovedAt, err = parseTime(approved); err != nil {
			return registrycontract.CanonicalSchemaRevision{}, err
		}
	}
	return sch, nil
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func parseTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339Nano, s)
}

// MappingProfileRepository persists versioned mapping profiles.
type MappingProfileRepository struct {
	db execer
}

func (r *MappingProfileRepository) beginTx(ctx context.Context) (*sql.Tx, error) {
	type txer interface {
		BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error)
	}
	if t, ok := r.db.(txer); ok {
		return t.BeginTx(ctx, nil)
	}
	return nil, fmt.Errorf("transactions not supported")
}

func (r *MappingProfileRepository) Put(profile registrycontract.MappingProfileRevision) error {
	if strings.TrimSpace(profile.ProfileID) == "" || strings.TrimSpace(profile.LogicalKey) == "" {
		return registrycontract.NewSanitizedError(registrycontract.CodeInvalidProfile, "A mapping profile requires an id and logical key.")
	}
	if strings.TrimSpace(profile.ContentJSON) == "" {
		return registrycontract.NewSanitizedError(registrycontract.CodeInvalidProfile, "A mapping profile requires content.")
	}
	if err := validateSafeJSONNoSecrets(profile.ContentJSON); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tx, err := r.beginTx(ctx)
	if err != nil {
		// BeginTx failed - propagate error to caller instead of silent fallback
		return fmt.Errorf("BeginTx failed: %w", err)
	}
	if tx != nil {
		// If here, beginTx succeeded with a valid transaction
		defer tx.Rollback()
		res, err := tx.ExecContext(ctx, `
			UPDATE mapping_profiles SET
				logical_key = ?, canonical_logical_key = ?, canonical_revision = ?,
				source_connection_id = ?, destination_connection_id = ?, source_scope = ?, destination_scope = ?,
				major_version = ?, minor_version = ?, revision = ?, state = ?, content_json = ?, content_hash = ?,
				created_by = ?, created_at = ?, approved_by = ?, approved_at = ?, notes = ?
			WHERE profile_id = ? AND state = 'draft'`,
			profile.LogicalKey, profile.CanonicalLogicalKey, profile.CanonicalRevision,
			profile.SourceConnectionID, profile.DestinationConnectionID, profile.SourceScope, profile.DestinationScope,
			profile.MajorVersion, profile.MinorVersion, profile.Revision, string(profile.State),
			profile.ContentJSON, profile.ContentHash, profile.CreatedBy, formatTime(profile.CreatedAt),
			profile.ApprovedBy, formatTime(profile.ApprovedAt), profile.Notes,
			profile.ProfileID,
		)
		if err != nil {
			return fmt.Errorf("update mapping profile: %w", err)
		}
		n, _ := res.RowsAffected()
		if n > 0 {
			if string(profile.State) == string(registrycontract.CanonicalStateApproved) {
				tx.Rollback()
				return registrycontract.NewSanitizedError(registrycontract.CodeSecretMaterial, "use approval transition for approved state")
			}
			if err := tx.Commit(); err != nil {
				return fmt.Errorf("commit mapping profile: %w", err)
			}
			return nil
		}
		var existingState string
		err = tx.QueryRowContext(ctx, `SELECT state FROM mapping_profiles WHERE profile_id = ?`, profile.ProfileID).Scan(&existingState)
		if err == nil {
			tx.Rollback()
			return registrycontract.NewSanitizedError(registrycontract.CodeSecretMaterial, "approved mapping profile revisions are immutable")
		}
		if !errors.Is(err, sql.ErrNoRows) {
			tx.Rollback()
			return fmt.Errorf("read existing mapping profile: %w", err)
		}
		if string(profile.State) == string(registrycontract.CanonicalStateApproved) {
			tx.Rollback()
			return registrycontract.NewSanitizedError(registrycontract.CodeSecretMaterial, "cannot insert approved revision")
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO mapping_profiles (
				profile_id, logical_key, canonical_logical_key, canonical_revision,
				source_connection_id, destination_connection_id, source_scope, destination_scope,
				major_version, minor_version, revision, state, content_json, content_hash,
				created_by, created_at, approved_by, approved_at, notes
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			profile.ProfileID, profile.LogicalKey, profile.CanonicalLogicalKey, profile.CanonicalRevision,
			profile.SourceConnectionID, profile.DestinationConnectionID, profile.SourceScope, profile.DestinationScope,
			profile.MajorVersion, profile.MinorVersion, profile.Revision, string(profile.State),
			profile.ContentJSON, profile.ContentHash, profile.CreatedBy, formatTime(profile.CreatedAt),
			profile.ApprovedBy, formatTime(profile.ApprovedAt), profile.Notes,
		)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("insert mapping profile: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit mapping profile: %w", err)
		}
		return nil
	}

	// fallback
	var existingState string
	err = r.db.QueryRowContext(ctx, `SELECT state FROM mapping_profiles WHERE profile_id = ?`, profile.ProfileID).Scan(&existingState)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("read existing mapping profile: %w", err)
	}
	if err == nil {
		if existingState == string(registrycontract.CanonicalStateApproved) {
			return registrycontract.NewSanitizedError(registrycontract.CodeSecretMaterial, "approved mapping profile revisions are immutable")
		}
		if string(profile.State) == string(registrycontract.CanonicalStateApproved) {
			return registrycontract.NewSanitizedError(registrycontract.CodeSecretMaterial, "use approval transition for approved state")
		}
		res, err := r.db.ExecContext(ctx, `
			UPDATE mapping_profiles SET
				logical_key = ?, canonical_logical_key = ?, canonical_revision = ?,
				source_connection_id = ?, destination_connection_id = ?, source_scope = ?, destination_scope = ?,
				major_version = ?, minor_version = ?, revision = ?, state = ?, content_json = ?, content_hash = ?,
				created_by = ?, created_at = ?, approved_by = ?, approved_at = ?, notes = ?
			WHERE profile_id = ? AND state = 'draft'`,
			profile.LogicalKey, profile.CanonicalLogicalKey, profile.CanonicalRevision,
			profile.SourceConnectionID, profile.DestinationConnectionID, profile.SourceScope, profile.DestinationScope,
			profile.MajorVersion, profile.MinorVersion, profile.Revision, string(profile.State),
			profile.ContentJSON, profile.ContentHash, profile.CreatedBy, formatTime(profile.CreatedAt),
			profile.ApprovedBy, formatTime(profile.ApprovedAt), profile.Notes,
			profile.ProfileID,
		)
		if err != nil {
			return fmt.Errorf("update mapping profile: %w", err)
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			return registrycontract.NewSanitizedError(registrycontract.CodeSecretMaterial, "approved mapping profile revisions are immutable")
		}
		return nil
	}

	if string(profile.State) == string(registrycontract.CanonicalStateApproved) {
		return registrycontract.NewSanitizedError(registrycontract.CodeSecretMaterial, "cannot insert approved revision")
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO mapping_profiles (
			profile_id, logical_key, canonical_logical_key, canonical_revision,
			source_connection_id, destination_connection_id, source_scope, destination_scope,
			major_version, minor_version, revision, state, content_json, content_hash,
			created_by, created_at, approved_by, approved_at, notes
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		profile.ProfileID, profile.LogicalKey, profile.CanonicalLogicalKey, profile.CanonicalRevision,
		profile.SourceConnectionID, profile.DestinationConnectionID, profile.SourceScope, profile.DestinationScope,
		profile.MajorVersion, profile.MinorVersion, profile.Revision, string(profile.State),
		profile.ContentJSON, profile.ContentHash, profile.CreatedBy, formatTime(profile.CreatedAt),
		profile.ApprovedBy, formatTime(profile.ApprovedAt), profile.Notes,
	)
	if err != nil {
		return fmt.Errorf("insert mapping profile: %w", err)
	}
	return nil
}

func (r *MappingProfileRepository) Get(profileID string) (registrycontract.MappingProfileRevision, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	row := r.db.QueryRowContext(ctx, `
		SELECT profile_id, logical_key, canonical_logical_key, canonical_revision,
			source_connection_id, destination_connection_id, source_scope, destination_scope,
			major_version, minor_version, revision, state, content_json, content_hash,
			created_by, created_at, approved_by, approved_at, notes
		FROM mapping_profiles WHERE profile_id = ?`, profileID)
	p, err := scanMappingProfile(row)
	if errors.Is(err, sql.ErrNoRows) {
		return registrycontract.MappingProfileRevision{}, false, nil
	}
	if err != nil {
		return registrycontract.MappingProfileRevision{}, false, err
	}
	return p, true, nil
}

func (r *MappingProfileRepository) GetByLogical(logicalKey string, revision int) (registrycontract.MappingProfileRevision, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	row := r.db.QueryRowContext(ctx, `
		SELECT profile_id, logical_key, canonical_logical_key, canonical_revision,
			source_connection_id, destination_connection_id, source_scope, destination_scope,
			major_version, minor_version, revision, state, content_json, content_hash,
			created_by, created_at, approved_by, approved_at, notes
		FROM mapping_profiles WHERE logical_key = ? AND revision = ?`, logicalKey, revision)
	p, err := scanMappingProfile(row)
	if errors.Is(err, sql.ErrNoRows) {
		return registrycontract.MappingProfileRevision{}, false, nil
	}
	if err != nil {
		return registrycontract.MappingProfileRevision{}, false, err
	}
	return p, true, nil
}

func (r *MappingProfileRepository) ListByLogical(logicalKey string) ([]registrycontract.MappingProfileRevision, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := r.db.QueryContext(ctx, `
		SELECT profile_id, logical_key, canonical_logical_key, canonical_revision,
			source_connection_id, destination_connection_id, source_scope, destination_scope,
			major_version, minor_version, revision, state, content_json, content_hash,
			created_by, created_at, approved_by, approved_at, notes
		FROM mapping_profiles WHERE logical_key = ? ORDER BY revision ASC`, logicalKey)
	if err != nil {
		return nil, fmt.Errorf("list mapping profiles: %w", err)
	}
	defer rows.Close()
	return scanMappingProfiles(rows)
}

func (r *MappingProfileRepository) ListAll() ([]registrycontract.MappingProfileRevision, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := r.db.QueryContext(ctx, `
		SELECT profile_id, logical_key, canonical_logical_key, canonical_revision,
			source_connection_id, destination_connection_id, source_scope, destination_scope,
			major_version, minor_version, revision, state, content_json, content_hash,
			created_by, created_at, approved_by, approved_at, notes
		FROM mapping_profiles ORDER BY logical_key ASC, revision ASC`)
	if err != nil {
		return nil, fmt.Errorf("list all mapping profiles: %w", err)
	}
	defer rows.Close()
	return scanMappingProfiles(rows)
}

func (r *MappingProfileRepository) ListBySource(connectionID string) ([]registrycontract.MappingProfileRevision, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := r.db.QueryContext(ctx, `
		SELECT profile_id, logical_key, canonical_logical_key, canonical_revision,
			source_connection_id, destination_connection_id, source_scope, destination_scope,
			major_version, minor_version, revision, state, content_json, content_hash,
			created_by, created_at, approved_by, approved_at, notes
		FROM mapping_profiles WHERE source_connection_id = ? OR destination_connection_id = ? ORDER BY logical_key ASC, revision ASC`,
		connectionID, connectionID)
	if err != nil {
		return nil, fmt.Errorf("list mapping profiles by source: %w", err)
	}
	defer rows.Close()
	return scanMappingProfiles(rows)
}

func scanMappingProfiles(rows *sql.Rows) ([]registrycontract.MappingProfileRevision, error) {
	out := make([]registrycontract.MappingProfileRevision, 0)
	for rows.Next() {
		p, err := scanMappingProfile(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate mapping profiles: %w", err)
	}
	return out, nil
}

func scanMappingProfile(s scanner) (registrycontract.MappingProfileRevision, error) {
	var (
		p        registrycontract.MappingProfileRevision
		state    string
		created  string
		approved string
	)
	if err := s.Scan(
		&p.ProfileID, &p.LogicalKey, &p.CanonicalLogicalKey, &p.CanonicalRevision,
		&p.SourceConnectionID, &p.DestinationConnectionID, &p.SourceScope, &p.DestinationScope,
		&p.MajorVersion, &p.MinorVersion, &p.Revision, &state, &p.ContentJSON, &p.ContentHash,
		&p.CreatedBy, &created, &p.ApprovedBy, &approved, &p.Notes,
	); err != nil {
		return registrycontract.MappingProfileRevision{}, err
	}
	p.State = registrycontract.CanonicalRevisionState(state)
	var err error
	if p.CreatedAt, err = parseTime(created); err != nil {
		return registrycontract.MappingProfileRevision{}, err
	}
	if approved != "" {
		if p.ApprovedAt, err = parseTime(approved); err != nil {
			return registrycontract.MappingProfileRevision{}, err
		}
	}
	return p, nil
}

// RevisionHistoryRepository records immutable revision history.
type RevisionHistoryRepository struct {
	db execer
}

func (r *RevisionHistoryRepository) Record(entry registrycontract.RevisionHistoryEntry) error {
	if strings.TrimSpace(entry.HistoryID) == "" || strings.TrimSpace(entry.ResourceID) == "" {
		return registrycontract.NewSanitizedError(registrycontract.CodeInvalidProfile, "A history entry requires an id and resource id.")
	}
	if entry.DetailJSON != "" {
		if err := validateSafeJSONNoSecrets(entry.DetailJSON); err != nil {
			return err
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO revision_history (history_id, resource_type, resource_id, logical_key, revision, action, actor, at, detail_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.HistoryID, entry.ResourceType, entry.ResourceID, entry.LogicalKey, entry.Revision,
		entry.Action, entry.Actor, formatTime(entry.At), entry.DetailJSON,
	)
	if err != nil {
		return fmt.Errorf("record revision history: %w", err)
	}
	return nil
}

func (r *RevisionHistoryRepository) ListByResource(resourceType, resourceID string) ([]registrycontract.RevisionHistoryEntry, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := r.db.QueryContext(ctx, `
		SELECT history_id, resource_type, resource_id, logical_key, revision, action, actor, at, detail_json
		FROM revision_history WHERE resource_type = ? AND resource_id = ? ORDER BY at ASC, history_id ASC`,
		resourceType, resourceID)
	if err != nil {
		return nil, fmt.Errorf("list revision history: %w", err)
	}
	defer rows.Close()
	return scanHistory(rows)
}

func (r *RevisionHistoryRepository) ListByLogical(logicalKey string) ([]registrycontract.RevisionHistoryEntry, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := r.db.QueryContext(ctx, `
		SELECT history_id, resource_type, resource_id, logical_key, revision, action, actor, at, detail_json
		FROM revision_history WHERE logical_key = ? ORDER BY at ASC, history_id ASC`,
		logicalKey)
	if err != nil {
		return nil, fmt.Errorf("list revision history by logical: %w", err)
	}
	defer rows.Close()
	return scanHistory(rows)
}

func scanHistory(rows *sql.Rows) ([]registrycontract.RevisionHistoryEntry, error) {
	out := make([]registrycontract.RevisionHistoryEntry, 0)
	for rows.Next() {
		var (
			e  registrycontract.RevisionHistoryEntry
			at string
		)
		if err := rows.Scan(&e.HistoryID, &e.ResourceType, &e.ResourceID, &e.LogicalKey, &e.Revision, &e.Action, &e.Actor, &at, &e.DetailJSON); err != nil {
			return nil, err
		}
		var err error
		if e.At, err = parseTime(at); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate revision history: %w", err)
	}
	return out, nil
}

func validateSafeJSONNoSecrets(raw string) error {
	blob := strings.ToLower(raw)
	for _, marker := range []string{"password", "token", "apikey", "api_key", "secret", "bearer ", "://", "@", "authorization"} {
		if marker == "://" || marker == "@" {
			if strings.Contains(blob, marker) {
				return registrycontract.NewSanitizedError(registrycontract.CodeSecretMaterial, "Content must not contain secrets, tokens, credential references or endpoints.")
			}
			continue
		}
		if strings.Contains(blob, marker) {
			return registrycontract.NewSanitizedError(registrycontract.CodeSecretMaterial, "Content must not contain secrets, tokens, credential references or endpoints.")
		}
	}
	return nil
}

// ApproveCanonicalSchema atomically transitions a draft revision to approved
func (r *CanonicalSchemaRepository) ApproveCanonicalSchema(ctx context.Context, schemaID, actor string, approvedAt time.Time) error {
	ctx2, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	res, err := r.db.ExecContext(ctx2, `
		UPDATE canonical_schemas 
		SET state = 'approved', approved_by = ?, approved_at = ?
		WHERE schema_id = ? AND state = 'draft'`,
		actor, formatTime(approvedAt), schemaID,
	)
	if err != nil {
		return fmt.Errorf("approve canonical schema: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return registrycontract.NewSanitizedError(registrycontract.CodeSecretMaterial, "cannot approve: not found or not in draft state")
	}
	return nil
}

// ApproveMappingProfile atomically transitions a draft mapping profile to approved.
func (r *MappingProfileRepository) ApproveMappingProfile(ctx context.Context, profileID, actor string, approvedAt time.Time) error {
	ctx2, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	res, err := r.db.ExecContext(ctx2, `
		UPDATE mapping_profiles 
		SET state = 'approved', approved_by = ?, approved_at = ?
		WHERE profile_id = ? AND state = 'draft'`,
		actor, formatTime(approvedAt), profileID,
	)
	if err != nil {
		return fmt.Errorf("approve mapping profile: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return registrycontract.NewSanitizedError(registrycontract.CodeSecretMaterial, "cannot approve: not found or not in draft state")
	}
	return nil
}
