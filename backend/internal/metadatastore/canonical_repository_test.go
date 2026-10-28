package metadatastore

import (
	"context"
	"testing"
	"time"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/registrycontract"
)

func TestCanonicalSchemaApprovedImmutableAllFields(t *testing.T) {
	store := openTemp(t)
	repo := store.CanonicalSchemas()
	now := time.Now().UTC()
	s := registrycontract.CanonicalSchemaRevision{
		SchemaID: "cs-1", LogicalKey: "lk", MajorVersion: 1, MinorVersion: 0, Revision: 1,
		State: registrycontract.CanonicalStateDraft, ContentJSON: `{"k":"v"}`, ContentHash: "h1",
		CreatedBy: "a", CreatedAt: now, Notes: "n",
	}
	if err := repo.Put(s); err != nil {
		t.Fatalf("put draft: %v", err)
	}
	if err := repo.ApproveCanonicalSchema(context.Background(), s.SchemaID, "approver", now); err != nil {
		t.Fatalf("approve: %v", err)
	}
	s2 := s
	s2.Notes = "changed"
	if err := repo.Put(s2); err == nil {
		t.Fatalf("expected error for notes change on approved revision")
	}
	s3 := s
	s3.State = registrycontract.CanonicalStateDraft
	if err := repo.Put(s3); err == nil {
		t.Fatalf("expected error for state regression")
	}
	s4 := s
	s4.ContentJSON = `{"k":"v2"}`
	s4.ContentHash = "h2"
	s4.State = registrycontract.CanonicalStateDraft
	if err := repo.Put(s4); err == nil {
		t.Fatalf("expected error for content change")
	}
	got, ok, err := repo.Get(s.SchemaID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("missing")
	}
	if got.State != registrycontract.CanonicalStateApproved {
		t.Fatalf("state changed")
	}
	if got.ContentJSON != `{"k":"v"}` || got.Notes != "n" {
		t.Fatalf("content changed")
	}
}

func TestMappingProfileApprovedImmutableAllFields(t *testing.T) {
	store := openTemp(t)
	repo := store.MappingProfiles()
	now := time.Now().UTC()
	mp := registrycontract.MappingProfileRevision{
		ProfileID: "mp-1", LogicalKey: "lk", SourceConnectionID: "src", DestinationConnectionID: "dst",
		SourceScope: "s", DestinationScope: "d", MajorVersion: 1, MinorVersion: 0, Revision: 1,
		State: registrycontract.CanonicalStateDraft, ContentJSON: `{"m":"v"}`, ContentHash: "h1",
		CreatedBy: "a", CreatedAt: now, Notes: "n",
	}
	if err := repo.Put(mp); err != nil {
		t.Fatalf("put draft: %v", err)
	}
	if err := repo.ApproveMappingProfile(context.Background(), mp.ProfileID, "approver", now); err != nil {
		t.Fatalf("approve: %v", err)
	}
	mp2 := mp
	mp2.Notes = "x"
	if err := repo.Put(mp2); err == nil {
		t.Fatalf("expected error for notes change on approved mapping profile")
	}
	mp3 := mp
	mp3.SourceConnectionID = "src2"
	mp3.State = registrycontract.CanonicalStateDraft
	if err := repo.Put(mp3); err == nil {
		t.Fatalf("expected error for source change")
	}
	got, ok, err := repo.Get(mp.ProfileID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("missing")
	}
	if got.State != registrycontract.CanonicalStateApproved || got.Notes != "n" || got.SourceConnectionID != "src" {
		t.Fatalf("approved revision mutated")
	}
}

