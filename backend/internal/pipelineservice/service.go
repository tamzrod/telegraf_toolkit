// Package pipelineservice stores pipeline revisions and mode transitions.
//
// It validates configuration and writes history. It does not deploy Telegraf,
// restart a collector, or write a historian.
package pipelineservice

import (
	"context"
	"encoding/json"
	"time"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/metadatastore"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/pipelinecontract"
)

// Error is a secret-free pipeline failure.
type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

const (
	CodeNotFound          = "not-found"
	CodeConflict          = "conflict"
	CodeImmutable         = "immutable"
	CodeInvalid           = "invalid"
	CodeConversionRefused = "conversion-refused"
)

// Review is a mode-transition proposal. Nothing is written.
type Review struct {
	FromMode     string
	ToMode       string
	CurrentText  string
	ProposedText string
	Proposed     pipelinecontract.Document
	Digest       string
}

// MappingCompiler resolves an approved mapping pin into processors.
// Implementations must not modify mapping rows.
type MappingCompiler interface {
	Prepare(ctx context.Context, doc pipelinecontract.Document) (pipelinecontract.Document, error)
}

// Service is the pipeline revision lifecycle.
type Service struct {
	revisions *metadatastore.PipelineRepository
	mapping   MappingCompiler
	now       func() time.Time
}

// SetMappingCompiler attaches the read-only mapping compiler used when a
// graph cites a mapping. Export and mode transitions compile a copy.
func (s *Service) SetMappingCompiler(compiler MappingCompiler) {
	s.mapping = compiler
}

// PrepareGraph compiles a mapping pin on a copy. Documents without a pin are
// returned unchanged. The stored revision is not written.
func (s *Service) PrepareGraph(ctx context.Context, doc pipelinecontract.Document) (pipelinecontract.Document, error) {
	return prepareGraph(ctx, s.mapping, doc)
}

// New returns a service that uses the metadata store clock.
func New(revisions *metadatastore.PipelineRepository) *Service {
	return &Service{revisions: revisions, now: time.Now}
}

// Create stores an empty graph draft. The draft is not deployed.
func (s *Service) Create(ctx context.Context, actor string) (metadatastore.PipelineRevision, error) {
	pipelineID, err := metadatastore.NewPipelineID()
	if err != nil {
		return metadatastore.PipelineRevision{}, err
	}
	revisionID, err := metadatastore.NewRevisionID()
	if err != nil {
		return metadatastore.PipelineRevision{}, err
	}
	raw, err := json.Marshal(emptyGraph())
	if err != nil {
		return metadatastore.PipelineRevision{}, err
	}
	return s.revisions.InsertDraft(ctx, metadatastore.PipelineRevision{
		RevisionID:   revisionID,
		PipelineID:   pipelineID,
		Number:       1,
		Mode:         string(pipelinecontract.ModeGraph),
		DocumentJSON: string(raw),
		CreatedBy:    actor,
		CreatedAt:    s.now().UTC(),
		Generation:   1,
	})
}

// Get loads one revision.
func (s *Service) Get(ctx context.Context, pipelineID string, number int) (metadatastore.PipelineRevision, error) {
	rev, err := s.revisions.Get(ctx, pipelineID, number)
	return rev, storeErr(err)
}

// List loads revision history, oldest first.
func (s *Service) List(ctx context.Context, pipelineID string) ([]metadatastore.PipelineRevision, error) {
	revs, err := s.revisions.List(ctx, pipelineID)
	return revs, storeErr(err)
}

// Save replaces the draft body without changing its mode.
func (s *Service) Save(ctx context.Context, actor, revisionID string, expectedGeneration int, raw []byte) (metadatastore.PipelineRevision, error) {
	doc, err := pipelinecontract.Parse(raw)
	if err != nil {
		return metadatastore.PipelineRevision{}, invalidDocument(err)
	}
	if err := pipelinecontract.Validate(doc); err != nil {
		return metadatastore.PipelineRevision{}, invalidDocument(err)
	}
	encoded, err := json.Marshal(doc)
	if err != nil {
		return metadatastore.PipelineRevision{}, invalidDocument(err)
	}
	rev, err := s.revisions.SaveDraft(ctx, revisionID, actor, string(encoded), string(doc.Mode), expectedGeneration, s.now())
	return rev, storeErr(err)
}

// PreviewTransition builds the difference a mode change would write.
func (s *Service) PreviewTransition(ctx context.Context, pipelineID string, number int, target string) (Review, error) {
	rev, err := s.revisions.Get(ctx, pipelineID, number)
	if err != nil {
		return Review{}, storeErr(err)
	}
	if rev.State != "draft" {
		return Review{}, &Error{Code: CodeImmutable, Message: "An approved revision cannot change mode."}
	}
	return buildReview(ctx, s.mapping, rev, target)
}

