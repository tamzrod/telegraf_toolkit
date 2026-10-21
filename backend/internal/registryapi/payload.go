package registryapi

import (
	"encoding/json"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/fixtures"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/registrycontract"
)

// scopePayload is the wire shape of a logical scope.
type scopePayload struct {
	Kind            string `json:"kind"`
	Database        string `json:"database,omitempty"`
	RetentionPolicy string `json:"retention_policy,omitempty"`
	Organization    string `json:"organization,omitempty"`
	Bucket          string `json:"bucket,omitempty"`
}

// capabilitiesPayload is the wire shape of probed capabilities.
type capabilitiesPayload struct {
	QueryDialects   []string `json:"query_dialects"`
	WriteProtocols  []string `json:"write_protocols"`
	SchemaDiscovery bool     `json:"schema_discovery"`
}

// profilePayload is the wire shape of a profile. It carries a credential
// reference ID, never a value.
type profilePayload struct {
	ConnectionID    string              `json:"connection_id"`
	DisplayName     string              `json:"display_name"`
	Family          string              `json:"family"`
	MajorVersion    int                 `json:"major_version"`
	Endpoint        string              `json:"endpoint"`
	Scope           scopePayload        `json:"scope"`
	Capabilities    capabilitiesPayload `json:"capabilities"`
	CredentialRefID string              `json:"credential_ref_id"`
	Mock            bool                `json:"mock"`
	Note            string              `json:"note"`
}

// credentialRefPayload is the wire shape of a credential reference. It has no
// value-bearing field.
type credentialRefPayload struct {
	CredentialRefID string `json:"credential_ref_id"`
	Alias           string `json:"alias"`
	Role            string `json:"role"`
	Fingerprint     string `json:"fingerprint,omitempty"`
	LastRotated     string `json:"last_rotated,omitempty"`
	ExpiresAt       string `json:"expires_at,omitempty"`
	Mock            bool   `json:"mock"`
}

// auditPayloadEvent is the wire shape of an audit event.
type auditPayloadEvent struct {
	Actor         string `json:"actor"`
	Action        string `json:"action"`
	TargetKind    string `json:"target_kind"`
	TargetID      string `json:"target_id"`
	Outcome       string `json:"outcome"`
	At            string `json:"at"`
	CorrelationID string `json:"correlation_id,omitempty"`
	Mock          bool   `json:"mock"`
}

// profileInput is the accepted body for creating a profile. It uses the same
// shape as the response and is validated through the contract before storage.
type profileInput struct {
	ConnectionID    string              `json:"connection_id"`
	DisplayName     string              `json:"display_name"`
	Family          string              `json:"family"`
	MajorVersion    int                 `json:"major_version"`
	Endpoint        string              `json:"endpoint"`
	Scope           scopePayload        `json:"scope"`
	Capabilities    capabilitiesPayload `json:"capabilities"`
	CredentialRefID string              `json:"credential_ref_id"`
	Mock            bool                `json:"mock"`
	Note            string              `json:"note"`
}

// toProfile converts and validates an input body. Validation runs through the
// contract so an invalid scope, missing reference or secret-bearing field is
// rejected with a sanitized error before any persistence occurs.
func (in profileInput) toProfile() (fixtures.ConnectionProfile, error) {
	profile := fixtures.ConnectionProfile{
		ConnectionID: in.ConnectionID,
		DisplayName:  in.DisplayName,
		Family:       fixtures.Family(in.Family),
		MajorVersion: in.MajorVersion,
		Endpoint:     in.Endpoint,
		Scope: fixtures.Scope{
			Kind:            fixtures.ScopeKind(in.Scope.Kind),
			Database:        in.Scope.Database,
			RetentionPolicy: in.Scope.RetentionPolicy,
			Organization:    in.Scope.Organization,
			Bucket:          in.Scope.Bucket,
		},
		Capabilities: fixtures.Capabilities{
			QueryDialects:   append([]string(nil), in.Capabilities.QueryDialects...),
			WriteProtocols:  append([]string(nil), in.Capabilities.WriteProtocols...),
			SchemaDiscovery: in.Capabilities.SchemaDiscovery,
		},
		CredentialRefID: in.CredentialRefID,
		Mock:            in.Mock,
		Note:            in.Note,
	}
	if err := registrycontract.ValidateProfileInput(profile); err != nil {
		return fixtures.ConnectionProfile{}, err
	}
	return profile, nil
}

