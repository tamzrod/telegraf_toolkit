// Package pipelineapi exposes pipeline draft and revision routes.
//
// Every route requires a bearer token and a server-side permission. Saving
// and approving write configuration only. No route deploys Telegraf or writes
// a historian.
package pipelineapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/authz"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/metadatastore"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/pipelinecontract"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/pipelineservice"
)

const bearerPrefix = "Bearer "

// API is the pipeline HTTP surface.
type API struct {
	service       *pipelineservice.Service
	authenticator *authz.Authenticator
	authorizer    *authz.Authorizer
}

// New builds the pipeline API.
func New(service *pipelineservice.Service, authenticator *authz.Authenticator) *API {
	return &API{service: service, authenticator: authenticator, authorizer: authz.NewAuthorizer()}
}

// Register mounts the pipeline routes.
func (a *API) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/pipelines", a.handleCollection)
	mux.HandleFunc("/api/v1/pipelines/", a.handlePipeline)
}

func (a *API) handleCollection(w http.ResponseWriter, r *http.Request) {
	principal, ok := a.principal(w, r)
	if !ok {
		return
	}
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	if !a.authorize(w, principal, authz.PermPipelineWrite) {
		return
	}
	rev, err := a.service.Create(r.Context(), principal.ID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, a.view(r.Context(), rev))
}

func (a *API) handlePipeline(w http.ResponseWriter, r *http.Request) {
	principal, ok := a.principal(w, r)
	if !ok {
		return
	}
	pipelineID, rest := splitPipelinePath(r.URL.Path)
	if pipelineID == "" {
		writeError(w, http.StatusNotFound, "not-found", "No pipeline was requested.")
		return
	}
	if len(rest) == 0 {
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}
		if !a.authorize(w, principal, authz.PermPipelineRead) {
			return
		}
		revs, err := a.service.List(r.Context(), pipelineID)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, a.view(r.Context(), revs[len(revs)-1]))
		return
	}
	if rest[0] == "revisions" && len(rest) == 1 {
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}
		if !a.authorize(w, principal, authz.PermPipelineRead) {
			return
		}
		revs, err := a.service.List(r.Context(), pipelineID)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		history := make([]historyView, 0, len(revs))
		for _, rev := range revs {
			history = append(history, historyItem(rev))
		}
		writeJSON(w, http.StatusOK, map[string]any{"revisions": history, "deployed": false})
		return
	}
	if rest[0] == "drafts" && len(rest) == 1 {
		if r.Method != http.MethodPost {
			methodNotAllowed(w, http.MethodPost)
			return
		}
		if !a.authorize(w, principal, authz.PermPipelineWrite) {
			return
		}
		var body struct {
			SourceRevision int `json:"source_revision"`
		}
		if err := decodeJSON(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid", "The request body could not be read.")
			return
		}
		rev, err := a.service.OpenDraft(r.Context(), principal.ID, pipelineID, body.SourceRevision)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, a.view(r.Context(), rev))
		return
	}
	if len(rest) < 2 || rest[0] != "revisions" {
		writeError(w, http.StatusNotFound, "not-found", "The pipeline route does not exist.")
		return
	}
	number, err := strconv.Atoi(rest[1])
	if err != nil || number < 1 {
		writeError(w, http.StatusNotFound, "not-found", "The pipeline revision does not exist.")
		return
	}
	action := ""
	if len(rest) == 3 {
		action = rest[2]
	}
	if len(rest) > 3 {
		writeError(w, http.StatusNotFound, "not-found", "The pipeline route does not exist.")
		return
	}
	switch {
	case action == "" && r.Method == http.MethodGet:
		if !a.authorize(w, principal, authz.PermPipelineRead) {
			return
		}
		rev, err := a.service.Get(r.Context(), pipelineID, number)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, a.view(r.Context(), rev))
	case action == "" && r.Method == http.MethodPut:
		if !a.authorize(w, principal, authz.PermPipelineWrite) {
			return
		}
		a.save(w, r, principal, pipelineID, number)
	case action == "transition-preview" && r.Method == http.MethodPost:
		if !a.authorize(w, principal, authz.PermPipelineWrite) {
			return
		}
		var body struct {
			TargetMode string `json:"target_mode"`
		}
		if err := decodeJSON(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid", "The request body could not be read.")
			return
		}
		review, err := a.service.PreviewTransition(r.Context(), pipelineID, number, body.TargetMode)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, reviewView(review))
	case action == "transition" && r.Method == http.MethodPost:
		if !a.authorize(w, principal, authz.PermPipelineWrite) {
			return
		}
		a.transition(w, r, principal, pipelineID, number)
	case action == "restore-snapshot" && r.Method == http.MethodPost:
		if !a.authorize(w, principal, authz.PermPipelineWrite) {
			return
		}
		a.restore(w, r, principal, pipelineID, number)
	case action == "approve" && r.Method == http.MethodPost:
		if !a.authorize(w, principal, authz.PermPipelineApprove) {
			return
		}
		a.approve(w, r, principal, pipelineID, number)
	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPut, http.MethodPost)
	}
}