// CommitTransition writes a reviewed mode change. The digest must match the preview.
func (s *Service) CommitTransition(ctx context.Context, actor, revisionID string, expectedGeneration int, target, digest string) (metadatastore.PipelineRevision, error) {
	return s.commitReviewed(ctx, actor, revisionID, expectedGeneration, target, digest, false)
}

// RestoreSnapshot swaps the draft with its stored snapshot after the same review.
func (s *Service) RestoreSnapshot(ctx context.Context, actor, revisionID string, expectedGeneration int, digest string) (metadatastore.PipelineRevision, error) {
	return s.commitReviewed(ctx, actor, revisionID, expectedGeneration, "snapshot", digest, true)
}

func (s *Service) commitReviewed(ctx context.Context, actor, revisionID string, expectedGeneration int, target, digest string, restore bool) (metadatastore.PipelineRevision, error) {
	current, err := s.revisions.GetByID(ctx, revisionID)
	if err != nil {
		return metadatastore.PipelineRevision{}, storeErr(err)
	}
	review, err := buildReview(ctx, s.mapping, current, target)
	if err != nil {
		return metadatastore.PipelineRevision{}, err
	}
	if review.Digest != digest {
		return metadatastore.PipelineRevision{}, &Error{Code: CodeConflict, Message: "The reviewed difference is stale. Review the transition again."}
	}
	encoded, err := json.Marshal(review.Proposed)
	if err != nil {
		return metadatastore.PipelineRevision{}, invalidDocument(err)
	}
	documentJSON := string(encoded)
	write := s.revisions.TransitionDraft
	if restore {
		documentJSON = current.SnapshotJSON
		write = s.revisions.RestoreDraft
	}
	rev, err := write(ctx, revisionID, actor, documentJSON, current.DocumentJSON, string(review.Proposed.Mode), expectedGeneration, s.now())
	return rev, storeErr(err)
}

// Approve freezes a draft. It revalidates the stored document so a legacy or
// damaged draft cannot become immutable without a valid manual TOML body.
// It does not deploy the document.
func (s *Service) Approve(ctx context.Context, actor, revisionID string, expectedGeneration int) (metadatastore.PipelineRevision, error) {
	current, err := s.revisions.GetByID(ctx, revisionID)
	if err != nil {
		return metadatastore.PipelineRevision{}, storeErr(err)
	}
	doc, err := pipelinecontract.Parse([]byte(current.DocumentJSON))
	if err != nil {
		return metadatastore.PipelineRevision{}, invalidDocument(err)
	}
	if err := pipelinecontract.Validate(doc); err != nil {
		return metadatastore.PipelineRevision{}, invalidDocument(err)
	}
	rev, err := s.revisions.ApproveDraft(ctx, revisionID, actor, expectedGeneration, s.now())
	return rev, storeErr(err)
}

// OpenDraft copies a historical revision into a new draft. The source row is
// not modified. Rollback is this copy, not an in-place rewrite.
func (s *Service) OpenDraft(ctx context.Context, actor, pipelineID string, sourceNumber int) (metadatastore.PipelineRevision, error) {
	history, err := s.revisions.List(ctx, pipelineID)
	if err != nil {
		return metadatastore.PipelineRevision{}, storeErr(err)
	}
	var source metadatastore.PipelineRevision
	found := false
	next := 1
	for _, rev := range history {
		if rev.State == "draft" {
			return metadatastore.PipelineRevision{}, &Error{Code: CodeConflict, Message: "An open draft already exists. Approve it or keep editing it."}
		}
		if rev.Number == sourceNumber {
			source = rev
			found = true
		}
		if rev.Number >= next {
			next = rev.Number + 1
		}
	}
	if !found {
		return metadatastore.PipelineRevision{}, &Error{Code: CodeNotFound, Message: "The source revision does not exist."}
	}
	revisionID, err := metadatastore.NewRevisionID()
	if err != nil {
		return metadatastore.PipelineRevision{}, err
	}
	opened, err := s.revisions.InsertOpenedDraft(ctx, metadatastore.PipelineRevision{
		RevisionID:       revisionID,
		PipelineID:       pipelineID,
		Number:           next,
		Mode:             source.Mode,
		DocumentJSON:     source.DocumentJSON,
		ParentRevisionID: source.RevisionID,
		CreatedBy:        actor,
		CreatedAt:        s.now().UTC(),
		Generation:       1,
	})
	return opened, storeErr(err)
}

func emptyGraph() pipelinecontract.Document {
	return pipelinecontract.Document{
		SchemaVersion:   pipelinecontract.SchemaVersion,
		TelegrafVersion: pipelinecontract.SupportedTelegrafVersion,
		Mode:            pipelinecontract.ModeGraph,
		Graph: &pipelinecontract.Graph{
			Instances:                       []pipelinecontract.Instance{},
			ProcessorOrder:                  []string{},
			SkipProcessorsBeforeAggregators: pipelinecontract.BoolPtr(false),
			SkipProcessorsAfterAggregators:  pipelinecontract.BoolPtr(false),
		},
	}
}
