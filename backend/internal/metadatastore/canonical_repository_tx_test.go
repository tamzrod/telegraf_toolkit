package metadatastore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/registrycontract"
)

// beginFailDB implements execer and BeginTx. BeginTx always fails so a Put
// must return before any write. Embedding execer would still allow a silent
// non-transactional fallback to persist a row; these tests forbid that.
type beginFailDB struct {
	execer
	err error
}

func (b beginFailDB) BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error) {
	return nil, b.err
}

func draftSchema(id string, now time.Time) registrycontract.CanonicalSchemaRevision {
	return registrycontract.CanonicalSchemaRevision{
		SchemaID: id, LogicalKey: "lk-" + id, MajorVersion: 1, MinorVersion: 0, Revision: 1,
		State: registrycontract.CanonicalStateDraft, ContentJSON: `{"k":"v"}`, ContentHash: "hash-v",
		CreatedBy: "creator", CreatedAt: now, Notes: "notes-v",
	}
}

func draftMapping(id string, now time.Time) registrycontract.MappingProfileRevision {
	return registrycontract.MappingProfileRevision{
		ProfileID: id, LogicalKey: "lk-" + id, CanonicalLogicalKey: "canon-" + id, CanonicalRevision: 1,
		SourceConnectionID: "src", DestinationConnectionID: "dst", SourceScope: "ss", DestinationScope: "ds",
		MajorVersion: 1, MinorVersion: 0, Revision: 1, State: registrycontract.CanonicalStateDraft,
		ContentJSON: `{"m":"v"}`, ContentHash: "hash-m", CreatedBy: "creator", CreatedAt: now, Notes: "notes-m",
	}
}

func TestCanonicalPutOnExternalTxRollsBack(t *testing.T) {
	store := openTemp(t)
	ctx := context.Background()
	schema := draftSchema("cs-ext-rb", time.Now().UTC())
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	repo := &CanonicalSchemaRepository{db: tx}
	if err := repo.Put(schema); err != nil {
		t.Fatalf("put on external tx: %v", err)
	}
	// Put must not commit or roll back the caller's transaction.
	if err := tx.Rollback(); err != nil {
		t.Fatalf("external rollback: %v", err)
	}
	_, ok, err := store.CanonicalSchemas().Get(schema.SchemaID)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("canonical put persisted after external transaction rollback")
	}
}

func TestMappingPutOnExternalTxRollsBack(t *testing.T) {
	store := openTemp(t)
	ctx := context.Background()
	profile := draftMapping("mp-ext-rb", time.Now().UTC())
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	repo := &MappingProfileRepository{db: tx}
	if err := repo.Put(profile); err != nil {
		t.Fatalf("put on external tx: %v", err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatalf("external rollback: %v", err)
	}
	_, ok, err := store.MappingProfiles().Get(profile.ProfileID)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("mapping put persisted after external transaction rollback")
	}
}

