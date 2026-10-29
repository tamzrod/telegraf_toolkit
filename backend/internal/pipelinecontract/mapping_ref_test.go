package pipelinecontract

import (
	"strings"
	"testing"
)

func TestUnresolvedMappingIsNotEncoded(t *testing.T) {
	doc := Document{
		SchemaVersion:   SchemaVersion,
		TelegrafVersion: SupportedTelegrafVersion,
		Mode:            ModeGraph,
		Graph: &Graph{
			Instances:                       []Instance{},
			ProcessorOrder:                  []string{},
			SkipProcessorsBeforeAggregators: BoolPtr(false),
			SkipProcessorsAfterAggregators:  BoolPtr(false),
			Mapping: &MappingRef{
				ProfileID:   "mp-1",
				Revision:    1,
				ContentHash: "hash-1",
			},
		},
	}
	if err := Validate(doc); err != nil {
		t.Fatalf("validate: %v", err)
	}
	_, err := Generate(doc)
	if err == nil || !strings.Contains(err.Error(), CodeMappingUnresolved) {
		t.Fatalf("generate err = %v", err)
	}
	if _, err := ExecutionPlan(doc); err == nil || !strings.Contains(err.Error(), CodeMappingUnresolved) {
		t.Fatalf("plan err = %v", err)
	}
	if strings.Contains(err.Error(), "mp-1") || strings.Contains(err.Error(), "hash-1") {
		t.Fatalf("unresolved mapping error quoted the pin: %v", err)
	}
}
