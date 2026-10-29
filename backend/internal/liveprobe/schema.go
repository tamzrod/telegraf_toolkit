// Authorized, bounded, read-only schema discovery (task P3-002).
//
// The approved read path (docs/decisions/phase-3-schema-discovery-gate.md §11,
// Option A1) executes a fixed allowlist of GET metadata statements against the
// deployment resolved from a stored Registry profile. There is no caller-
// supplied query, no POST, no body, no DDL, no SELECT ... INTO and no write.
//
// Safety model:
//   - The endpoint, scope and credential come from the profile only. This
//     package never accepts a URL, credential or query from the caller.
//   - The same endpoint policy, single DNS resolution, pinned peers and
//     verified TLS as Probe/Verify apply.
//   - Only SHOW MEASUREMENTS, SHOW FIELD KEYS FROM <validated measurement> and
//     SHOW TAG KEYS FROM <validated measurement> are ever issued.
//   - A field/tag read first observes the measurement list and requires an
//     exact membership match, so a crafted name cannot reach the statement.
//     The identifier is also escaped, as defense in depth.
//   - Every step has a deadline, a request budget, a row cap and a response
//     byte cap. Truncation is reported, never hidden.
//   - Outcomes are three-valued. A 404, 405, 401, timeout, server error or
//     unrecognized body is unknown, never "unsupported". This read path claims
//     no "unsupported" state from a negative observation; only an observed,
//     positive metadata response is "supported". InfluxDB 1.x is unknown.
//   - Observations cite the check, method, path and observed edition.
package liveprobe

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/registrycontract"
)

// SchemaScope kinds. They mirror fixtures.ScopeKind; a test asserts parity so
// the two spellings cannot drift.
const (
	SchemaScopeOrganizationBucket      = "organization-bucket"
	SchemaScopeDatabase                = "database"
	SchemaScopeDatabaseRetentionPolicy = "database-retention-policy"
)

// SchemaKind is the metadata a read asks for.
type SchemaKind string

// Supported schema kinds.
const (
	SchemaMeasurements SchemaKind = "measurements"
	SchemaFields       SchemaKind = "fields"
	SchemaTags         SchemaKind = "tags"
)

// Schema read bounds. They are server-side constants: a caller cannot raise
// them through the API.
const (
	DefaultSchemaDeadline    = 15 * time.Second
	MaxSchemaDeadline        = 30 * time.Second
	DefaultSchemaMaxRequests = 8
	MaxSchemaMaxRequests     = 12
	DefaultSchemaMaxRows     = 1000
	MaxSchemaMaxRows         = 5000
	schemaResponseLimit      = 1 << 20
	schemaPerRequestTimeout  = 5 * time.Second
	maxMeasurementNameLength = 512
)

// SchemaScope is the logical database scope of a profile, resolved on the
// server from the stored profile. It is never built from request parameters.
type SchemaScope struct {
	Kind            string
	Organization    string
	Bucket          string
	Database        string
	RetentionPolicy string
}

// Canonical returns the stable scope identity used for display and for
// detecting a stale or changed selection. It matches fixtures.Scope.String.
func (s SchemaScope) Canonical() string {
	switch s.Kind {
	case SchemaScopeOrganizationBucket:
		return s.Organization + "/" + s.Bucket
	case SchemaScopeDatabaseRetentionPolicy:
		return s.Database + "/" + s.RetentionPolicy
	case SchemaScopeDatabase:
		return s.Database
	default:
		return s.Kind
	}
}

// SchemaRequest asks for one bounded metadata read. Measurement is required
// for SchemaFields and SchemaTags and must be empty for SchemaMeasurements.
type SchemaRequest struct {
	ConnectionID string
	Endpoint     string
	Scope        SchemaScope
	Kind         SchemaKind
	Measurement  string
	Policy       Policy
	Credential   Credential
	Deadline     time.Duration
	MaxRequests  int
	MaxRows      int
}

// SchemaField is an observed field key and its reported type.
type SchemaField struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// SchemaObservation is the evidence behind one step of a schema read. It never
// contains a credential, a resolved secret or a credential-bearing URL.
type SchemaObservation struct {
	// Check names the step, for example "schema-v2-measurements".
	Check string `json:"check"`
	// Method is the HTTP method used. The read path only ever uses GET.
	Method string `json:"method"`
	// Path is the request path with its query. It is built from fixed
	// statements and a validated measurement name and never contains a secret.
	Path string `json:"path"`
	// Status is the observed HTTP status, or 0 when no request was made.
	Status int `json:"status"`
	// Outcome is the three-valued result of this step.
	Outcome registrycontract.ProbeOutcome `json:"outcome"`
	// Edition is the deployment the step ran against, when identified.
	Edition string `json:"edition,omitempty"`
	// Detail is a short, sanitized explanation.
	Detail string `json:"detail"`
}

