// Package synccontract encodes the synchronization block state vocabulary.
//
// It exists so that a count comparison can never be reported as verified
// equivalence: COUNT_MATCH is a distinct state from VERIFIED, and the only
// transition into VERIFIED requires an explicit verification policy to have
// been satisfied. See docs/SYNCHRONIZATION.md and docs/contracts/README.md.
package synccontract

import "errors"

// State is a synchronization block state.
type State string

// Block states from the proposed state machine.
const (
	StatePending     State = "PENDING"
	StateScanning    State = "SCANNING"
	StateCountMatch  State = "COUNT_MATCH"
	StateDirty       State = "DIRTY"
	StateMissing     State = "MISSING"
	StateError       State = "ERROR"
	StateQueued      State = "QUEUED"
	StateCopying     State = "COPYING"
	StateVerifying   State = "VERIFYING"
	StateVerified    State = "VERIFIED"
	StateNeedsReview State = "NEEDS_REVIEW"
)

// Policy is the requested verification policy for a block.
type Policy string

// Verification policies, ordered from weakest to strongest.
const (
	PolicyCountOnly       Policy = "count-only"
	PolicyPointLevel      Policy = "point-level"
	PolicyCanonicalDigest Policy = "canonical-digest"
)

// ErrCountMatchIsNotVerified is returned when a count-only result is presented
// as verified equivalence.
var ErrCountMatchIsNotVerified = errors.New("count match is not verified equivalence")

// Satisfies reports whether a state, produced under the given policy, may be
// displayed as verified.
//
// COUNT_MATCH is never verified. VERIFIED is only valid when a policy stronger
// than count-only was requested and applied.
func Satisfies(state State, policy Policy) bool {
	if state == StateCountMatch {
		return false
	}
	if state != StateVerified {
		return false
	}
	return policy == PolicyPointLevel || policy == PolicyCanonicalDigest
}

// Label returns the display label for a state. It deliberately keeps
// COUNT_MATCH visually distinct from VERIFIED.
func Label(state State) string {
	switch state {
	case StateCountMatch:
		return "Count matched (not verified)"
	case StateVerified:
		return "Verified"
	case StateNeedsReview:
		return "Needs review"
	case StateMissing:
		return "Missing"
	case StateDirty:
		return "Differs"
	default:
		return string(state)
	}
}

// ValidateDisplay returns an error when a state/policy pair would misrepresent
// a count match as verified.
func ValidateDisplay(state State, policy Policy) error {
	if state == StateVerified && !Satisfies(state, policy) {
		return ErrCountMatchIsNotVerified
	}
	return nil
}
