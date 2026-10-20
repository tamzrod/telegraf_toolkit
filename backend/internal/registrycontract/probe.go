package registrycontract

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/fixtures"
)

// ProbeOutcome is the three-valued result of a capability check. "unknown" is
// deliberately distinct from "unsupported": a probe that could not complete
// must not be reported as a capability absence.
type ProbeOutcome string

// Probe outcomes.
const (
	OutcomeSupported   ProbeOutcome = "supported"
	OutcomeUnsupported ProbeOutcome = "unsupported"
	OutcomeUnknown     ProbeOutcome = "unknown"
)

// Edition identifies a concrete InfluxDB product edition/deployment. Dialect and
// write support is a property of the edition, not of the major version alone.
type Edition string

// Known editions used by the compatibility matrix.
const (
	EditionInflux1xOSS         Edition = "influxdb-1.x-oss"
	EditionInflux1xEnterprise  Edition = "influxdb-1.x-enterprise"
	EditionInflux2xOSS         Edition = "influxdb-2.x-oss"
	EditionInflux2xCloud       Edition = "influxdb-2.x-cloud"
	EditionInflux3xCore        Edition = "influxdb-3.x-core"
	EditionInflux3xCloudServer Edition = "influxdb-3.x-cloud-serverless"
)

// TLSExpectation records the transport policy a probe must honour. Verification
// is required by default; relaxing it is an explicit exception.
type TLSExpectation struct {
	Verify        bool   `json:"verify"`
	ServerName    string `json:"server_name,omitempty"`
	ExceptionNote string `json:"exception_note,omitempty"`
}

// DefaultTLSExpectation returns the TLS-by-default expectation for a host.
func DefaultTLSExpectation(serverName string) TLSExpectation {
	return TLSExpectation{Verify: true, ServerName: serverName}
}

// ProbeRequest is a capability-probe request. A positive deadline is required
// so a probe cannot hang a caller indefinitely.
type ProbeRequest struct {
	ConnectionID string         `json:"connection_id"`
	Edition      Edition        `json:"edition"`
	Deadline     time.Duration  `json:"deadline"`
	TLS          TLSExpectation `json:"tls"`
}

// Validate enforces the request invariants.
func (r ProbeRequest) Validate() error {
	if r.ConnectionID == "" {
		return NewSanitizedError(CodeInvalidProfile, "A probe requires a connection_id.")
	}
	if r.Deadline <= 0 {
		return NewSanitizedError(CodeInvalidProfile, "A probe requires a positive deadline.")
	}
	if r.TLS.Verify && r.TLS.ServerName == "" {
		return NewSanitizedError(CodeTLSValidationFailed, "TLS verification requires a server name.")
	}
	if !r.TLS.Verify && r.TLS.ExceptionNote == "" {
		return NewSanitizedError(CodeTLSValidationFailed, "Disabling TLS verification requires an explicit exception note.")
	}
	return nil
}

// CapabilityReport is the result of a probe. Mock reports are always flagged so
// a synthetic result can never be read as a live integration result.
type CapabilityReport struct {
	ConnectionID    string                  `json:"connection_id"`
	Edition         Edition                 `json:"edition"`
	ProbedAt        time.Time               `json:"probed_at"`
	QueryDialects   map[string]ProbeOutcome `json:"query_dialects"`
	WriteProtocols  map[string]ProbeOutcome `json:"write_protocols"`
	SchemaDiscovery ProbeOutcome            `json:"schema_discovery"`
	TLS             TLSExpectation          `json:"tls"`
	Deadline        time.Duration           `json:"deadline"`
	Mock            bool                    `json:"mock"`
	Error           *SanitizedError         `json:"error,omitempty"`
}

// SupportsDialect reports whether the report positively confirms a dialect.
// An unknown outcome is never treated as support.
func (r CapabilityReport) SupportsDialect(dialect string) bool {
	return r.QueryDialects[dialect] == OutcomeSupported
}

// CanWrite reports whether the report positively confirms a write protocol.
func (r CapabilityReport) CanWrite(protocol string) bool {
	return r.WriteProtocols[protocol] == OutcomeSupported
}

// Validate enforces report invariants, including that a mock report carries no
// secret material and that its outcomes are within the known set.
func (r CapabilityReport) Validate() error {
	if r.ConnectionID == "" {
		return NewSanitizedError(CodeInvalidProfile, "A report requires a connection_id.")
	}
	for _, outcome := range allOutcomes(r) {
		switch outcome {
		case OutcomeSupported, OutcomeUnsupported, OutcomeUnknown:
		default:
			return NewSanitizedError(CodeInvalidProfile, fmt.Sprintf("Unknown probe outcome %q.", outcome))
		}
	}
	if r.Error != nil {
		if err := r.Error.Validate(); err != nil {
			return NewSanitizedError(CodeSecretMaterial, "Probe error must not contain sensitive material.")
		}
	}
	return nil
}