// SchemaResult is the sanitized result of a schema read. Measurements, Fields
// and Tags are always non-nil so an observed empty schema is distinguishable
// from an absent field.
type SchemaResult struct {
	ConnectionID string
	// Scope is the canonical scope identity the read used.
	Scope string
	// Edition is the identified deployment, or empty when no request was made.
	Edition      string
	Kind         SchemaKind
	Measurement  string
	Outcome      registrycontract.ProbeOutcome
	Measurements []string
	Fields       []SchemaField
	Tags         []string
	Observations []SchemaObservation
	Truncated    bool
	Requests     int
	ReadAt       time.Time
}

// ReadSchema performs one authorized, bounded, read-only metadata read.
//
// Endpoint-policy and DNS failures are returned as sanitized errors. A
// transport failure after validation (timeout, TLS, refused connection) and an
// answer that does not prove the metadata are reported as an unknown outcome
// with its observation, so "unknown" is never presented as support and never
// becomes an error that might be mistaken for a capability conclusion.
func (p *Prober) ReadSchema(ctx context.Context, req SchemaRequest) (SchemaResult, error) {
	if err := req.Policy.Validate(); err != nil {
		return SchemaResult{}, err
	}
	if strings.TrimSpace(req.ConnectionID) == "" {
		return SchemaResult{}, registrycontract.NewSanitizedError(registrycontract.CodeInvalidProfile, "A schema read requires a connection_id.")
	}
	if req.Deadline <= 0 {
		return SchemaResult{}, registrycontract.NewSanitizedError(registrycontract.CodeInvalidProfile, "A schema read requires a positive deadline.")
	}
	if req.MaxRequests <= 0 {
		return SchemaResult{}, registrycontract.NewSanitizedError(registrycontract.CodeInvalidProfile, "A schema read requires a positive request budget.")
	}
	if req.MaxRows <= 0 {
		return SchemaResult{}, registrycontract.NewSanitizedError(registrycontract.CodeInvalidProfile, "A schema read requires a positive row cap.")
	}
	switch req.Kind {
	case SchemaMeasurements:
		if req.Measurement != "" {
			return SchemaResult{}, registrycontract.NewSanitizedError(registrycontract.CodeInvalidProfile, "A measurement listing does not take a measurement name.")
		}
	case SchemaFields, SchemaTags:
		if err := validateMeasurementName(req.Measurement); err != nil {
			return SchemaResult{}, err
		}
	default:
		return SchemaResult{}, registrycontract.NewSanitizedError(registrycontract.CodeInvalidProfile, "Unknown schema read kind.")
	}
	if err := validateSchemaScope(req.Scope); err != nil {
		return SchemaResult{}, err
	}

	result := SchemaResult{
		ConnectionID: req.ConnectionID,
		Scope:        req.Scope.Canonical(),
		Kind:         req.Kind,
		Measurement:  req.Measurement,
		Outcome:      registrycontract.OutcomeUnknown,
		Measurements: make([]string, 0),
		Fields:       make([]SchemaField, 0),
		Tags:         make([]string, 0),
		Observations: make([]SchemaObservation, 0),
		ReadAt:       time.Now().UTC(),
	}

	// InfluxDB 1.x is not verified by this read path. No request is made, and
	// the outcome stays unknown: the absence of evidence is not support.
	if req.Scope.Kind == SchemaScopeDatabaseRetentionPolicy {
		result.Observations = append(result.Observations, SchemaObservation{
			Check:   "schema-1x-unverified",
			Method:  "none",
			Edition: "unknown",
			Outcome: registrycontract.OutcomeUnknown,
			Detail:  "not attempted: the read path does not verify InfluxDB 1.x metadata discovery",
		})
		return result, nil
	}

	endpoint, err := validateEndpoint(req.Endpoint, req.Policy)
	if err != nil {
		return SchemaResult{}, err
	}
	// One DNS resolution, then dial only the validated peers. A second lookup
	// cannot redirect the read (F-01).
	peers, err := p.resolveAndCheckPeers(ctx, endpoint.Hostname(), req.Policy)
	if err != nil {
		return SchemaResult{}, err
	}
	client, err := newPinnedClient(endpoint, peers, req.Policy)
	if err != nil {
		return SchemaResult{}, err
	}

	ctx, cancel := context.WithTimeout(ctx, req.Deadline)
	defer cancel()

	base := *endpoint
	base.Path = ""
	base.RawQuery = ""
	base.Fragment = ""
	root := strings.TrimRight(base.String(), "/")

	budget := req.MaxRequests
	get := func(check, path string) (SchemaObservation, string, bool, error) {
		if budget <= 0 {
			result.Observations = append(result.Observations, SchemaObservation{
				Check:   check,
				Method:  "GET",
				Path:    path,
				Edition: result.Edition,
				Outcome: registrycontract.OutcomeUnknown,
				Detail:  "not attempted: the read request budget was exhausted",
			})
			return SchemaObservation{}, "", false, errSchemaBudget
		}
		budget--
		result.Requests++
		requestCtx, cancelRequest := context.WithTimeout(ctx, schemaPerRequestTimeout)
		defer cancelRequest()
		status, body, truncated, err := p.getLimited(requestCtx, client, root+path, req.Credential, schemaResponseLimit)
		if err != nil {
			result.Observations = append(result.Observations, SchemaObservation{
				Check:   check,
				Method:  "GET",
				Path:    path,
				Edition: result.Edition,
				Outcome: registrycontract.OutcomeUnknown,
				Detail:  "the request did not complete",
			})
			return SchemaObservation{}, "", false, errSchemaRequestError
		}
		return SchemaObservation{Check: check, Method: "GET", Path: path, Status: status}, body, truncated, nil
	}

	// Identify the product from a read-only response. The label identifies the
	// product only; the adapter still has to observe the metadata itself.
	edition, err := p.probeEdition(root, func(path string) (int, string, error) {
		if budget <= 0 {
			return 0, "", errSchemaBudget
		}
		budget--
		result.Requests++
		requestCtx, cancelRequest := context.WithTimeout(ctx, schemaPerRequestTimeout)
		defer cancelRequest()
		status, body, _, err := p.getLimited(requestCtx, client, root+path, req.Credential, schemaResponseLimit)
		if err != nil {
			return 0, "", errSchemaRequestError
		}
		return status, body, nil
	}, req.Credential)
	if err != nil {
		if errors.Is(err, errSchemaBudget) || errors.Is(err, errSchemaRequestError) {
			result.Observations = append(result.Observations, SchemaObservation{
				Check:   "schema-edition",
				Method:  "GET",
				Outcome: registrycontract.OutcomeUnknown,
				Detail:  "not identified: the deployment edition could not be observed",
			})
			result.Outcome = registrycontract.OutcomeUnknown
			return result, nil
		}
		// An unidentifiable deployment is an unknown edition, not a claim and
		// not a capability conclusion. Other policy failures remain errors.
		var sanitized *registrycontract.SanitizedError
		if errors.As(err, &sanitized) && sanitized.Code == registrycontract.CodeEndpointRejected {
			result.Observations = append(result.Observations, SchemaObservation{
				Check:   "schema-edition",
				Method:  "GET",
				Outcome: registrycontract.OutcomeUnknown,
				Detail:  "not identified: the deployment edition could not be observed",
			})
			result.Outcome = registrycontract.OutcomeUnknown
			return result, nil
		}
		return SchemaResult{}, err
	}
	result.Edition = edition

	// A scope that does not match the observed edition cannot be read safely:
	// the statement would target a different namespace than the profile says.
	if !scopeMatchesEdition(req.Scope, edition) {
		result.Observations = append(result.Observations, SchemaObservation{
			Check:   "schema-scope-edition",
			Method:  "GET",
			Outcome: registrycontract.OutcomeUnknown,
			Edition: edition,
			Detail:  "not attempted: the stored profile scope does not match the detected deployment edition",
		})
		return result, nil
	}

	state := &schemaReadState{
		req:    req,
		result: &result,
		get:    get,
	}
	if req.Scope.Kind == SchemaScopeOrganizationBucket {
		err = p.readSchemaV2(ctx, state)
	} else {
		err = p.readSchemaV3(ctx, state)
	}
	if err != nil {
		if errors.Is(err, errSchemaBudget) || errors.Is(err, errSchemaRequestError) {
			result.Outcome = registrycontract.OutcomeUnknown
			return result, nil
		}
		return SchemaResult{}, err
	}
	return result, nil
}