func (a *API) save(w http.ResponseWriter, r *http.Request, principal authz.Principal, pipelineID string, number int) {
	var body struct {
		ExpectedGeneration int             `json:"expected_generation"`
		Document           json.RawMessage `json:"document"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid", "The request body could not be read.")
		return
	}
	current, err := a.service.Get(r.Context(), pipelineID, number)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	rev, err := a.service.Save(r.Context(), principal.ID, current.RevisionID, body.ExpectedGeneration, body.Document)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, a.view(r.Context(), rev))
}

func (a *API) transition(w http.ResponseWriter, r *http.Request, principal authz.Principal, pipelineID string, number int) {
	var body struct {
		ExpectedGeneration int    `json:"expected_generation"`
		TargetMode         string `json:"target_mode"`
		Digest             string `json:"digest"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid", "The request body could not be read.")
		return
	}
	current, err := a.service.Get(r.Context(), pipelineID, number)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	rev, err := a.service.CommitTransition(r.Context(), principal.ID, current.RevisionID, body.ExpectedGeneration, body.TargetMode, body.Digest)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, a.view(r.Context(), rev))
}

func (a *API) restore(w http.ResponseWriter, r *http.Request, principal authz.Principal, pipelineID string, number int) {
	var body struct {
		ExpectedGeneration int    `json:"expected_generation"`
		Digest             string `json:"digest"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid", "The request body could not be read.")
		return
	}
	current, err := a.service.Get(r.Context(), pipelineID, number)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	rev, err := a.service.RestoreSnapshot(r.Context(), principal.ID, current.RevisionID, body.ExpectedGeneration, body.Digest)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, a.view(r.Context(), rev))
}

func (a *API) approve(w http.ResponseWriter, r *http.Request, principal authz.Principal, pipelineID string, number int) {
	var body struct {
		ExpectedGeneration int `json:"expected_generation"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid", "The request body could not be read.")
		return
	}
	current, err := a.service.Get(r.Context(), pipelineID, number)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	rev, err := a.service.Approve(r.Context(), principal.ID, current.RevisionID, body.ExpectedGeneration)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	view := a.view(r.Context(), rev)
	if view.Deployed {
		writeError(w, http.StatusInternalServerError, "invalid", "Approval tried to deploy a pipeline.")
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func splitPipelinePath(path string) (string, []string) {
	const prefix = "/api/v1/pipelines/"
	if !strings.HasPrefix(path, prefix) {
		return "", nil
	}
	trimmed := strings.Trim(strings.TrimPrefix(path, prefix), "/")
	if trimmed == "" {
		return "", nil
	}
	parts := strings.Split(trimmed, "/")
	return parts[0], parts[1:]
}

type revisionView struct {
	PipelineID  string          `json:"pipeline_id"`
	RevisionID  string          `json:"revision_id"`
	Revision    int             `json:"revision"`
	State       string          `json:"state"`
	Mode        string          `json:"mode"`
	Generation  int             `json:"generation"`
	Document    json.RawMessage `json:"document"`
	HasSnapshot bool            `json:"has_snapshot"`
	ExportTOML  string          `json:"export_toml,omitempty"`
	ExportIssue string          `json:"export_issue,omitempty"`
	Deployed    bool            `json:"deployed"`
}

type historyView struct {
	Revision    int    `json:"revision"`
	State       string `json:"state"`
	Mode        string `json:"mode"`
	Generation  int    `json:"generation"`
	CreatedBy   string `json:"created_by"`
	ApprovedBy  string `json:"approved_by,omitempty"`
	HasSnapshot bool   `json:"has_snapshot"`
}

func historyItem(rev metadatastore.PipelineRevision) historyView {
	return historyView{
		Revision:    rev.Number,
		State:       rev.State,
		Mode:        rev.Mode,
		Generation:  rev.Generation,
		CreatedBy:   rev.CreatedBy,
		ApprovedBy:  rev.ApprovedBy,
		HasSnapshot: rev.SnapshotJSON != "",
	}
}

func (a *API) view(ctx context.Context, rev metadatastore.PipelineRevision) revisionView {
	view := revisionView{
		PipelineID:  rev.PipelineID,
		RevisionID:  rev.RevisionID,
		Revision:    rev.Number,
		State:       rev.State,
		Mode:        rev.Mode,
		Generation:  rev.Generation,
		Document:    json.RawMessage(rev.DocumentJSON),
		HasSnapshot: rev.SnapshotJSON != "",
		Deployed:    false,
	}
	doc, err := pipelinecontract.Parse([]byte(rev.DocumentJSON))
	if err != nil {
		view.ExportIssue = "The stored document could not be read."
		return view
	}
	switch doc.Mode {
	case pipelinecontract.ModeManual:
		if doc.Manual != nil {
			view.ExportTOML = doc.Manual.TOML
		}
	case pipelinecontract.ModeGraph:
		prepared, prepErr := a.service.PrepareGraph(ctx, doc)
		if prepErr != nil {
			var pipeline *pipelineservice.Error
			if errors.As(prepErr, &pipeline) {
				view.ExportIssue = pipeline.Message
			} else {
				view.ExportIssue = "The mapping could not be compiled."
			}
			return view
		}
		toml, genErr := pipelinecontract.Generate(prepared)
		if genErr != nil {
			view.ExportIssue = "The graph cannot be encoded without dropping configuration."
			return view
		}
		view.ExportTOML = toml
	}
	return view
}

func reviewView(review pipelineservice.Review) map[string]any {
	return map[string]any{
		"from_mode":     review.FromMode,
		"to_mode":       review.ToMode,
		"current_text":  review.CurrentText,
		"proposed_text": review.ProposedText,
		"digest":        review.Digest,
		"deployed":      false,
	}
}

func (a *API) principal(w http.ResponseWriter, r *http.Request) (authz.Principal, bool) {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, bearerPrefix) {
		writeError(w, http.StatusUnauthorized, "authentication-required", "A bearer token is required.")
		return authz.Principal{}, false
	}
	token := strings.TrimSpace(strings.TrimPrefix(header, bearerPrefix))
	principal, ok := a.authenticator.Authenticate(token)
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication-failed", "The supplied token is not valid.")
		return authz.Principal{}, false
	}
	return principal, true
}