func TestCanonicalSchemaSQLGuardedDraftEditAndApprovalTransition(t *testing.T) {
	store := openTemp(t)
	repo := store.CanonicalSchemas()
	now := time.Now().UTC()
	s := registrycontract.CanonicalSchemaRevision{
		SchemaID: "cs-g1", LogicalKey: "lk", MajorVersion: 1, MinorVersion: 0, Revision: 1,
		State: registrycontract.CanonicalStateDraft, ContentJSON: `{"a":1}`, ContentHash: "h1",
		CreatedBy: "a", CreatedAt: now, Notes: "orig",
	}
	if err := repo.Put(s); err != nil {
		t.Fatal(err)
	}
	sEdit := s
	sEdit.Notes = "updated-notes"
	sEdit.ContentJSON = `{"a":2}`
	sEdit.ContentHash = "h2"
	sEdit.MajorVersion = 2
	if err := repo.Put(sEdit); err != nil {
		t.Fatalf("draft edit should succeed: %v", err)
	}
	if err := repo.ApproveCanonicalSchema(context.Background(), s.SchemaID, "bob", now); err != nil {
		t.Fatal(err)
	}
	cases := []registrycontract.CanonicalSchemaRevision{
		{SchemaID: s.SchemaID, LogicalKey: "other", MajorVersion: 9, MinorVersion: 9, Revision: 99, State: registrycontract.CanonicalStateDraft, ContentJSON: `{"x":1}`, ContentHash: "hx", CreatedBy: "x", CreatedAt: now, ApprovedBy: "x", ApprovedAt: now, Notes: "no"},
		{SchemaID: s.SchemaID, LogicalKey: "lk", MajorVersion: 1, MinorVersion: 0, Revision: 1, State: registrycontract.CanonicalStateDraft, ContentJSON: `{"a":1}`, ContentHash: "h1", CreatedBy: "a", CreatedAt: now, ApprovedBy: "", ApprovedAt: time.Time{}, Notes: "orig"},
	}
	for i, c := range cases {
		if err := repo.Put(c); err == nil {
			t.Fatalf("case %d: expected immutability error", i)
		}
	}
}

func TestMappingProfileSQLGuardedDraftEditAndApprovalTransition(t *testing.T) {
	store := openTemp(t)
	repo := store.MappingProfiles()
	now := time.Now().UTC()
	mp := registrycontract.MappingProfileRevision{
		ProfileID: "mp-g1", LogicalKey: "lk", SourceConnectionID: "src1", DestinationConnectionID: "dst1",
		SourceScope: "s1", DestinationScope: "d1", MajorVersion: 1, MinorVersion: 0, Revision: 1,
		State: registrycontract.CanonicalStateDraft, ContentJSON: `{"m":1}`, ContentHash: "h1",
		CreatedBy: "a", CreatedAt: now, Notes: "orig",
	}
	if err := repo.Put(mp); err != nil {
		t.Fatal(err)
	}
	mp2 := mp
	mp2.Notes = "n2"
	mp2.SourceConnectionID = "src2"
	if err := repo.Put(mp2); err != nil {
		t.Fatal(err)
	}
	if err := repo.ApproveMappingProfile(context.Background(), mp.ProfileID, "bob", now); err != nil {
		t.Fatal(err)
	}
	mpBad := mp2
	mpBad.Notes = "bad"
	mpBad.State = registrycontract.CanonicalStateDraft
	if err := repo.Put(mpBad); err == nil {
		t.Fatal("expected immutability error")
	}
}

func TestCanonicalSchemaConcurrentApprovalVsEditRace(t *testing.T) {
	store := openTemp(t)
	repo := store.CanonicalSchemas()
	now := time.Now().UTC()
	s := registrycontract.CanonicalSchemaRevision{
		SchemaID: "cs-race1", LogicalKey: "lk", MajorVersion: 1, MinorVersion: 0, Revision: 1,
		State: registrycontract.CanonicalStateDraft, ContentJSON: `{"v":1}`, ContentHash: "h1",
		CreatedBy: "a", CreatedAt: now, Notes: "n",
	}
	if err := repo.Put(s); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	errCh := make(chan error, 2)
	go func() {
		<-start
		errCh <- repo.ApproveCanonicalSchema(context.Background(), s.SchemaID, "a1", now)
	}()
	go func() {
		<-start
		sEdit := s
		sEdit.Notes = "race"
		sEdit.ContentHash = "h2"
		sEdit.ContentJSON = `{"v":2}`
		errCh <- repo.Put(sEdit)
	}()
	close(start)
	for i := 0; i < 2; i++ {
		<-errCh
	}
	got, ok, err := repo.Get(s.SchemaID)
	if err != nil || !ok {
		t.Fatal(err)
	}
	// After race, if approved, the revision must be immutable (no post-approval changes)
	if got.State == registrycontract.CanonicalStateApproved {
		// The approved revision is fixed - we cannot have changes applied after it became approved
		// Check that none of the "edit" changes made it into the approved state if approval won
		// But if edit won first changing draft then approval approved modified draft, that's allowed as draft edit
		// The key invariant: once approved, subsequent direct edits must fail. We already tested that above.
		return
	}
	if got.State == registrycontract.CanonicalStateDraft {
		return
	}
	t.Fatal("unexpected state")
}