// schemaReadState carries the bounded request closure through the adapter.
//
// database, retentionPolicy and edition are resolved from the stored profile
// and the observed deployment; they are never read from a request.
type schemaReadState struct {
	req             SchemaRequest
	result          *SchemaResult
	database        string
	retentionPolicy string
	edition         string
	get             func(check, path string) (SchemaObservation, string, bool, error)
}

// Internal control signals. The caller turns each into an unknown outcome with
// its observation, not an HTTP error: a read that could not complete is not
// evidence of support, and it is not a claim that the capability is absent.
var (
	errSchemaBudget       = errors.New("schema read request budget exhausted")
	errSchemaRequestError = errors.New("schema read request did not complete")
)

// readSchemaV2 reads metadata from an InfluxDB 2.x deployment using the
// 1.x-compatibility metadata endpoint. The DBRP database name (not the bucket
// alias) is required by that endpoint, so it is resolved with read-only API
// calls first.
func (p *Prober) readSchemaV2(ctx context.Context, state *schemaReadState) error {
	state.edition = "influxdb-2.x-oss"
	bucketID, ok, err := state.resolveBucketV2(state.req.Scope.Organization, state.req.Scope.Bucket)
	if err != nil {
		return err
	}
	if !ok {
		state.result.Observations = append(state.result.Observations, SchemaObservation{
			Check:   "schema-v2-scope",
			Method:  "GET",
			Outcome: registrycontract.OutcomeUnknown,
			Edition: "influxdb-2.x-oss",
			Detail:  "the profile's organization/bucket scope was not observed on the deployment",
		})
		state.result.Outcome = registrycontract.OutcomeUnknown
		return nil
	}
	database, retentionPolicy, ok, err := state.resolveDBRPV2(state.req.Scope.Organization, bucketID)
	if err != nil {
		return err
	}
	if !ok {
		state.result.Observations = append(state.result.Observations, SchemaObservation{
			Check:   "schema-v2-dbrp",
			Method:  "GET",
			Outcome: registrycontract.OutcomeUnknown,
			Edition: "influxdb-2.x-oss",
			Detail:  "no database/retention-policy mapping was observed for the profile's bucket",
		})
		state.result.Outcome = registrycontract.OutcomeUnknown
		return nil
	}
	state.database = database
	state.retentionPolicy = retentionPolicy

	if state.req.Kind == SchemaMeasurements {
		return state.readV2Measurements()
	}
	// Fields and tags require the name to be a member of the observed
	// measurement list. This is what stops a crafted name reaching a statement.
	if err := state.requireObservedMeasurement(); err != nil {
		return err
	}
	return state.readV2MeasurementDetail()
}