func (a *API) authorize(w http.ResponseWriter, principal authz.Principal, perm authz.Permission) bool {
	err := a.authorizer.Authorize(principal, perm)
	if err == nil {
		return true
	}
	if errors.Is(err, authz.ErrUnauthenticated) {
		writeError(w, http.StatusUnauthorized, "authentication-required", "Authentication is required.")
		return false
	}
	writeError(w, http.StatusForbidden, "permission-denied", "The principal is not permitted to perform this action.")
	return false
}

func writeServiceError(w http.ResponseWriter, err error) {
	var pipeline *pipelineservice.Error
	if errors.As(err, &pipeline) {
		status := http.StatusBadRequest
		switch pipeline.Code {
		case pipelineservice.CodeNotFound:
			status = http.StatusNotFound
		case pipelineservice.CodeConflict, pipelineservice.CodeImmutable, pipelineservice.CodeConversionRefused:
			status = http.StatusConflict
		}
		writeError(w, status, pipeline.Code, pipeline.Message)
		return
	}
	writeError(w, http.StatusInternalServerError, "invalid", "The pipeline revision could not be saved.")
}

func decodeJSON(r *http.Request, target any) error {
	raw, err := io.ReadAll(http.MaxBytesReader(nil, r.Body, 1<<20))
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"code": code, "message": message})
}

func methodNotAllowed(w http.ResponseWriter, allowed ...string) {
	w.Header().Set("Allow", strings.Join(allowed, ", "))
	writeError(w, http.StatusMethodNotAllowed, "method-not-allowed", "The method is not allowed for this route.")
}