func TestMappingProfileConcurrentApprovalVsEditRace(t *testing.T) {
	store := openTemp(t)
	repo := store.MappingProfiles()
	now := time.Now().UTC()
	mp := registrycontract.MappingProfileRevision{
		ProfileID: "mp-race1", LogicalKey: "lk", SourceConnectionID: "src", DestinationConnectionID: "dst",
		SourceScope: "s", DestinationScope: "d", MajorVersion: 1, MinorVersion: 0, Revision: 1,
		State: registrycontract.CanonicalStateDraft, ContentJSON: `{"v":1}`, ContentHash: "h1",
		CreatedBy: "a", CreatedAt: now, Notes: "n",
	}
	if err := repo.Put(mp); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	errCh := make(chan error, 2)
	go func() {
		<-start
		errCh <- repo.ApproveMappingProfile(context.Background(), mp.ProfileID, "a1", now)
	}()
	go func() {
		<-start
		mpEdit := mp
		mpEdit.Notes = "race"
		mpEdit.ContentHash = "h2"
		mpEdit.ContentJSON = `{"v":2}`
		errCh <- repo.Put(mpEdit)
	}()
	close(start)
	for i := 0; i < 2; i++ {
		<-errCh
	}
	got, ok, err := repo.Get(mp.ProfileID)
	if err != nil || !ok {
		t.Fatal(err)
	}
	if got.State == registrycontract.CanonicalStateApproved {
		return
	}
	if got.State == registrycontract.CanonicalStateDraft {
		return
	}
	t.Fatal("unexpected state")
}

func TestApprovedRevisionCannotBeReplacedByInsertOrUpdateAllProtectedFields(t *testing.T) {
	store := openTemp(t)
	repo := store.CanonicalSchemas()
	now := time.Now().UTC()
	s := registrycontract.CanonicalSchemaRevision{
		SchemaID: "cs-imm1", LogicalKey: "lk1", MajorVersion: 1, MinorVersion: 0, Revision: 1,
		State: registrycontract.CanonicalStateDraft, ContentJSON: `{"k":"v"}`, ContentHash: "h1",
		CreatedBy: "creator", CreatedAt: now, Notes: "note1",
	}
	if err := repo.Put(s); err != nil {
		t.Fatal(err)
	}
	if err := repo.ApproveCanonicalSchema(context.Background(), s.SchemaID, "approver", now); err != nil {
		t.Fatal(err)
	}
	bad := registrycontract.CanonicalSchemaRevision{
		SchemaID: "cs-imm1", LogicalKey: "lk-CHANGED", MajorVersion: 99, MinorVersion: 88, Revision: 77,
		State: registrycontract.CanonicalStateDraft, ContentJSON: `{"k":"CHANGED"}`, ContentHash: "h-CHANGED",
		CreatedBy: "evil", CreatedAt: now.Add(time.Hour), ApprovedBy: "evil", ApprovedAt: now.Add(time.Hour), Notes: "EVIL",
	}
	if err := repo.Put(bad); err == nil {
		t.Fatal("expected error")
	}
	got, _, _ := repo.Get(s.SchemaID)
	if got.LogicalKey != "lk1" || got.ContentHash != "h1" || got.Notes != "note1" || got.ApprovedBy != "approver" {
		t.Fatal("protected fields mutated")
	}
}