// readSchemaV3 reads metadata from an InfluxDB 3 Core deployment using the
// InfluxQL query endpoint measured on the pinned edition.
func (p *Prober) readSchemaV3(ctx context.Context, state *schemaReadState) error {
	state.database = state.req.Scope.Database
	state.edition = "influxdb-3.x-core"

	if state.req.Kind == SchemaMeasurements {
		return state.readV3Measurements()
	}
	if err := state.requireObservedMeasurement(); err != nil {
		return err
	}
	return state.readV3MeasurementDetail()
}

// resolveBucketV2 asks the deployment for the profile's bucket by name and
// organization. It returns ok=false when the deployment answers successfully
// without that bucket; an unrecognized answer leaves the read unknown.
func (s *schemaReadState) resolveBucketV2(org, bucket string) (string, bool, error) {
	params := url.Values{}
	params.Set("name", bucket)
	params.Set("org", org)
	path := "/api/v2/buckets?" + params.Encode()
	obs, body, truncated, err := s.get("schema-v2-buckets", path)
	if err != nil {
		return "", false, err
	}
	if obs.Status != http.StatusOK {
		outcome, detail := schemaOutcome(obs.Status, false)
		obs.Detail = detail
		s.appendObserved(obs, outcome)
		return "", false, nil
	}
	if truncated {
		s.appendObserved(obs, registrycontract.OutcomeUnknown)
		s.result.Truncated = true
		return "", false, nil
	}
	var decoded struct {
		Buckets []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"buckets"`
	}
	if err := json.Unmarshal([]byte(body), &decoded); err != nil {
		s.appendObserved(obs, registrycontract.OutcomeUnknown)
		return "", false, nil
	}
	for _, candidate := range decoded.Buckets {
		if candidate.Name == bucket && candidate.ID != "" {
			s.appendObserved(obs, registrycontract.OutcomeSupported)
			return candidate.ID, true, nil
		}
	}
	s.appendObserved(obs, registrycontract.OutcomeUnknown)
	return "", false, nil
}

// resolveDBRPV2 asks for the bucket's database/retention-policy mapping. It
// only reads; it never creates a mapping.
func (s *schemaReadState) resolveDBRPV2(org, bucketID string) (string, string, bool, error) {
	params := url.Values{}
	params.Set("org", org)
	params.Set("bucketID", bucketID)
	path := "/api/v2/dbrps?" + params.Encode()
	obs, body, truncated, err := s.get("schema-v2-dbrps", path)
	if err != nil {
		return "", "", false, err
	}
	if obs.Status != http.StatusOK {
		outcome, detail := schemaOutcome(obs.Status, false)
		obs.Detail = detail
		s.appendObserved(obs, outcome)
		return "", "", false, nil
	}
	if truncated {
		s.appendObserved(obs, registrycontract.OutcomeUnknown)
		s.result.Truncated = true
		return "", "", false, nil
	}
	var decoded struct {
		Content []struct {
			Database        string `json:"database"`
			RetentionPolicy string `json:"retention_policy"`
			Default         bool   `json:"default"`
			BucketID        string `json:"bucketID"`
		} `json:"content"`
	}
	if err := json.Unmarshal([]byte(body), &decoded); err != nil {
		s.appendObserved(obs, registrycontract.OutcomeUnknown)
		return "", "", false, nil
	}
	chosen := -1
	for i, candidate := range decoded.Content {
		if candidate.BucketID != bucketID || candidate.Database == "" {
			continue
		}
		if candidate.Default {
			chosen = i
			break
		}
		if chosen < 0 {
			chosen = i
		}
	}
	if chosen < 0 {
		s.appendObserved(obs, registrycontract.OutcomeUnknown)
		return "", "", false, nil
	}
	s.appendObserved(obs, registrycontract.OutcomeSupported)
	return decoded.Content[chosen].Database, decoded.Content[chosen].RetentionPolicy, true, nil
}

func (s *schemaReadState) v2QueryPath(statement string) string {
	params := url.Values{}
	params.Set("db", s.database)
	if s.retentionPolicy != "" {
		params.Set("rp", s.retentionPolicy)
	}
	params.Set("q", statement)
	return "/query?" + params.Encode()
}

func (s *schemaReadState) v3QueryPath(statement string) string {
	params := url.Values{}
	params.Set("db", s.database)
	params.Set("q", statement)
	params.Set("format", "json")
	return "/api/v3/query_influxql?" + params.Encode()
}

// readV2Measurements observes the measurement list and records it.
func (s *schemaReadState) readV2Measurements() error {
	names, obs, outcome, err := s.queryV2Measurements()
	if err != nil {
		return err
	}
	s.appendObserved(obs, outcome)
	s.result.Measurements = capStrings(names, s.req.MaxRows, &s.result.Truncated)
	s.result.Outcome = outcome
	return nil
}

func (s *schemaReadState) readV3Measurements() error {
	names, obs, outcome, err := s.queryV3Measurements()
	if err != nil {
		return err
	}
	s.appendObserved(obs, outcome)
	s.result.Measurements = capStrings(names, s.req.MaxRows, &s.result.Truncated)
	s.result.Outcome = outcome
	return nil
}

// requireObservedMeasurement verifies that the requested measurement is in the
// observed measurement list. A name outside the list is refused; a truncated
// list cannot prove absence, so it leaves the read unknown rather than a 404.
func (s *schemaReadState) requireObservedMeasurement() error {
	var (
		names   []string
		obs     SchemaObservation
		outcome registrycontract.ProbeOutcome
		err     error
	)
	if s.edition == "influxdb-2.x-oss" {
		names, obs, outcome, err = s.queryV2Measurements()
	} else {
		names, obs, outcome, err = s.queryV3Measurements()
	}
	if err != nil {
		return err
	}
	s.appendObserved(obs, outcome)
	if outcome != registrycontract.OutcomeSupported {
		s.result.Outcome = registrycontract.OutcomeUnknown
		return nil
	}
	truncated := len(names) > s.req.MaxRows
	names = capStrings(names, s.req.MaxRows, &s.result.Truncated)
	if containsString(names, s.req.Measurement) {
		return nil
	}
	if truncated {
		s.result.Observations = append(s.result.Observations, SchemaObservation{
			Check:   "schema-measurement-membership",
			Method:  "GET",
			Outcome: registrycontract.OutcomeUnknown,
			Edition: s.edition,
			Detail:  "the measurement could not be verified because the observed measurement list was truncated",
		})
		s.result.Outcome = registrycontract.OutcomeUnknown
		return nil
	}
	return registrycontract.NewSanitizedError(registrycontract.CodeMeasurementNotFound, "No measurement matches the requested name in the profile's scope.")
}

// readV2MeasurementDetail reads field keys or tag keys for a measurement whose
// membership has already been observed.
func (s *schemaReadState) readV2MeasurementDetail() error {
	statement := s.statementForKind()
	path := s.v2QueryPath(statement)
	obs, body, truncated, err := s.get("schema-v2-"+string(s.req.Kind), path)
	if err != nil {
		return err
	}
	if obs.Status != http.StatusOK {
		outcome, detail := schemaOutcome(obs.Status, false)
		obs.Detail = detail
		s.appendObserved(obs, outcome)
		s.result.Outcome = registrycontract.OutcomeUnknown
		return nil
	}
	if truncated {
		s.appendObserved(obs, registrycontract.OutcomeUnknown)
		s.result.Truncated = true
		s.result.Outcome = registrycontract.OutcomeUnknown
		return nil
	}
	recognized := parseInfluxQLV2Detail(s.req, body, s.result)
	outcome, detail := schemaOutcome(obs.Status, recognized)
	obs.Detail = detail
	s.appendObserved(obs, outcome)
	s.result.Outcome = outcome
	return nil
}

func (s *schemaReadState) readV3MeasurementDetail() error {
	statement := s.statementForKind()
	path := s.v3QueryPath(statement)
	obs, body, truncated, err := s.get("schema-v3-"+string(s.req.Kind), path)
	if err != nil {
		return err
	}
	if obs.Status != http.StatusOK {
		outcome, detail := schemaOutcome(obs.Status, false)
		obs.Detail = detail
		s.appendObserved(obs, outcome)
		s.result.Outcome = registrycontract.OutcomeUnknown
		return nil
	}
	if truncated {
		s.appendObserved(obs, registrycontract.OutcomeUnknown)
		s.result.Truncated = true
		s.result.Outcome = registrycontract.OutcomeUnknown
		return nil
	}
	recognized := parseJSONV3Detail(s.req, body, s.result)
	outcome, detail := schemaOutcome(obs.Status, recognized)
	obs.Detail = detail
	s.appendObserved(obs, outcome)
	s.result.Outcome = outcome
	return nil
}

// statementForKind returns one of the three approved statements with a validated
// and quoted measurement identifier. No caller string is ever interpolated
// without validation.
func (s *schemaReadState) statementForKind() string {
	switch s.req.Kind {
	case SchemaFields:
		return "SHOW FIELD KEYS FROM " + quoteMeasurement(s.req.Measurement)
	case SchemaTags:
		return "SHOW TAG KEYS FROM " + quoteMeasurement(s.req.Measurement)
	default:
		return "SHOW MEASUREMENTS"
	}
}

func (s *schemaReadState) queryV2Measurements() ([]string, SchemaObservation, registrycontract.ProbeOutcome, error) {
	path := s.v2QueryPath("SHOW MEASUREMENTS")
	obs, body, truncated, err := s.get("schema-v2-measurements", path)
	if err != nil {
		return nil, obs, registrycontract.OutcomeUnknown, err
	}
	if obs.Status != http.StatusOK {
		outcome, detail := schemaOutcome(obs.Status, false)
		obs.Detail = detail
		return nil, obs, outcome, nil
	}
	if truncated {
		s.result.Truncated = true
		obs.Detail = "the response exceeded the bounded size"
		return nil, obs, registrycontract.OutcomeUnknown, nil
	}
	names, recognized := parseInfluxQLV2Measurements(body)
	if !recognized {
		outcome, detail := schemaOutcome(obs.Status, false)
		obs.Detail = detail
		return nil, obs, outcome, nil
	}
	sort.Strings(names)
	obs.Detail = "the endpoint returned the expected metadata"
	return names, obs, registrycontract.OutcomeSupported, nil
}

func (s *schemaReadState) queryV3Measurements() ([]string, SchemaObservation, registrycontract.ProbeOutcome, error) {
	path := s.v3QueryPath("SHOW MEASUREMENTS")
	obs, body, truncated, err := s.get("schema-v3-measurements", path)
	if err != nil {
		return nil, obs, registrycontract.OutcomeUnknown, err
	}
	if obs.Status != http.StatusOK {
		outcome, detail := schemaOutcome(obs.Status, false)
		obs.Detail = detail
		return nil, obs, outcome, nil
	}
	if truncated {
		s.result.Truncated = true
		obs.Detail = "the response exceeded the bounded size"
		return nil, obs, registrycontract.OutcomeUnknown, nil
	}
	names, recognized := parseJSONV3Measurements(body)
	if !recognized {
		outcome, detail := schemaOutcome(obs.Status, false)
		obs.Detail = detail
		return nil, obs, outcome, nil
	}
	sort.Strings(names)
	obs.Detail = "the endpoint returned the expected metadata"
	return names, obs, registrycontract.OutcomeSupported, nil
}

func (s *schemaReadState) appendObserved(obs SchemaObservation, outcome registrycontract.ProbeOutcome) {
	if obs.Edition == "" {
		obs.Edition = s.edition
	}
	obs.Outcome = outcome
	s.result.Observations = append(s.result.Observations, obs)
}

// schemaOutcome maps an observed status and body recognition to the
// three-valued vocabulary. Only a 200 with the expected shape is support.
func schemaOutcome(status int, recognized bool) (registrycontract.ProbeOutcome, string) {
	switch {
	case status == http.StatusOK && recognized:
		return registrycontract.OutcomeSupported, "the endpoint returned the expected metadata"
	case status == http.StatusOK:
		return registrycontract.OutcomeUnknown, "the endpoint answered 200 with an unrecognized response shape"
	case status == http.StatusUnauthorized, status == http.StatusForbidden:
		return registrycontract.OutcomeUnknown, "the deployment denied the read credential"
	case status == http.StatusNotFound:
		return registrycontract.OutcomeUnknown, "the requested database or metadata path was not found"
	case status == http.StatusMethodNotAllowed:
		return registrycontract.OutcomeUnknown, "the endpoint does not serve GET for this route"
	case status == http.StatusRequestTimeout, status == http.StatusGatewayTimeout:
		return registrycontract.OutcomeUnknown, "the deployment did not answer in time"
	case status >= 500:
		return registrycontract.OutcomeUnknown, "the deployment reported a server error"
	case status == 0:
		return registrycontract.OutcomeUnknown, "the request did not complete"
	default:
		return registrycontract.OutcomeUnknown, "the deployment rejected the metadata statement"
	}
}

// parseInfluxQLV2Measurements parses the 1.x-compatible results envelope. It
// reports recognition separately from emptiness: an empty schema is evidence,
// an unexpected shape is not.
func parseInfluxQLV2Measurements(body string) ([]string, bool) {
	series, recognized := parseInfluxQLV2Series(body)
	if !recognized {
		return nil, false
	}
	names := make([]string, 0)
	for _, entry := range series {
		if !isNameColumns(entry.Columns) {
			return nil, false
		}
		for _, row := range entry.Values {
			if len(row) == 0 {
				continue
			}
			var name string
			if err := json.Unmarshal(row[0], &name); err != nil || name == "" {
				return nil, false
			}
			names = append(names, name)
		}
	}
	return dedupStrings(names), true
}

// parseInfluxQLV2Detail parses fields or tags for one measurement.
func parseInfluxQLV2Detail(req SchemaRequest, body string, result *SchemaResult) bool {
	series, recognized := parseInfluxQLV2Series(body)
	if !recognized {
		return false
	}
	switch req.Kind {
	case SchemaFields:
		fields := make([]SchemaField, 0)
		for _, entry := range series {
			if len(entry.Columns) != 2 || entry.Columns[0] != "fieldKey" || entry.Columns[1] != "fieldType" {
				return false
			}
			if entry.Name != "" && entry.Name != req.Measurement {
				continue
			}
			for _, row := range entry.Values {
				if len(row) != 2 {
					return false
				}
				var name, fieldType string
				if err := json.Unmarshal(row[0], &name); err != nil || name == "" {
					return false
				}
				if err := json.Unmarshal(row[1], &fieldType); err != nil {
					return false
				}
				fields = append(fields, SchemaField{Name: name, Type: fieldType})
			}
		}
		result.Fields = dedupFields(fields)
		return true
	case SchemaTags:
		tags := make([]string, 0)
		for _, entry := range series {
			if !isTagKeyColumns(entry.Columns) {
				return false
			}
			if entry.Name != "" && entry.Name != req.Measurement {
				continue
			}
			for _, row := range entry.Values {
				if len(row) == 0 {
					return false
				}
				var tag string
				if err := json.Unmarshal(row[0], &tag); err != nil || tag == "" {
					return false
				}
				tags = append(tags, tag)
			}
		}
		result.Tags = dedupStrings(tags)
		return true
	default:
		return false
	}
}

type influxQLV2Series struct {
	Name    string              `json:"name"`
	Columns []string            `json:"columns"`
	Values  [][]json.RawMessage `json:"values"`
}

func parseInfluxQLV2Series(body string) ([]influxQLV2Series, bool) {
	var decoded struct {
		Results []struct {
			Error  string             `json:"error"`
			Series []influxQLV2Series `json:"series"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(body), &decoded); err != nil {
		return nil, false
	}
	if len(decoded.Results) == 0 {
		return nil, false
	}
	out := make([]influxQLV2Series, 0, len(decoded.Results))
	for _, entry := range decoded.Results {
		if entry.Error != "" {
			return nil, false
		}
		out = append(out, entry.Series...)
	}
	return out, true
}