func TestCanonicalPutJoinsOuterTransactionWithHistory(t *testing.T) {
	store := openTemp(t)
	ctx := context.Background()
	now := time.Now().UTC()
	schema := draftSchema("cs-outer", now)
	err := store.WithinTransaction(ctx, func(repos registrycontract.Repositories) error {
		if err := repos.CanonicalSchemas.Put(schema); err != nil {
			return err
		}
		return repos.RevisionHistory.Record(registrycontract.RevisionHistoryEntry{
			HistoryID: "h-cs-outer", ResourceType: "canonical_schema", ResourceID: schema.SchemaID,
			LogicalKey: schema.LogicalKey, Revision: schema.Revision, Action: "put", Actor: "tester",
			At: now, DetailJSON: "{}",
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	got, ok, err := store.CanonicalSchemas().Get(schema.SchemaID)
	if err != nil || !ok {
		t.Fatalf("committed schema missing: ok=%v err=%v", ok, err)
	}
	if got.ContentJSON != schema.ContentJSON || got.ContentHash != schema.ContentHash || got.Notes != schema.Notes {
		t.Fatalf("committed schema mismatch: %+v", got)
	}
	hist, err := store.RevisionHistory().ListByResource("canonical_schema", schema.SchemaID)
	if err != nil {
		t.Fatal(err)
	}
	if len(hist) != 1 || hist[0].HistoryID != "h-cs-outer" {
		t.Fatalf("history = %+v", hist)
	}

	edited := schema
	edited.Notes = "should-roll-back"
	edited.ContentJSON = `{"k":"rb"}`
	edited.ContentHash = "hash-rb"
	err = store.WithinTransaction(ctx, func(repos registrycontract.Repositories) error {
		if err := repos.CanonicalSchemas.Put(edited); err != nil {
			return err
		}
		return errors.New("history failed")
	})
	if err == nil || !strings.Contains(err.Error(), "history failed") {
		t.Fatalf("expected history failure, got %v", err)
	}
	got, ok, err = store.CanonicalSchemas().Get(schema.SchemaID)
	if err != nil || !ok {
		t.Fatal(err)
	}
	if got.ContentJSON != schema.ContentJSON || got.ContentHash != schema.ContentHash || got.Notes != schema.Notes {
		t.Fatalf("outer rollback did not preserve canonical content: %+v", got)
	}
}

func TestMappingPutJoinsOuterTransactionWithHistory(t *testing.T) {
	store := openTemp(t)
	ctx := context.Background()
	now := time.Now().UTC()
	profile := draftMapping("mp-outer", now)
	err := store.WithinTransaction(ctx, func(repos registrycontract.Repositories) error {
		if err := repos.MappingProfiles.Put(profile); err != nil {
			return err
		}
		return repos.RevisionHistory.Record(registrycontract.RevisionHistoryEntry{
			HistoryID: "h-mp-outer", ResourceType: "mapping_profile", ResourceID: profile.ProfileID,
			LogicalKey: profile.LogicalKey, Revision: profile.Revision, Action: "put", Actor: "tester",
			At: now, DetailJSON: "{}",
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	got, ok, err := store.MappingProfiles().Get(profile.ProfileID)
	if err != nil || !ok {
		t.Fatalf("committed profile missing: ok=%v err=%v", ok, err)
	}
	if got.ContentJSON != profile.ContentJSON || got.ContentHash != profile.ContentHash || got.Notes != profile.Notes {
		t.Fatalf("committed profile mismatch: %+v", got)
	}

	edited := profile
	edited.Notes = "should-roll-back"
	edited.ContentJSON = `{"m":"rb"}`
	edited.ContentHash = "hash-rb"
	edited.SourceConnectionID = "other-src"
	err = store.WithinTransaction(ctx, func(repos registrycontract.Repositories) error {
		if err := repos.MappingProfiles.Put(edited); err != nil {
			return err
		}
		return errors.New("history failed")
	})
	if err == nil || !strings.Contains(err.Error(), "history failed") {
		t.Fatalf("expected history failure, got %v", err)
	}
	got, ok, err = store.MappingProfiles().Get(profile.ProfileID)
	if err != nil || !ok {
		t.Fatal(err)
	}
	if got.ContentJSON != profile.ContentJSON || got.ContentHash != profile.ContentHash || got.Notes != profile.Notes || got.SourceConnectionID != profile.SourceConnectionID {
		t.Fatalf("outer rollback did not preserve mapping content: %+v", got)
	}
}

func TestCanonicalBeginTxErrorWritesNothing(t *testing.T) {
	store := openTemp(t)
	schema := draftSchema("cs-begin-fail", time.Now().UTC())
	repo := &CanonicalSchemaRepository{db: beginFailDB{execer: store.db, err: errors.New("injected begin failure")}}
	err := repo.Put(schema)
	if err == nil || !strings.Contains(err.Error(), "injected begin failure") {
		t.Fatalf("begin error = %v", err)
	}
	_, ok, getErr := store.CanonicalSchemas().Get(schema.SchemaID)
	if getErr != nil {
		t.Fatal(getErr)
	}
	if ok {
		t.Fatal("canonical put wrote a row after BeginTx failed")
	}
}

func TestMappingBeginTxErrorWritesNothing(t *testing.T) {
	store := openTemp(t)
	profile := draftMapping("mp-begin-fail", time.Now().UTC())
	repo := &MappingProfileRepository{db: beginFailDB{execer: store.db, err: errors.New("injected begin failure")}}
	err := repo.Put(profile)
	if err == nil || !strings.Contains(err.Error(), "injected begin failure") {
		t.Fatalf("begin error = %v", err)
	}
	_, ok, getErr := store.MappingProfiles().Get(profile.ProfileID)
	if getErr != nil {
		t.Fatal(getErr)
	}
	if ok {
		t.Fatal("mapping put wrote a row after BeginTx failed")
	}
}

func TestCanonicalEditBeforeApprovalAndRejectsLaterEdits(t *testing.T) {
	store := openTemp(t)
	repo := store.CanonicalSchemas()
	now := time.Now().UTC()
	original := draftSchema("cs-seq", now)
	if err := repo.Put(original); err != nil {
		t.Fatal(err)
	}
	edited := original
	edited.Notes = "edited-notes"
	edited.ContentJSON = `{"k":"edited"}`
	edited.ContentHash = "hash-edited"
	edited.MajorVersion = 2
	if err := repo.Put(edited); err != nil {
		t.Fatalf("draft edit: %v", err)
	}
	if err := repo.ApproveCanonicalSchema(context.Background(), original.SchemaID, "approver", now); err != nil {
		t.Fatal(err)
	}
	approved, ok, err := repo.Get(original.SchemaID)
	if err != nil || !ok {
		t.Fatal(err)
	}
	if approved.State != registrycontract.CanonicalStateApproved ||
		approved.ContentJSON != edited.ContentJSON ||
		approved.ContentHash != edited.ContentHash ||
		approved.Notes != edited.Notes ||
		approved.MajorVersion != edited.MajorVersion ||
		approved.LogicalKey != original.LogicalKey ||
		approved.ApprovedBy != "approver" {
		t.Fatalf("edit-then-approve persisted %+v", approved)
	}

	// An unguarded upsert after approval would change these fields and fail this check.
	bad := approved
	bad.State = registrycontract.CanonicalStateDraft
	bad.Notes = "tampered-notes"
	bad.ContentJSON = `{"k":"tampered"}`
	bad.ContentHash = "hash-tampered"
	bad.LogicalKey = "tampered-key"
	bad.MajorVersion = 9
	bad.MinorVersion = 8
	bad.Revision = 7
	bad.CreatedBy = "tamperer"
	bad.ApprovedBy = "tamperer"
	if err := repo.Put(bad); err == nil {
		t.Fatal("put after approval succeeded")
	}
	after, ok, err := repo.Get(original.SchemaID)
	if err != nil || !ok {
		t.Fatal(err)
	}
	if after.State != approved.State ||
		after.ContentJSON != approved.ContentJSON ||
		after.ContentHash != approved.ContentHash ||
		after.Notes != approved.Notes ||
		after.LogicalKey != approved.LogicalKey ||
		after.MajorVersion != approved.MajorVersion ||
		after.MinorVersion != approved.MinorVersion ||
		after.Revision != approved.Revision ||
		after.CreatedBy != approved.CreatedBy ||
		after.ApprovedBy != approved.ApprovedBy ||
		after.SchemaID != approved.SchemaID {
		t.Fatalf("approved canonical revision changed: before=%+v after=%+v", approved, after)
	}
}

func legalSchemaTriple(got, base registrycontract.CanonicalSchemaRevision) bool {
	if got.LogicalKey != base.LogicalKey || got.SchemaID != base.SchemaID {
		return false
	}
	if got.ContentJSON == base.ContentJSON && got.ContentHash == base.ContentHash && got.Notes == base.Notes {
		return true
	}
	// Interleaving tests write notes, hash and JSON from the same iteration index.
	var n int
	if _, err := fmt.Sscanf(got.Notes, "schema-notes-%d", &n); err != nil {
		return false
	}
	return got.Notes == fmt.Sprintf("schema-notes-%d", n) && got.ContentHash == fmt.Sprintf("h-sch-%d", n) && got.ContentJSON == fmt.Sprintf(`{"iter":%d}`, n)
}

func legalMappingTriple(got, base registrycontract.MappingProfileRevision) bool {
	if got.LogicalKey != base.LogicalKey || got.ProfileID != base.ProfileID || got.SourceConnectionID != base.SourceConnectionID {
		return false
	}
	if got.ContentJSON == base.ContentJSON && got.ContentHash == base.ContentHash && got.Notes == base.Notes {
		return true
	}
	var n int
	if _, err := fmt.Sscanf(got.Notes, "mapper-notes-%d", &n); err != nil {
		return false
	}
	return got.Notes == fmt.Sprintf("mapper-notes-%d", n) && got.ContentHash == fmt.Sprintf("h-mp-%d", n) && got.ContentJSON == fmt.Sprintf(`{"iter":%d}`, n)
}

func assertCanonicalRaceSnapshot(t *testing.T, repo *CanonicalSchemaRepository, got, base registrycontract.CanonicalSchemaRevision, editNotes, editHash, editJSON string) {
	t.Helper()
	original := got.ContentJSON == base.ContentJSON && got.ContentHash == base.ContentHash && got.Notes == base.Notes
	edited := got.ContentJSON == editJSON && got.ContentHash == editHash && got.Notes == editNotes
	if !original && !edited {
		t.Fatalf("canonical content/hash/notes diverged: %+v", got)
	}
	if got.LogicalKey != base.LogicalKey || got.SchemaID != base.SchemaID || got.CreatedBy != base.CreatedBy {
		t.Fatalf("canonical identity changed: %+v", got)
	}
	if got.State != registrycontract.CanonicalStateApproved && got.State != registrycontract.CanonicalStateDraft {
		t.Fatalf("unexpected canonical state %s", got.State)
	}
	if got.State != registrycontract.CanonicalStateApproved {
		return
	}
	tamper := got
	tamper.State = registrycontract.CanonicalStateDraft
	tamper.Notes = "tamper"
	tamper.ContentHash = "tamper"
	tamper.ContentJSON = `{"tamper":true}`
	if err := repo.Put(tamper); err == nil {
		t.Fatal("edit after approval succeeded")
	}
	after, ok, err := repo.Get(base.SchemaID)
	if err != nil || !ok {
		t.Fatal(err)
	}
	if after.State != got.State || after.ContentJSON != got.ContentJSON || after.ContentHash != got.ContentHash || after.Notes != got.Notes {
		t.Fatalf("approved canonical revision changed after rejected edit: %+v", after)
	}
}

func assertMappingRaceSnapshot(t *testing.T, repo *MappingProfileRepository, got, base registrycontract.MappingProfileRevision, editNotes, editHash, editJSON string) {
	t.Helper()
	original := got.ContentJSON == base.ContentJSON && got.ContentHash == base.ContentHash && got.Notes == base.Notes
	edited := got.ContentJSON == editJSON && got.ContentHash == editHash && got.Notes == editNotes
	if !original && !edited {
		t.Fatalf("mapping content/hash/notes diverged: %+v", got)
	}
	if got.LogicalKey != base.LogicalKey || got.ProfileID != base.ProfileID || got.SourceConnectionID != base.SourceConnectionID || got.DestinationConnectionID != base.DestinationConnectionID {
		t.Fatalf("mapping identity changed: %+v", got)
	}
	if got.State != registrycontract.CanonicalStateApproved && got.State != registrycontract.CanonicalStateDraft {
		t.Fatalf("unexpected mapping state %s", got.State)
	}
	if got.State != registrycontract.CanonicalStateApproved {
		return
	}
	tamper := got
	tamper.State = registrycontract.CanonicalStateDraft
	tamper.Notes = "tamper"
	tamper.ContentHash = "tamper"
	tamper.ContentJSON = `{"tamper":true}`
	tamper.SourceConnectionID = "tamper"
	if err := repo.Put(tamper); err == nil {
		t.Fatal("edit after approval succeeded")
	}
	after, ok, err := repo.Get(base.ProfileID)
	if err != nil || !ok {
		t.Fatal(err)
	}
	if after.State != got.State || after.ContentJSON != got.ContentJSON || after.ContentHash != got.ContentHash || after.Notes != got.Notes || after.SourceConnectionID != got.SourceConnectionID {
		t.Fatalf("approved mapping revision changed after rejected edit: %+v", after)
	}
}

func TestMappingEditBeforeApprovalAndRejectsLaterEdits(t *testing.T) {
	store := openTemp(t)
	repo := store.MappingProfiles()
	now := time.Now().UTC()
	original := draftMapping("mp-seq", now)
	if err := repo.Put(original); err != nil {
		t.Fatal(err)
	}
	edited := original
	edited.Notes = "edited-notes"
	edited.ContentJSON = `{"m":"edited"}`
	edited.ContentHash = "hash-edited"
	edited.SourceConnectionID = "src-edited"
	edited.DestinationScope = "ds-edited"
	if err := repo.Put(edited); err != nil {
		t.Fatalf("draft edit: %v", err)
	}
	if err := repo.ApproveMappingProfile(context.Background(), original.ProfileID, "approver", now); err != nil {
		t.Fatal(err)
	}
	approved, ok, err := repo.Get(original.ProfileID)
	if err != nil || !ok {
		t.Fatal(err)
	}
	if approved.State != registrycontract.CanonicalStateApproved ||
		approved.ContentJSON != edited.ContentJSON ||
		approved.ContentHash != edited.ContentHash ||
		approved.Notes != edited.Notes ||
		approved.SourceConnectionID != edited.SourceConnectionID ||
		approved.DestinationScope != edited.DestinationScope ||
		approved.LogicalKey != original.LogicalKey ||
		approved.ApprovedBy != "approver" {
		t.Fatalf("edit-then-approve persisted %+v", approved)
	}

	bad := approved
	bad.State = registrycontract.CanonicalStateDraft
	bad.Notes = "tampered-notes"
	bad.ContentJSON = `{"m":"tampered"}`
	bad.ContentHash = "hash-tampered"
	bad.SourceConnectionID = "tampered-src"
	bad.DestinationConnectionID = "tampered-dst"
	bad.SourceScope = "tampered-ss"
	bad.DestinationScope = "tampered-ds"
	bad.LogicalKey = "tampered-key"
	bad.CanonicalLogicalKey = "tampered-canon"
	if err := repo.Put(bad); err == nil {
		t.Fatal("put after approval succeeded")
	}
	after, ok, err := repo.Get(original.ProfileID)
	if err != nil || !ok {
		t.Fatal(err)
	}
	if after.State != approved.State ||
		after.ContentJSON != approved.ContentJSON ||
		after.ContentHash != approved.ContentHash ||
		after.Notes != approved.Notes ||
		after.SourceConnectionID != approved.SourceConnectionID ||
		after.DestinationConnectionID != approved.DestinationConnectionID ||
		after.SourceScope != approved.SourceScope ||
		after.DestinationScope != approved.DestinationScope ||
		after.LogicalKey != approved.LogicalKey ||
		after.CanonicalLogicalKey != approved.CanonicalLogicalKey ||
		after.ApprovedBy != approved.ApprovedBy ||
		after.ProfileID != approved.ProfileID {
		t.Fatalf("approved mapping revision changed: before=%+v after=%+v", approved, after)
	}
}
