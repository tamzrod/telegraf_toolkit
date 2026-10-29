package pipelineservice

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/metadatastore"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/pipelinecontract"
)

func buildReview(rev metadatastore.PipelineRevision, target string) (Review, error) {
	doc, err := pipelinecontract.Parse([]byte(rev.DocumentJSON))
	if err != nil {
		return Review{}, invalidDocument(err)
	}
	if target == "snapshot" {
		return reviewSnapshot(rev, doc)
	}
	if target != string(pipelinecontract.ModeGraph) && target != string(pipelinecontract.ModeManual) {
		return Review{}, &Error{Code: CodeInvalid, Message: "The target mode must be graph or manual."}
	}
	if target == rev.Mode {
		return Review{}, &Error{Code: CodeConflict, Message: "The revision is already in that mode."}
	}
	proposed, err := propose(doc, rev.SnapshotJSON, pipelinecontract.Mode(target))
	if err != nil {
		return Review{}, err
	}
	proposedRaw, err := json.Marshal(proposed)
	if err != nil {
		return Review{}, invalidDocument(err)
	}
	currentText, err := exportText(doc)
	if err != nil {
		return Review{}, err
	}
	proposedText, err := exportText(proposed)
	if err != nil {
		return Review{}, err
	}
	return Review{
		FromMode:     rev.Mode,
		ToMode:       string(proposed.Mode),
		CurrentText:  currentText,
		ProposedText: proposedText,
		Proposed:     proposed,
		Digest:       digest(rev.DocumentJSON, string(proposedRaw)),
	}, nil
}

func reviewSnapshot(rev metadatastore.PipelineRevision, current pipelinecontract.Document) (Review, error) {
	if rev.SnapshotJSON == "" {
		return Review{}, &Error{Code: CodeConflict, Message: "This revision has no snapshot to restore."}
	}
	snapshot, err := pipelinecontract.Parse([]byte(rev.SnapshotJSON))
	if err != nil {
		return Review{}, invalidDocument(err)
	}
	if err := pipelinecontract.Validate(snapshot); err != nil {
		return Review{}, invalidDocument(err)
	}
	proposedRaw, err := json.Marshal(snapshot)
	if err != nil {
		return Review{}, invalidDocument(err)
	}
	currentText, err := exportText(current)
	if err != nil {
		return Review{}, err
	}
	proposedText, err := exportText(snapshot)
	if err != nil {
		return Review{}, err
	}
	return Review{
		FromMode:     rev.Mode,
		ToMode:       string(snapshot.Mode),
		CurrentText:  currentText,
		ProposedText: proposedText,
		Proposed:     snapshot,
		Digest:       digest(rev.DocumentJSON, string(proposedRaw)),
	}, nil
}

func propose(doc pipelinecontract.Document, snapshotJSON string, target pipelinecontract.Mode) (pipelinecontract.Document, error) {
	switch target {
	case pipelinecontract.ModeManual:
		if doc.Mode != pipelinecontract.ModeGraph {
			return pipelinecontract.Document{}, &Error{Code: CodeConflict, Message: "Only a graph revision can switch to manual mode."}
		}
		toml, err := pipelinecontract.Generate(doc)
		if err != nil {
			return pipelinecontract.Document{}, &Error{Code: CodeConversionRefused, Message: "The graph cannot be encoded without dropping configuration. It stayed in graph mode."}
		}
		proposed := pipelinecontract.Document{
			SchemaVersion:   doc.SchemaVersion,
			TelegrafVersion: doc.TelegrafVersion,
			Mode:            pipelinecontract.ModeManual,
			Manual:          &pipelinecontract.Manual{TOML: toml},
		}
		if err := pipelinecontract.Validate(proposed); err != nil {
			return pipelinecontract.Document{}, &Error{Code: CodeConversionRefused, Message: "The graph cannot be stored as manual TOML without dropping configuration. It stayed in graph mode."}
		}
		return proposed, nil
	case pipelinecontract.ModeGraph:
		if doc.Mode != pipelinecontract.ModeManual || doc.Manual == nil {
			return pipelinecontract.Document{}, &Error{Code: CodeConflict, Message: "Only a manual revision can switch to graph mode."}
		}
		if snapshotJSON != "" {
			snapshot, err := pipelinecontract.Parse([]byte(snapshotJSON))
			if err == nil && snapshot.Mode == pipelinecontract.ModeGraph {
				if toml, genErr := pipelinecontract.Generate(snapshot); genErr == nil && toml == doc.Manual.TOML {
					return snapshot, nil
				}
			}
		}
		parsed, err := pipelinecontract.ParsePreview(doc.Manual.TOML)
		if err != nil {
			return pipelinecontract.Document{}, &Error{Code: CodeConversionRefused, Message: "Manual TOML cannot be represented as a graph. The manual revision was kept."}
		}
		again, err := pipelinecontract.Generate(parsed)
		if err != nil || again != doc.Manual.TOML {
			return pipelinecontract.Document{}, &Error{Code: CodeConversionRefused, Message: "Manual TOML cannot be represented as a graph. The manual revision was kept."}
		}
		return parsed, nil
	default:
		return pipelinecontract.Document{}, &Error{Code: CodeInvalid, Message: "The target mode must be graph or manual."}
	}
}

func exportText(doc pipelinecontract.Document) (string, error) {
	switch doc.Mode {
	case pipelinecontract.ModeManual:
		if doc.Manual == nil {
			return "", &Error{Code: CodeInvalid, Message: "Manual mode has no TOML body."}
		}
		return doc.Manual.TOML, nil
	case pipelinecontract.ModeGraph:
		toml, err := pipelinecontract.Generate(doc)
		if err != nil {
			return "", &Error{Code: CodeConversionRefused, Message: "The graph cannot be encoded without dropping configuration. It stayed in graph mode."}
		}
		return toml, nil
	default:
		return "", &Error{Code: CodeInvalid, Message: "The revision mode is not supported."}
	}
}

func digest(currentJSON, proposedJSON string) string {
	sum := sha256.Sum256([]byte(currentJSON + "\x00" + proposedJSON))
	return hex.EncodeToString(sum[:])
}

func invalidDocument(err error) error {
	var contract *pipelinecontract.Error
	if errors.As(err, &contract) {
		return &Error{Code: CodeInvalid, Message: contract.Message}
	}
	return &Error{Code: CodeInvalid, Message: "The pipeline document was rejected."}
}

func storeErr(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, metadatastore.ErrPipelineNotFound):
		return &Error{Code: CodeNotFound, Message: "The pipeline revision does not exist."}
	case errors.Is(err, metadatastore.ErrPipelineImmutable):
		return &Error{Code: CodeImmutable, Message: "An approved revision is immutable. Open a new draft to change it."}
	case errors.Is(err, metadatastore.ErrPipelineConflict):
		return &Error{Code: CodeConflict, Message: "The revision changed or already has an open draft."}
	default:
		return err
	}
}