// parseJSONV3Measurements parses the JSON array the pinned 3.x InfluxQL
// endpoint returns for SHOW MEASUREMENTS.
func parseJSONV3Measurements(body string) ([]string, bool) {
	rows, ok := parseJSONV3Rows(body)
	if !ok {
		return nil, false
	}
	names := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.IOXMeasurement != "" && row.IOXMeasurement != "measurements" {
			return nil, false
		}
		if row.Name == "" {
			return nil, false
		}
		names = append(names, row.Name)
	}
	return dedupStrings(names), true
}

// parseJSONV3Detail parses fields or tags for one measurement.
func parseJSONV3Detail(req SchemaRequest, body string, result *SchemaResult) bool {
	rows, ok := parseJSONV3Rows(body)
	if !ok {
		return false
	}
	switch req.Kind {
	case SchemaFields:
		fields := make([]SchemaField, 0, len(rows))
		for _, row := range rows {
			if row.IOXMeasurement != "" && row.IOXMeasurement != req.Measurement {
				continue
			}
			if row.FieldKey == "" {
				return false
			}
			fields = append(fields, SchemaField{Name: row.FieldKey, Type: row.FieldType})
		}
		result.Fields = dedupFields(fields)
		return true
	case SchemaTags:
		tags := make([]string, 0, len(rows))
		for _, row := range rows {
			if row.IOXMeasurement != "" && row.IOXMeasurement != req.Measurement {
				continue
			}
			if row.TagKey == "" {
				return false
			}
			tags = append(tags, row.TagKey)
		}
		result.Tags = dedupStrings(tags)
		return true
	default:
		return false
	}
}

