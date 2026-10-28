package metadatastore

import (
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
		State: registrycontract.CanonicalStateApproved, ContentJSON: `{"k":"v"}`, ContentHash: "h1",
		CreatedBy: "a", CreatedAt: now, ApprovedBy: "a", ApprovedAt: now, Notes: "n",
	}
	if err := repo.Put(s); err != nil {
		t.Fatalf("put: %v", err)
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
	if err := repo.Put(s4); err == nil {
		t.Fatalf("expected error for content change")
	}
}

func TestMappingProfileApprovedImmutableAllFields(t *testing.T) {
	store := openTemp(t)
	repo := store.MappingProfiles()
	now := time.Now().UTC()
	mp := registrycontract.MappingProfileRevision{
		ProfileID: "mp-1", LogicalKey: "lk", SourceConnectionID: "src", DestinationConnectionID: "dst",
		SourceScope: "s", DestinationScope: "d", MajorVersion: 1, MinorVersion: 0, Revision: 1,
		State: registrycontract.CanonicalStateApproved, ContentJSON: `{"m":"v"}`, ContentHash: "h1",
		CreatedBy: "a", CreatedAt: now, ApprovedBy: "a", ApprovedAt: now, Notes: "n",
	}
	if err := repo.Put(mp); err != nil {
		t.Fatalf("put: %v", err)
	}
	mp2 := mp
	mp2.Notes = "x"
	if err := repo.Put(mp2); err == nil {
		t.Fatalf("expected error for notes change on approved mapping profile")
	}
	mp3 := mp
	mp3.SourceConnectionID = "src2"
	if err := repo.Put(mp3); err == nil {
		t.Fatalf("expected error for source change")
	}
}
