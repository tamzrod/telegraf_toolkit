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

	// Enforce complete immutability of approved revisions at the SQLite boundary.
	var (
		existingState    string
		existingLogical  string
		existingMajor    int
		existingMinor    int
		existingRevision int
		existingContent  string
		existingHash     string
		existingCreated  string
		existingApproved string
		existingCreator  string
		existingApprover string
		existingNotes    string
	)
	err := r.db.QueryRowContext(ctx, `SELECT state, logical_key, major_version, minor_version, revision, content_json, content_hash, created_by, created_at, approved_by, approved_at, notes FROM canonical_schemas WHERE schema_id = ?`,
		schema.SchemaID).Scan(&existingState, &existingLogical, &existingMajor, &existingMinor, &existingRevision, &existingContent, &existingHash, &existingCreator, &existingCreated, &existingApprover, &existingApproved, &existingNotes)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("read existing canonical schema: %w", err)
	}
	if err == nil && existingState == string(registrycontract.CanonicalStateApproved) {
		// Approved revisions are immutable. Do not allow state regression or metadata/content changes.
		if string(schema.State) != existingState {
			return registrycontract.NewSanitizedError(registrycontract.CodeSecretMaterial, "approved canonical schema revisions are immutable")
		}
		if schema.LogicalKey != existingLogical || schema.MajorVersion != existingMajor || schema.MinorVersion != existingMinor || schema.Revision != existingRevision {
			return registrycontract.NewSanitizedError(registrycontract.CodeSecretMaterial, "approved canonical schema revisions are immutable")
		}
		if schema.ContentJSON != existingContent || schema.ContentHash != existingHash {
			return registrycontract.NewSanitizedError(registrycontract.CodeSecretMaterial, "approved canonical schema revisions are immutable")
		}
		if schema.CreatedBy != existingCreator || schema.ApprovedBy != existingApprover || schema.Notes != existingNotes {
			// Allow updates only if they match existing approved metadata; otherwise reject
			if schema.CreatedBy != existingCreator || schema.ApprovedBy != existingApprover {
				return registrycontract.NewSanitizedError(registrycontract.CodeSecretMaterial, "approved canonical schema revisions are immutable")
			}
		}
		// Timestamps must remain consistent
		if formatTime(schema.CreatedAt) != existingCreated || formatTime(schema.ApprovedAt) != existingApproved {
			return registrycontract.NewSanitizedError(registrycontract.CodeSecretMaterial, "approved canonical schema revisions are immutable")
		}
	}

	_, err = r.db.ExecContext(ctx, `
		INSERT INTO canonical_schemas (
			schema_id, logical_key, major_version, minor_version, revision,
			state, content_json, content_hash, created_by, created_at,
			approved_by, approved_at, notes
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(schema_id) DO UPDATE SET
			logical_key = excluded.logical_key,
			major_version = excluded.major_version,
			minor_version = excluded.minor_version,
			revision = excluded.revision,
			state = excluded.state,
			content_json = excluded.content_json,
			content_hash = excluded.content_hash,
			created_by = excluded.created_by,
			created_at = excluded.created_at,
			approved_by = excluded.approved_by,
			approved_at = excluded.approved_at,
			notes = excluded.notes`,
		schema.SchemaID, schema.LogicalKey, schema.MajorVersion, schema.MinorVersion, schema.Revision,
		string(schema.State), schema.ContentJSON, schema.ContentHash, schema.CreatedBy,
		formatTime(schema.CreatedAt), schema.ApprovedBy, formatTime(schema.ApprovedAt), schema.Notes,
	)
	if err != nil {
		return fmt.Errorf("upsert canonical schema: %w", err)
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

	var (
		existingState    string
		existingLogical  string
		existingCanonLog string
		existingCanonRev int
		existingSrcConn  string
		existingDstConn  string
		existingSrcScope string
		existingDstScope string
		existingMajor    int
		existingMinor    int
		existingRevision int
		existingContent  string
		existingHash     string
		existingCreator  string
		existingApprover string
		existingCreated  string
		existingApproved string
		existingNotes    string
	)
	err := r.db.QueryRowContext(ctx, `SELECT state, logical_key, canonical_logical_key, canonical_revision, source_connection_id, destination_connection_id, source_scope, destination_scope, major_version, minor_version, revision, content_json, content_hash, created_by, approved_by, created_at, approved_at, notes FROM mapping_profiles WHERE profile_id = ?`,
		profile.ProfileID).Scan(&existingState, &existingLogical, &existingCanonLog, &existingCanonRev, &existingSrcConn, &existingDstConn, &existingSrcScope, &existingDstScope, &existingMajor, &existingMinor, &existingRevision, &existingContent, &existingHash, &existingCreator, &existingApprover, &existingCreated, &existingApproved, &existingNotes)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("read existing mapping profile: %w", err)
	}
	if err == nil && existingState == string(registrycontract.CanonicalStateApproved) {
		if string(profile.State) != existingState {
			return registrycontract.NewSanitizedError(registrycontract.CodeSecretMaterial, "approved mapping profile revisions are immutable")
		}
		if profile.LogicalKey != existingLogical || profile.CanonicalLogicalKey != existingCanonLog || profile.CanonicalRevision != existingCanonRev {
			return registrycontract.NewSanitizedError(registrycontract.CodeSecretMaterial, "approved mapping profile revisions are immutable")
		}
		if profile.SourceConnectionID != existingSrcConn || profile.DestinationConnectionID != existingDstConn || profile.SourceScope != existingSrcScope || profile.DestinationScope != existingDstScope {
			return registrycontract.NewSanitizedError(registrycontract.CodeSecretMaterial, "approved mapping profile revisions are immutable")
		}
		if profile.MajorVersion != existingMajor || profile.MinorVersion != existingMinor || profile.Revision != existingRevision {
			return registrycontract.NewSanitizedError(registrycontract.CodeSecretMaterial, "approved mapping profile revisions are immutable")
		}
		if profile.ContentJSON != existingContent || profile.ContentHash != existingHash {
			return registrycontract.NewSanitizedError(registrycontract.CodeSecretMaterial, "approved mapping profile revisions are immutable")
		}
		if profile.CreatedBy != existingCreator || profile.ApprovedBy != existingApprover {
			return registrycontract.NewSanitizedError(registrycontract.CodeSecretMaterial, "approved mapping profile revisions are immutable")
		}
		if formatTime(profile.CreatedAt) != existingCreated || formatTime(profile.ApprovedAt) != existingApproved {
			return registrycontract.NewSanitizedError(registrycontract.CodeSecretMaterial, "approved mapping profile revisions are immutable")
		}
	}

	_, err = r.db.ExecContext(ctx, `
		INSERT INTO mapping_profiles (
			profile_id, logical_key, canonical_logical_key, canonical_revision,
			source_connection_id, destination_connection_id, source_scope, destination_scope,
			major_version, minor_version, revision, state, content_json, content_hash,
			created_by, created_at, approved_by, approved_at, notes
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(profile_id) DO UPDATE SET
			logical_key = excluded.logical_key,
			canonical_logical_key = excluded.canonical_logical_key,
			canonical_revision = excluded.canonical_revision,
			source_connection_id = excluded.source_connection_id,
			destination_connection_id = excluded.destination_connection_id,
			source_scope = excluded.source_scope,
			destination_scope = excluded.destination_scope,
			major_version = excluded.major_version,
			minor_version = excluded.minor_version,
			revision = excluded.revision,
			state = excluded.state,
			content_json = excluded.content_json,
			content_hash = excluded.content_hash,
			created_by = excluded.created_by,
			created_at = excluded.created_at,
			approved_by = excluded.approved_by,
			approved_at = excluded.approved_at,
			notes = excluded.notes`,
		profile.ProfileID, profile.LogicalKey, profile.CanonicalLogicalKey, profile.CanonicalRevision,
		profile.SourceConnectionID, profile.DestinationConnectionID, profile.SourceScope, profile.DestinationScope,
		profile.MajorVersion, profile.MinorVersion, profile.Revision, string(profile.State),
		profile.ContentJSON, profile.ContentHash, profile.CreatedBy, formatTime(profile.CreatedAt),
		profile.ApprovedBy, formatTime(profile.ApprovedAt), profile.Notes,
	)
	if err != nil {
		return fmt.Errorf("upsert mapping profile: %w", err)
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
	// basic redaction check using same secret markers
	blob := strings.ToLower(raw)
	for _, marker := range []string{"password", "token", "apikey", "api_key", "secret", "bearer ", "://", "@", "authorization"} {
		// allow simple patterns but we must not have credentials/URLs/tokens as per spec
		// spec says no secrets/tokens/credential refs/endpoints in export/import JSON
		if marker == "://" || marker == "@" {
			// endpoints/URLs with creds
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
