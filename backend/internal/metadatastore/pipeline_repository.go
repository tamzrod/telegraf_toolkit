package metadatastore

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/registrycontract"
)

// Sentinel errors for pipeline revision writes. Messages stay in the service
// layer; these values are for errors.Is only.
var (
	ErrPipelineNotFound  = errors.New("pipeline revision not found")
	ErrPipelineConflict  = errors.New("pipeline revision conflict")
	ErrPipelineImmutable = errors.New("approved pipeline revision is immutable")
)

// PipelineRevision is one stored configuration revision. DocumentJSON is the
// pipeline contract document. SnapshotJSON is the previous authoritative
// document captured at a mode transition, or empty.
type PipelineRevision struct {
	RevisionID       string
	PipelineID       string
	Number           int
	State            string
	Mode             string
	DocumentJSON     string
	SnapshotJSON     string
	ParentRevisionID string
	CreatedBy        string
	CreatedAt        time.Time
	ApprovedBy       string
	ApprovedAt       time.Time
	Generation       int
}

// PipelineRepository persists pipeline revisions in the metadata store.
type PipelineRepository struct {
	db *sql.DB
}

// NewPipelineID returns a new pipeline id.
func NewPipelineID() (string, error) { return newOpaqueID("p") }

// NewRevisionID returns a new revision id.
func NewRevisionID() (string, error) { return newOpaqueID("r") }

func newOpaqueID(prefix string) (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", fmt.Errorf("new pipeline id: %w", err)
	}
	return prefix + hex.EncodeToString(buf[:]), nil
}

// InsertDraft creates revision 1, or a later draft when Number is already set.
// The pipeline must not already have an open draft.
func (r *PipelineRepository) InsertDraft(ctx context.Context, rev PipelineRevision) (PipelineRevision, error) {
	return r.insertDraft(ctx, rev, "pipeline.create")
}

func (r *PipelineRepository) InsertOpenedDraft(ctx context.Context, rev PipelineRevision) (PipelineRevision, error) {
	return r.insertDraft(ctx, rev, "pipeline.open-draft")
}