type jsonV3Row struct {
	IOXMeasurement string `json:"iox::measurement"`
	Name           string `json:"name"`
	FieldKey       string `json:"fieldKey"`
	FieldType      string `json:"fieldType"`
	TagKey         string `json:"tagKey"`
}

func parseJSONV3Rows(body string) ([]jsonV3Row, bool) {
	var rows []jsonV3Row
	if err := json.Unmarshal([]byte(body), &rows); err != nil {
		return nil, false
	}
	return rows, true
}

func isNameColumns(columns []string) bool {
	return len(columns) == 1 && columns[0] == "name"
}

func isTagKeyColumns(columns []string) bool {
	return len(columns) == 1 && columns[0] == "tagKey"
}

func validateMeasurementName(name string) error {
	if strings.TrimSpace(name) == "" {
		return registrycontract.NewSanitizedError(registrycontract.CodeInvalidProfile, "A field or tag read requires a measurement name.")
	}
	if len(name) > maxMeasurementNameLength {
		return registrycontract.NewSanitizedError(registrycontract.CodeInvalidProfile, "The measurement name is too long.")
	}
	if !utf8.ValidString(name) {
		return registrycontract.NewSanitizedError(registrycontract.CodeInvalidProfile, "The measurement name must be valid UTF-8.")
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return registrycontract.NewSanitizedError(registrycontract.CodeInvalidProfile, "The measurement name contains a control character.")
		}
	}
	return nil
}

