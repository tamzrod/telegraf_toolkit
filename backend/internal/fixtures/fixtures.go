// Package fixtures provides deterministic, test-only mock data for the
// Toolkit contract surface.
//
// Every value here is synthetic. Nothing in this package contacts a real
// database, Docker daemon, Telegraf runtime or historian. Credential values are
// references only; no secret material is present.
package fixtures

import (
	"errors"
	"fmt"
)

// Family identifies the database product family of a mock profile.
type Family string

// Supported mock database families.
const (
	FamilyInfluxDB Family = "influxdb"
)

// ScopeKind identifies the logical scope shape of a mock profile.
type ScopeKind string

// Mock scope kinds, matching the contract inventory's logical database scope.
const (
	ScopeDatabaseRetentionPolicy ScopeKind = "database-retention-policy" // InfluxDB 1.x
	ScopeOrganizationBucket      ScopeKind = "organization-bucket"       // InfluxDB 2.x
	ScopeDatabase                ScopeKind = "database"                  // InfluxDB 3.x
)

// Capabilities is the probed capability set of a mock profile. Capabilities are
// detected per connection, never inferred from a version label alone.
type Capabilities struct {
	QueryDialects   []string `json:"query_dialects"`
	WriteProtocols  []string `json:"write_protocols"`
	SchemaDiscovery bool     `json:"schema_discovery"`
}

// HasDialect reports whether the capability set includes a query dialect.
func (c Capabilities) HasDialect(dialect string) bool {
	for _, d := range c.QueryDialects {
		if d == dialect {
			return true
		}
	}
	return false
}

// CanWrite reports whether the capability set includes a write protocol.
func (c Capabilities) CanWrite(protocol string) bool {
	for _, p := range c.WriteProtocols {
		if p == protocol {
			return true
		}
	}
	return false
}

// Scope is the logical database scope a mock profile targets.
type Scope struct {
	Kind            ScopeKind `json:"kind"`
	Database        string    `json:"database,omitempty"`
	RetentionPolicy string    `json:"retention_policy,omitempty"`
	Organization    string    `json:"organization,omitempty"`
	Bucket          string    `json:"bucket,omitempty"`
}

// String returns a stable, human-readable scope identifier.
func (s Scope) String() string {
	switch s.Kind {
	case ScopeDatabaseRetentionPolicy:
		return fmt.Sprintf("%s/%s", s.Database, s.RetentionPolicy)
	case ScopeOrganizationBucket:
		return fmt.Sprintf("%s/%s", s.Organization, s.Bucket)
	case ScopeDatabase:
		return s.Database
	default:
		return string(s.Kind)
	}
}

// ConnectionProfile is a mock connection profile. It mirrors the shape of the
// contract inventory without being an implemented wire format.
type ConnectionProfile struct {
	ConnectionID    string       `json:"connection_id"`
	DisplayName     string       `json:"display_name"`
	Family          Family       `json:"family"`
	MajorVersion    int          `json:"major_version"`
	Scope           Scope        `json:"scope"`
	Capabilities    Capabilities `json:"capabilities"`
	CredentialRefID string       `json:"credential_ref_id"`
	Mock            bool         `json:"mock"`
	Note            string       `json:"note"`
}

// ErrSameScopeSelection indicates a source and destination that are not
// distinct, which the contract forbids.
var ErrSameScopeSelection = errors.New("source and destination must be distinct")

// ErrUnsupportedCapability indicates a profile lacks a required capability.
var ErrUnsupportedCapability = errors.New("required capability is not supported")

// ValidateSelection enforces the contract rule that source and destination are
// explicit and distinct.
func ValidateSelection(source, destination ConnectionProfile) error {
	if source.ConnectionID == "" || destination.ConnectionID == "" {
		return errors.New("source and destination must both be selected explicitly")
	}
	if source.ConnectionID == destination.ConnectionID && source.Scope.String() == destination.Scope.String() {
		return fmt.Errorf("%w: %s %s", ErrSameScopeSelection, source.ConnectionID, source.Scope)
	}
	return nil
}

