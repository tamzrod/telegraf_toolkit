package pipelineservice

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/metadatastore"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/pipelinecontract"
)

func TestModeTransitionSnapshotApprovalAndRollback(t *testing.T) {
	svc, store := openService(t)
	ctx := context.Background()
	created, err := svc.Create(ctx, "config-1")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.Mode != "graph" || created.Generation != 1 || created.State != "draft" {
		t.Fatalf("created = %+v", created)
	}

	graph := emptyGraph()
	graph.Graph.Instances = []pipelinecontract.Instance{{
		ID: "file-1", Kind: pipelinecontract.KindOutput, Plugin: "file",
		Options: map[string]pipelinecontract.OptionValue{
			"files":       pipelinecontract.StringListOption("stdout"),
			"data_format": pipelinecontract.StringOption("influx"),
		},
	}}
	raw, _ := json.Marshal(graph)
	saved, err := svc.Save(ctx, "config-1", created.RevisionID, 1, raw)
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	approvedBytes := saved.DocumentJSON

	review, err := svc.PreviewTransition(ctx, created.PipelineID, 1, "manual")
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if review.FromMode != "graph" || review.ToMode != "manual" || review.Digest == "" {
		t.Fatalf("review = %+v", review)
	}
	if review.ProposedText == "" || review.CurrentText != review.ProposedText {
		t.Fatal("graph to manual preview changed the TOML")
	}
	manual, err := svc.CommitTransition(ctx, "config-1", created.RevisionID, saved.Generation, "manual", review.Digest)
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	if manual.Mode != "manual" || manual.SnapshotJSON != approvedBytes {
		t.Fatal("snapshot did not keep the graph document")
	}

	edited := pipelinecontract.Document{
		SchemaVersion:   pipelinecontract.SchemaVersion,
		TelegrafVersion: pipelinecontract.SupportedTelegrafVersion,
		Mode:            pipelinecontract.ModeManual,
		Manual:          &pipelinecontract.Manual{TOML: review.ProposedText + "\n# local comment\n"},
	}
	editedRaw, _ := json.Marshal(edited)
	kept, err := svc.Save(ctx, "config-1", created.RevisionID, manual.Generation, editedRaw)
	if err != nil {
		t.Fatalf("save manual edit: %v", err)
	}
	if _, err := svc.PreviewTransition(ctx, created.PipelineID, 1, "graph"); err == nil || !errors.As(err, new(*Error)) || err.(*Error).Code != CodeConversionRefused {
		t.Fatalf("edited manual conversion = %v", err)
	}
	still, err := svc.Get(ctx, created.PipelineID, 1)
	if err != nil || still.DocumentJSON != kept.DocumentJSON || still.Mode != "manual" {
		t.Fatal("refused conversion changed the revision")
	}

	restoreReview, err := svc.PreviewTransition(ctx, created.PipelineID, 1, "snapshot")
	if err != nil {
		t.Fatalf("restore preview: %v", err)
	}
	restored, err := svc.RestoreSnapshot(ctx, "config-1", created.RevisionID, still.Generation, restoreReview.Digest)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if restored.Mode != "graph" || restored.DocumentJSON != approvedBytes {
		t.Fatal("restore did not return the snapshotted graph")
	}

	approved, err := svc.Approve(ctx, "admin-1", created.RevisionID, restored.Generation)
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if approved.State != "approved" || approved.DocumentJSON != restored.DocumentJSON {
		t.Fatal("approval changed the document")
	}
	if _, err := svc.Save(ctx, "config-1", created.RevisionID, approved.Generation, raw); err == nil || !errors.As(err, new(*Error)) || err.(*Error).Code != CodeImmutable {
		t.Fatalf("save after approve = %v", err)
	}
	unchanged, err := svc.Get(ctx, created.PipelineID, 1)
	if err != nil || unchanged.DocumentJSON != approved.DocumentJSON || unchanged.State != "approved" {
		t.Fatal("immutable revision was rewritten")
	}

	opened, err := svc.OpenDraft(ctx, "config-1", created.PipelineID, 1)
	if err != nil {
		t.Fatalf("rollback draft: %v", err)
	}
	if opened.Number != 2 || opened.State != "draft" || opened.DocumentJSON != approved.DocumentJSON {
		t.Fatalf("opened = %+v", opened)
	}
	if again, err := svc.Get(ctx, created.PipelineID, 1); err != nil || again.State != "approved" || again.DocumentJSON != approved.DocumentJSON {
		t.Fatal("rollback rewrote the approved revision")
	}

	events, err := store.Audit().List()
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	actions := map[string]int{}
	for _, event := range events {
		if event.TargetID != created.PipelineID {
			continue
		}
		actions[event.Action]++
		if event.Outcome != "succeeded" {
			t.Fatalf("audit outcome = %s", event.Outcome)
		}
	}
	for _, action := range []string{"pipeline.create", "pipeline.save", "pipeline.transition", "pipeline.restore", "pipeline.approve", "pipeline.open-draft"} {
		if actions[action] == 0 {
			t.Fatalf("missing audit %s in %v", action, actions)
		}
	}
}