func allOutcomes(r CapabilityReport) []ProbeOutcome {
	out := []ProbeOutcome{r.SchemaDiscovery}
	for _, o := range r.QueryDialects {
		out = append(out, o)
	}
	for _, o := range r.WriteProtocols {
		out = append(out, o)
	}
	return out
}

// probeTimeoutError returns a sanitized error for a probe that exceeded its
// deadline. It carries no endpoint credentials.
func probeTimeoutError(connectionID string) error {
	return NewSanitizedError(CodeEndpointRejected,
		fmt.Sprintf("Capability probe for %q exceeded its deadline.", connectionID))
}

// MatrixEntry is the expected capability profile for one edition. It encodes
// variance between editions so tests can prove capabilities are not inferred
// from a major version alone.
type MatrixEntry struct {
	Edition         Edition
	MajorVersion    int
	QueryDialects   map[string]ProbeOutcome
	WriteProtocols  map[string]ProbeOutcome
	SchemaDiscovery ProbeOutcome
	Note            string
}

// CompatibilityMatrix returns the isolated, mock-only edition matrix. Every
// value is an expectation used by tests, not a live probe result.
func CompatibilityMatrix() []MatrixEntry {
	return []MatrixEntry{
		{
			Edition:         EditionInflux1xOSS,
			MajorVersion:    1,
			QueryDialects:   map[string]ProbeOutcome{"influxql": OutcomeSupported, "flux": OutcomeUnsupported, "sql": OutcomeUnsupported},
			WriteProtocols:  map[string]ProbeOutcome{"line-protocol": OutcomeSupported},
			SchemaDiscovery: OutcomeSupported,
			Note:            "1.x OSS: InfluxQL only; username/password auth.",
		},
		{
			Edition:         EditionInflux1xEnterprise,
			MajorVersion:    1,
			QueryDialects:   map[string]ProbeOutcome{"influxql": OutcomeSupported, "flux": OutcomeUnsupported, "sql": OutcomeUnsupported},
			WriteProtocols:  map[string]ProbeOutcome{"line-protocol": OutcomeSupported},
			SchemaDiscovery: OutcomeSupported,
			Note:            "1.x Enterprise: same dialect surface as OSS in this fixture.",
		},
		{
			Edition:         EditionInflux2xOSS,
			MajorVersion:    2,
			QueryDialects:   map[string]ProbeOutcome{"influxql": OutcomeSupported, "flux": OutcomeSupported, "sql": OutcomeUnsupported},
			WriteProtocols:  map[string]ProbeOutcome{"line-protocol": OutcomeSupported},
			SchemaDiscovery: OutcomeSupported,
			Note:            "2.x OSS: Flux + InfluxQL; token auth.",
		},
		{
			Edition:         EditionInflux2xCloud,
			MajorVersion:    2,
			QueryDialects:   map[string]ProbeOutcome{"influxql": OutcomeUnknown, "flux": OutcomeSupported, "sql": OutcomeUnsupported},
			WriteProtocols:  map[string]ProbeOutcome{"line-protocol": OutcomeSupported},
			SchemaDiscovery: OutcomeUnknown,
			Note:            "2.x Cloud: InfluxQL and schema discovery reported unknown, not assumed.",
		},
		{
			Edition:         EditionInflux3xCore,
			MajorVersion:    3,
			QueryDialects:   map[string]ProbeOutcome{"influxql": OutcomeSupported, "flux": OutcomeUnsupported, "sql": OutcomeSupported},
			WriteProtocols:  map[string]ProbeOutcome{"line-protocol": OutcomeSupported},
			SchemaDiscovery: OutcomeUnsupported,
			Note:            "3.x Core: SQL + InfluxQL; schema discovery unsupported in this fixture.",
		},
		{
			Edition:         EditionInflux3xCloudServer,
			MajorVersion:    3,
			QueryDialects:   map[string]ProbeOutcome{"influxql": OutcomeUnsupported, "flux": OutcomeUnsupported, "sql": OutcomeSupported},
			WriteProtocols:  map[string]ProbeOutcome{"line-protocol": OutcomeSupported},
			SchemaDiscovery: OutcomeUnsupported,
			Note:            "3.x Cloud Serverless: SQL only; InfluxQL reported unsupported.",
		},
	}
}