func (r *PipelineRepository) insertDraft(ctx context.Context, rev PipelineRevision, action string) (PipelineRevision, error) {
	if rev.RevisionID == "" || rev.PipelineID == "" || rev.Number < 1 {
		return PipelineRevision{}, fmt.Errorf("pipeline draft is incomplete")
	}
	if rev.Generation < 1 {
		rev.Generation = 1
	}
	now := rev.CreatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	rev.CreatedAt = now.UTC()
	rev.State = "draft"

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return PipelineRevision{}, fmt.Errorf("begin pipeline insert: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO pipeline_revisions (
			revision_id, pipeline_id, revision, state, mode, document_json, snapshot_json,
			parent_revision_id, created_by, created_at, approved_by, approved_at, save_generation
		) VALUES (?, ?, ?, 'draft', ?, ?, ?, ?, ?, ?, '', '', ?)`,
		rev.RevisionID, rev.PipelineID, rev.Number, rev.Mode, rev.DocumentJSON, rev.SnapshotJSON,
		rev.ParentRevisionID, rev.CreatedBy, formatTime(rev.CreatedAt), rev.Generation,
	)
	if isUniqueViolation(err) {
		return PipelineRevision{}, ErrPipelineConflict
	}
	if err != nil {
		return PipelineRevision{}, fmt.Errorf("insert pipeline revision: %w", err)
	}
	if err := insertPipelineAudit(ctx, tx, rev.CreatedBy, action, rev.PipelineID, rev.CreatedAt); err != nil {
		return PipelineRevision{}, err
	}
	if err := tx.Commit(); err != nil {
		return PipelineRevision{}, fmt.Errorf("commit pipeline insert: %w", err)
	}
	return rev, nil
}

// GetByID returns one revision by its id.
func (r *PipelineRepository) GetByID(ctx context.Context, revisionID string) (PipelineRevision, error) {
	row := r.db.QueryRowContext(ctx, pipelineSelect+` WHERE revision_id = ?`, revisionID)
	rev, err := scanPipeline(row)
	if errors.Is(err, sql.ErrNoRows) {
		return PipelineRevision{}, ErrPipelineNotFound
	}
	return rev, err
}

// Get returns one revision.
func (r *PipelineRepository) Get(ctx context.Context, pipelineID string, number int) (PipelineRevision, error) {
	row := r.db.QueryRowContext(ctx, pipelineSelect+` WHERE pipeline_id = ? AND revision = ?`, pipelineID, number)
	rev, err := scanPipeline(row)
	if errors.Is(err, sql.ErrNoRows) {
		return PipelineRevision{}, ErrPipelineNotFound
	}
	return rev, err
}

// List returns every revision of a pipeline, oldest first.
func (r *PipelineRepository) List(ctx context.Context, pipelineID string) ([]PipelineRevision, error) {
	rows, err := r.db.QueryContext(ctx, pipelineSelect+` WHERE pipeline_id = ? ORDER BY revision`, pipelineID)
	if err != nil {
		return nil, fmt.Errorf("list pipeline revisions: %w", err)
	}
	defer rows.Close()
	out := []PipelineRevision{}
	for rows.Next() {
		rev, err := scanPipeline(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rev)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pipeline revisions: %w", err)
	}
	if len(out) == 0 {
		return nil, ErrPipelineNotFound
	}
	return out, nil
}

// SaveDraft replaces a draft document when the generation still matches.
// The mode column must stay the same. Approved rows are not updated.
func (r *PipelineRepository) SaveDraft(ctx context.Context, revisionID, actor, documentJSON, mode string, expectedGeneration int, at time.Time) (PipelineRevision, error) {
	return r.mutate(ctx, revisionID, actor, "pipeline.save", expectedGeneration, at, func(current PipelineRevision) (string, string, string, error) {
		if current.Mode != mode {
			return "", "", "", ErrPipelineConflict
		}
		return documentJSON, current.SnapshotJSON, mode, nil
	})
}

// TransitionDraft stores a new authoritative document and keeps the previous
// document as the recoverable snapshot.
func (r *PipelineRepository) TransitionDraft(ctx context.Context, revisionID, actor, documentJSON, snapshotJSON, mode string, expectedGeneration int, at time.Time) (PipelineRevision, error) {
	return r.mutate(ctx, revisionID, actor, "pipeline.transition", expectedGeneration, at, func(PipelineRevision) (string, string, string, error) {
		return documentJSON, snapshotJSON, mode, nil
	})
}

// RestoreDraft swaps the draft document with its stored snapshot.
func (r *PipelineRepository) RestoreDraft(ctx context.Context, revisionID, actor, documentJSON, snapshotJSON, mode string, expectedGeneration int, at time.Time) (PipelineRevision, error) {
	return r.mutate(ctx, revisionID, actor, "pipeline.restore", expectedGeneration, at, func(PipelineRevision) (string, string, string, error) {
		return documentJSON, snapshotJSON, mode, nil
	})
}

// ApproveDraft marks a draft approved without changing its document.
func (r *PipelineRepository) ApproveDraft(ctx context.Context, revisionID, actor string, expectedGeneration int, at time.Time) (PipelineRevision, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return PipelineRevision{}, fmt.Errorf("begin pipeline approve: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	current, err := lockPipeline(ctx, tx, revisionID)
	if err != nil {
		return PipelineRevision{}, err
	}
	if current.State != "draft" {
		return PipelineRevision{}, ErrPipelineImmutable
	}
	if current.Generation != expectedGeneration {
		return PipelineRevision{}, ErrPipelineConflict
	}
	at = at.UTC()
	res, err := tx.ExecContext(ctx, `
		UPDATE pipeline_revisions
		SET state = 'approved', approved_by = ?, approved_at = ?
		WHERE revision_id = ? AND state = 'draft' AND save_generation = ?`,
		actor, formatTime(at), revisionID, expectedGeneration,
	)
	if err != nil {
		return PipelineRevision{}, fmt.Errorf("approve pipeline revision: %w", err)
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return PipelineRevision{}, ErrPipelineConflict
	}
	if err := insertPipelineAudit(ctx, tx, actor, "pipeline.approve", current.PipelineID, at); err != nil {
		return PipelineRevision{}, err
	}
	if err := tx.Commit(); err != nil {
		return PipelineRevision{}, fmt.Errorf("commit pipeline approve: %w", err)
	}
	current.State = "approved"
	current.ApprovedBy = actor
	current.ApprovedAt = at
	return current, nil
}

type draftChange func(current PipelineRevision) (documentJSON, snapshotJSON, mode string, err error)

func (r *PipelineRepository) mutate(ctx context.Context, revisionID, actor, action string, expectedGeneration int, at time.Time, change draftChange) (PipelineRevision, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return PipelineRevision{}, fmt.Errorf("begin pipeline write: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	current, err := lockPipeline(ctx, tx, revisionID)
	if err != nil {
		return PipelineRevision{}, err
	}
	if current.State != "draft" {
		return PipelineRevision{}, ErrPipelineImmutable
	}
	if current.Generation != expectedGeneration {
		return PipelineRevision{}, ErrPipelineConflict
	}
	documentJSON, snapshotJSON, mode, err := change(current)
	if err != nil {
		return PipelineRevision{}, err
	}
	at = at.UTC()
	res, err := tx.ExecContext(ctx, `
		UPDATE pipeline_revisions
		SET document_json = ?, snapshot_json = ?, mode = ?, save_generation = save_generation + 1
		WHERE revision_id = ? AND state = 'draft' AND save_generation = ?`,
		documentJSON, snapshotJSON, mode, revisionID, expectedGeneration,
	)
	if err != nil {
		return PipelineRevision{}, fmt.Errorf("update pipeline revision: %w", err)
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return PipelineRevision{}, ErrPipelineConflict
	}
	if err := insertPipelineAudit(ctx, tx, actor, action, current.PipelineID, at); err != nil {
		return PipelineRevision{}, err
	}
	if err := tx.Commit(); err != nil {
		return PipelineRevision{}, fmt.Errorf("commit pipeline write: %w", err)
	}
	current.DocumentJSON = documentJSON
	current.SnapshotJSON = snapshotJSON
	current.Mode = mode
	current.Generation++
	return current, nil
}

func lockPipeline(ctx context.Context, tx *sql.Tx, revisionID string) (PipelineRevision, error) {
	row := tx.QueryRowContext(ctx, pipelineSelect+` WHERE revision_id = ?`, revisionID)
	rev, err := scanPipeline(row)
	if errors.Is(err, sql.ErrNoRows) {
		return PipelineRevision{}, ErrPipelineNotFound
	}
	if err != nil {
		return PipelineRevision{}, err
	}
	return rev, nil
}

const pipelineSelect = `
	SELECT revision_id, pipeline_id, revision, state, mode, document_json, snapshot_json,
		parent_revision_id, created_by, created_at, approved_by, approved_at, save_generation
	FROM pipeline_revisions`

type pipelineScanner interface {
	Scan(dest ...any) error
}

func scanPipeline(row pipelineScanner) (PipelineRevision, error) {
	var rev PipelineRevision
	var created, approved string
	if err := row.Scan(
		&rev.RevisionID, &rev.PipelineID, &rev.Number, &rev.State, &rev.Mode, &rev.DocumentJSON,
		&rev.SnapshotJSON, &rev.ParentRevisionID, &rev.CreatedBy, &created, &rev.ApprovedBy,
		&approved, &rev.Generation,
	); err != nil {
		return PipelineRevision{}, err
	}
	var err error
	if rev.CreatedAt, err = parseTime(created); err != nil {
		return PipelineRevision{}, err
	}
	if rev.ApprovedAt, err = parseTime(approved); err != nil {
		return PipelineRevision{}, err
	}
	return rev, nil
}

func insertPipelineAudit(ctx context.Context, tx *sql.Tx, actor, action, pipelineID string, at time.Time) error {
	event := registrycontract.AuditEvent{
		Actor:      actor,
		Action:     action,
		TargetKind: registrycontract.TargetPipeline,
		TargetID:   pipelineID,
		Outcome:    registrycontract.OutcomeSucceeded,
		At:         at.UTC(),
	}
	if err := event.Validate(); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO audit_events (
			actor, action, target_kind, target_id, outcome, at, correlation_id, mock
		) VALUES (?, ?, ?, ?, ?, ?, '', 0)`,
		event.Actor, event.Action, string(event.TargetKind), event.TargetID,
		string(event.Outcome), formatTime(event.At),
	)
	if err != nil {
		return fmt.Errorf("audit pipeline revision: %w", err)
	}
	return nil
}