func TestSaveRaceKeepsOneGeneration(t *testing.T) {
	svc, store := openService(t)
	ctx := context.Background()
	created, err := svc.Create(ctx, "config-1")
	if err != nil {
		t.Fatal(err)
	}
	left := emptyGraph()
	left.Graph.Instances = []pipelinecontract.Instance{{ID: "cpu-1", Kind: pipelinecontract.KindInput, Plugin: "cpu"}}
	right := emptyGraph()
	right.Graph.Instances = []pipelinecontract.Instance{{ID: "mem-1", Kind: pipelinecontract.KindInput, Plugin: "mem"}}
	leftRaw, _ := json.Marshal(left)
	rightRaw, _ := json.Marshal(right)

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, raw := range [][]byte{leftRaw, rightRaw} {
		wg.Add(1)
		go func(body []byte) {
			defer wg.Done()
			_, err := svc.Save(ctx, "config-1", created.RevisionID, created.Generation, body)
			errs <- err
		}(raw)
	}
	wg.Wait()
	close(errs)
	wins := 0
	for err := range errs {
		if err == nil {
			wins++
			continue
		}
		var pipeline *Error
		if !errors.As(err, &pipeline) || pipeline.Code != CodeConflict {
			t.Fatalf("race error = %v", err)
		}
	}
	if wins != 1 {
		t.Fatalf("wins = %d", wins)
	}
	got, err := svc.Get(ctx, created.PipelineID, 1)
	if err != nil || got.Generation != 2 {
		t.Fatalf("generation = %+v %v", got, err)
	}
	events, err := store.Audit().List()
	if err != nil {
		t.Fatal(err)
	}
	saves := 0
	for _, event := range events {
		if event.Action == "pipeline.save" {
			saves++
		}
	}
	if saves != 1 {
		t.Fatalf("save audits = %d", saves)
	}
}

func TestMalformedManualTOMLIsRejectedAtSaveAndApprove(t *testing.T) {
	svc, store := openService(t)
	ctx := context.Background()
	created, err := svc.Create(ctx, "config-1")
	if err != nil {
		t.Fatal(err)
	}
	malformed := pipelinecontract.Document{
		SchemaVersion:   pipelinecontract.SchemaVersion,
		TelegrafVersion: pipelinecontract.SupportedTelegrafVersion,
		Mode:            pipelinecontract.ModeManual,
		Manual:          &pipelinecontract.Manual{TOML: "[[outputs.file]\n  path = \"/tmp/a\"\n"},
	}
	raw, err := json.Marshal(malformed)
	if err != nil {
		t.Fatal(err)
	}

	_, err = svc.Save(ctx, "config-1", created.RevisionID, created.Generation, raw)
	var pipeline *Error
	if !errors.As(err, &pipeline) || pipeline.Code != CodeInvalid {
		t.Fatalf("save error = %v", err)
	}
	if !strings.Contains(pipeline.Message, "syntactically valid") {
		t.Fatalf("save diagnostic = %q", pipeline.Message)
	}
	unchanged, err := svc.Get(ctx, created.PipelineID, 1)
	if err != nil || unchanged.Generation != created.Generation || unchanged.DocumentJSON != created.DocumentJSON {
		t.Fatal("malformed manual save changed the draft")
	}

	// A legacy or damaged draft can bypass Save. Approval must still refuse it
	// rather than freezing malformed TOML as an approved revision.
	pipelineID, err := metadatastore.NewPipelineID()
	if err != nil {
		t.Fatal(err)
	}
	revisionID, err := metadatastore.NewRevisionID()
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := store.Pipelines().InsertDraft(ctx, metadatastore.PipelineRevision{
		RevisionID:   revisionID,
		PipelineID:   pipelineID,
		Number:       1,
		Mode:         string(pipelinecontract.ModeManual),
		DocumentJSON: string(raw),
		CreatedBy:    "legacy",
		Generation:   1,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Approve(ctx, "admin-1", legacy.RevisionID, legacy.Generation)
	if !errors.As(err, &pipeline) || pipeline.Code != CodeInvalid {
		t.Fatalf("approve error = %v", err)
	}
	stillDraft, err := svc.Get(ctx, legacy.PipelineID, 1)
	if err != nil || stillDraft.State != "draft" {
		t.Fatalf("malformed manual draft was approved: %+v %v", stillDraft, err)
	}
}

func TestInvalidSaveDoesNotAudit(t *testing.T) {
	svc, store := openService(t)
	ctx := context.Background()
	created, err := svc.Create(ctx, "config-1")
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Save(ctx, "config-1", created.RevisionID, 1, []byte(`{"mode":"graph"}`))
	if err == nil {
		t.Fatal("expected invalid document")
	}
	got, err := svc.Get(ctx, created.PipelineID, 1)
	if err != nil || got.Generation != 1 || got.DocumentJSON != created.DocumentJSON {
		t.Fatal("invalid save changed the draft")
	}
	events, err := store.Audit().List()
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		if event.Action == "pipeline.save" {
			t.Fatal("invalid save wrote an audit")
		}
	}
}

func openService(t *testing.T) (*Service, *metadatastore.Store) {
	t.Helper()
	store, err := metadatastore.Open(context.Background(), metadatastore.Options{
		Path: filepath.Join(t.TempDir(), "meta.db"),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return New(store.Pipelines()), store
}