// MatrixEntryFor returns the matrix entry for an edition.
func MatrixEntryFor(edition Edition) (MatrixEntry, bool) {
	for _, entry := range CompatibilityMatrix() {
		if entry.Edition == edition {
			return entry, true
		}
	}
	return MatrixEntry{}, false
}

// EditionProber is a mock-only capability prober driven by the edition matrix.
// It performs no network call and always marks its reports mock.
type EditionProber struct {
	now func() time.Time
}

// NewEditionProber returns a prober with a deterministic clock injected for
// tests. Passing nil uses time.Now.
func NewEditionProber(now func() time.Time) *EditionProber {
	if now == nil {
		now = time.Now
	}
	return &EditionProber{now: now}
}

// Probe returns a mock capability report for the requested edition, honouring
// the deadline and TLS expectations. An unknown edition yields a sanitized
// error rather than a guessed capability set.
func (p *EditionProber) Probe(ctx context.Context, req ProbeRequest) (CapabilityReport, error) {
	if err := req.Validate(); err != nil {
		return CapabilityReport{}, err
	}
	entry, ok := MatrixEntryFor(req.Edition)
	if !ok {
		return CapabilityReport{}, NewSanitizedError(CodeProfileNotFound,
			fmt.Sprintf("No compatibility fixture for edition %q.", req.Edition))
	}

	ctx, cancel := context.WithTimeout(ctx, req.Deadline)
	defer cancel()
	select {
	case <-ctx.Done():
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return CapabilityReport{}, probeTimeoutError(req.ConnectionID)
		}
		return CapabilityReport{}, NewSanitizedError(CodeEndpointRejected, "Capability probe was cancelled.")
	default:
	}

	report := CapabilityReport{
		ConnectionID:    req.ConnectionID,
		Edition:         req.Edition,
		ProbedAt:        p.now().UTC(),
		QueryDialects:   cloneOutcomes(entry.QueryDialects),
		WriteProtocols:  cloneOutcomes(entry.WriteProtocols),
		SchemaDiscovery: entry.SchemaDiscovery,
		TLS:             req.TLS,
		Deadline:        req.Deadline,
		Mock:            true,
	}
	if err := report.Validate(); err != nil {
		return CapabilityReport{}, err
	}
	return report, nil
}

// ProbeCapabilities adapts the edition prober to the CapabilityProber interface
// using the fixture profile's edition-derived capabilities. It is mock-only.
func (p *EditionProber) ProbeCapabilities(ctx context.Context, profile fixtures.ConnectionProfile, deadline time.Duration) (fixtures.Capabilities, error) {
	entry, ok := matrixEntryForMajor(profile.MajorVersion)
	if !ok {
		return fixtures.Capabilities{}, NewSanitizedError(CodeUnsupportedCapability,
			fmt.Sprintf("No compatibility fixture for major version %d.", profile.MajorVersion))
	}
	report, err := p.Probe(ctx, ProbeRequest{
		ConnectionID: profile.ConnectionID,
		Edition:      entry.Edition,
		Deadline:     deadline,
		TLS:          DefaultTLSExpectation(profile.ConnectionID),
	})
	if err != nil {
		return fixtures.Capabilities{}, err
	}
	return report.ToCapabilities(), nil
}

// ToCapabilities projects a report onto the fixture capability shape, keeping
// only positively-supported capabilities.
func (r CapabilityReport) ToCapabilities() fixtures.Capabilities {
	caps := fixtures.Capabilities{SchemaDiscovery: r.SchemaDiscovery == OutcomeSupported}
	for dialect, outcome := range r.QueryDialects {
		if outcome == OutcomeSupported {
			caps.QueryDialects = append(caps.QueryDialects, dialect)
		}
	}
	for protocol, outcome := range r.WriteProtocols {
		if outcome == OutcomeSupported {
			caps.WriteProtocols = append(caps.WriteProtocols, protocol)
		}
	}
	sort.Strings(caps.QueryDialects)
	sort.Strings(caps.WriteProtocols)
	return caps
}

func matrixEntryForMajor(major int) (MatrixEntry, bool) {
	for _, entry := range CompatibilityMatrix() {
		if entry.MajorVersion == major {
			return entry, true
		}
	}
	return MatrixEntry{}, false
}

func cloneOutcomes(in map[string]ProbeOutcome) map[string]ProbeOutcome {
	out := make(map[string]ProbeOutcome, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