func profilesPayload(profiles []fixtures.ConnectionProfile) []profilePayload {
	out := make([]profilePayload, 0, len(profiles))
	for _, profile := range profiles {
		out = append(out, profileToPayload(profile))
	}
	return out
}

func profileToPayload(profile fixtures.ConnectionProfile) profilePayload {
	return profilePayload{
		ConnectionID: profile.ConnectionID,
		DisplayName:  profile.DisplayName,
		Family:       string(profile.Family),
		MajorVersion: profile.MajorVersion,
		Endpoint:     profile.Endpoint,
		Scope: scopePayload{
			Kind:            string(profile.Scope.Kind),
			Database:        profile.Scope.Database,
			RetentionPolicy: profile.Scope.RetentionPolicy,
			Organization:    profile.Scope.Organization,
			Bucket:          profile.Scope.Bucket,
		},
		Capabilities: capabilitiesPayload{
			QueryDialects:   nonNilStrings(profile.Capabilities.QueryDialects),
			WriteProtocols:  nonNilStrings(profile.Capabilities.WriteProtocols),
			SchemaDiscovery: profile.Capabilities.SchemaDiscovery,
		},
		CredentialRefID: profile.CredentialRefID,
		Mock:            profile.Mock,
		Note:            profile.Note,
	}
}

// credentialRefResponse is the wire shape of a single credential reference. It
// carries the reference, the alias, the role and the fingerprint - never a
// credential value.
func credentialRefResponse(ref registrycontract.CredentialRef) credentialRefPayload {
	payload := credentialRefPayload{
		CredentialRefID: ref.CredentialRefID,
		Alias:           ref.Alias,
		Role:            string(ref.Role),
		Fingerprint:     ref.Fingerprint,
		Mock:            ref.Mock,
	}
	if !ref.LastRotated.IsZero() {
		payload.LastRotated = ref.LastRotated.UTC().Format("2006-01-02T15:04:05Z07:00")
	}
	if !ref.ExpiresAt.IsZero() {
		payload.ExpiresAt = ref.ExpiresAt.UTC().Format("2006-01-02T15:04:05Z07:00")
	}
	return payload
}

func credentialRefsPayload(refs []registrycontract.CredentialRef) []credentialRefPayload {
	out := make([]credentialRefPayload, 0, len(refs))
	for _, ref := range refs {
		payload := credentialRefPayload{
			CredentialRefID: ref.CredentialRefID,
			Alias:           ref.Alias,
			Role:            string(ref.Role),
			Fingerprint:     ref.Fingerprint,
			Mock:            ref.Mock,
		}
		if !ref.LastRotated.IsZero() {
			payload.LastRotated = ref.LastRotated.UTC().Format("2006-01-02T15:04:05Z07:00")
		}
		if !ref.ExpiresAt.IsZero() {
			payload.ExpiresAt = ref.ExpiresAt.UTC().Format("2006-01-02T15:04:05Z07:00")
		}
		out = append(out, payload)
	}
	return out
}

func auditPayload(events []registrycontract.AuditEvent) []auditPayloadEvent {
	out := make([]auditPayloadEvent, 0, len(events))
	for _, event := range events {
		out = append(out, auditPayloadEvent{
			Actor:         event.Actor,
			Action:        event.Action,
			TargetKind:    string(event.TargetKind),
			TargetID:      event.TargetID,
			Outcome:       string(event.Outcome),
			At:            event.At.UTC().Format("2006-01-02T15:04:05Z07:00"),
			CorrelationID: event.CorrelationID,
			Mock:          event.Mock,
		})
	}
	return out
}

func nonNilStrings(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}

// containsSecretLikeKey reports whether a raw JSON object carries a secret-like
// key anywhere. It is a defensive check applied to a request body before
// decoding, so a payload that attempts to smuggle a credential value is refused
// rather than partially parsed.
func containsSecretLikeKey(raw []byte) bool {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return false
	}
	return scanValue(value)
}

func scanValue(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		for key, nested := range typed {
			if secretLikeKey(key) {
				return true
			}
			if scanValue(nested) {
				return true
			}
		}
	case []any:
		for _, nested := range typed {
			if scanValue(nested) {
				return true
			}
		}
	}
	return false
}

func secretLikeKey(key string) bool {
	switch key {
	case "value", "password", "passwd", "token", "apikey", "api_key", "secret":
		return true
	default:
		return false
	}
}