// RequiresReadableSource reports whether the source can serve a read/query.
func RequiresReadableSource(source ConnectionProfile, dialect string) error {
	if !source.Capabilities.HasDialect(dialect) {
		return fmt.Errorf("%w: source %s cannot query dialect %q", ErrUnsupportedCapability, source.ConnectionID, dialect)
	}
	return nil
}

// RequiresWritableDestination reports whether the destination can accept writes.
func RequiresWritableDestination(destination ConnectionProfile, protocol string) error {
	if !destination.Capabilities.CanWrite(protocol) {
		return fmt.Errorf("%w: destination %s cannot write protocol %q", ErrUnsupportedCapability, destination.ConnectionID, protocol)
	}
	return nil
}

// Profile1x returns a deterministic mock InfluxDB 1.x profile.
func Profile1x() ConnectionProfile {
	return ConnectionProfile{
		ConnectionID: "mock-influx-1x",
		DisplayName:  "MOCK InfluxDB 1.x (plant-a)",
		Family:       FamilyInfluxDB,
		MajorVersion: 1,
		Scope: Scope{
			Kind:            ScopeDatabaseRetentionPolicy,
			Database:        "telemetry",
			RetentionPolicy: "autogen",
		},
		Capabilities: Capabilities{
			QueryDialects:   []string{"influxql"},
			WriteProtocols:  []string{"line-protocol"},
			SchemaDiscovery: true,
		},
		CredentialRefID: "mock-cred-readonly-1x",
		Mock:            true,
		Note:            "Test-only mock. No live endpoint.",
	}
}

// Profile2x returns a deterministic mock InfluxDB 2.x profile.
func Profile2x() ConnectionProfile {
	return ConnectionProfile{
		ConnectionID: "mock-influx-2x",
		DisplayName:  "MOCK InfluxDB 2.x (plant-b)",
		Family:       FamilyInfluxDB,
		MajorVersion: 2,
		Scope: Scope{
			Kind:         ScopeOrganizationBucket,
			Organization: "acme",
			Bucket:       "historian",
		},
		Capabilities: Capabilities{
			QueryDialects:   []string{"flux", "influxql"},
			WriteProtocols:  []string{"line-protocol"},
			SchemaDiscovery: true,
		},
		CredentialRefID: "mock-cred-write-2x",
		Mock:            true,
		Note:            "Test-only mock. No live endpoint.",
	}
}

// Profile3x returns a deterministic mock InfluxDB 3.x profile.
func Profile3x() ConnectionProfile {
	return ConnectionProfile{
		ConnectionID: "mock-influx-3x",
		DisplayName:  "MOCK InfluxDB 3.x (plant-c)",
		Family:       FamilyInfluxDB,
		MajorVersion: 3,
		Scope: Scope{
			Kind:     ScopeDatabase,
			Database: "telemetry_v3",
		},
		Capabilities: Capabilities{
			QueryDialects:   []string{"sql", "influxql"},
			WriteProtocols:  []string{"line-protocol"},
			SchemaDiscovery: false,
		},
		CredentialRefID: "mock-cred-readonly-3x",
		Mock:            true,
		Note:            "Test-only mock. Schema discovery unsupported (probed).",
	}
}

// Profile3xSQLOnly returns a mock profile whose probed capabilities omit
// InfluxQL, used to exercise unsupported-capability handling.
func Profile3xSQLOnly() ConnectionProfile {
	p := Profile3x()
	p.ConnectionID = "mock-influx-3x-sql-only"
	p.DisplayName = "MOCK InfluxDB 3.x SQL-only (plant-d)"
	p.Capabilities.QueryDialects = []string{"sql"}
	return p
}

// Profiles returns all mock profiles in a deterministic order.
func Profiles() []ConnectionProfile {
	return []ConnectionProfile{Profile1x(), Profile2x(), Profile3x()}
}