func validateSchemaScope(scope SchemaScope) error {
	switch scope.Kind {
	case SchemaScopeOrganizationBucket:
		if strings.TrimSpace(scope.Organization) == "" || strings.TrimSpace(scope.Bucket) == "" {
			return registrycontract.NewSanitizedError(registrycontract.CodeScopeIncomplete, "The profile scope requires an organization and a bucket.")
		}
	case SchemaScopeDatabase:
		if strings.TrimSpace(scope.Database) == "" {
			return registrycontract.NewSanitizedError(registrycontract.CodeScopeIncomplete, "The profile scope requires a database.")
		}
	case SchemaScopeDatabaseRetentionPolicy:
		if strings.TrimSpace(scope.Database) == "" || strings.TrimSpace(scope.RetentionPolicy) == "" {
			return registrycontract.NewSanitizedError(registrycontract.CodeScopeIncomplete, "The profile scope requires a database and a retention policy.")
		}
	default:
		return registrycontract.NewSanitizedError(registrycontract.CodeScopeIncomplete, "The profile scope kind is not recognized.")
	}
	return nil
}

func scopeMatchesEdition(scope SchemaScope, edition string) bool {
	switch scope.Kind {
	case SchemaScopeOrganizationBucket:
		return edition == "influxdb-2.x-oss"
	case SchemaScopeDatabase:
		return edition == "influxdb-3.x-core"
	default:
		return false
	}
}

// quoteMeasurement quotes an InfluxQL identifier. The caller must have already
// validated the name and observed it in the measurement list; escaping is a
// second, independent barrier.
func quoteMeasurement(name string) string {
	escaped := strings.ReplaceAll(name, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return `"` + escaped + `"`
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func dedupStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func dedupFields(fields []SchemaField) []SchemaField {
	seen := make(map[string]bool, len(fields))
	out := make([]SchemaField, 0, len(fields))
	for _, field := range fields {
		key := field.Name + "\x00" + field.Type
		if field.Name == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, field)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name == out[j].Name {
			return out[i].Type < out[j].Type
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// capStrings applies the row cap and records truncation. The caller has already
// sorted the values, so the retained prefix is deterministic.
func capStrings(values []string, max int, truncated *bool) []string {
	if max <= 0 || len(values) <= max {
		return values
	}
	*truncated = true
	return values[:max]
}
