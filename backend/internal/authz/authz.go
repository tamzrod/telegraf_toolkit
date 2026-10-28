// Package authz enforces backend authorization for Registry operations.
//
// Authorization is server-side and authoritative: the UI is never a security
// boundary (docs/DATABASE_REGISTRY.md, docs/contracts/registry.md). Every
// request resolves to a Principal, and a request with no authenticated
// principal is denied. There are no anonymously writable endpoints.
//
// Roles follow docs/DATABASE_REGISTRY.md: viewer, operator, configurator and
// administrator. Permissions are additive and least-privilege by default: a
// principal has no permission it was not granted.
package authz

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
)

// Role is a named set of permissions.
type Role string

// Registry roles, ordered from least to most privileged.
const (
	RoleViewer        Role = "viewer"
	RoleOperator      Role = "operator"
	RoleConfigurator  Role = "configurator"
	RoleAdministrator Role = "administrator"
)

// Permission is a single authorized action.
type Permission string

// Registry permissions.
const (
	PermProfileRead  Permission = "profile.read"
	PermProfileWrite Permission = "profile.write"
	PermProfileProbe Permission = "profile.probe"
	// PermSchemaRead authorizes the bounded, read-only metadata read of a
	// registered profile's actual schema. It is deliberately distinct from
	// profile.probe so it can be granted or revoked on its own.
	PermSchemaRead        Permission = "schema.read"
	PermCredentialRead    Permission = "credential.read"
	PermCredentialWrite   Permission = "credential.write"
	PermCredentialAttach  Permission = "credential.attach"
	PermAuditRead         Permission = "audit.read"
	PermCredentialResolve Permission = "credential.resolve"
	// P3-005 canonical schema and mapping profile permissions
	PermCanonicalRead  Permission = "canonical.read"
	PermCanonicalWrite Permission = "canonical.write"
	PermMappingRead    Permission = "mapping.read"
	PermMappingWrite   Permission = "mapping.write"
)

// rolePermissions maps a role to its granted permissions. The map is the single
// source of truth for the role model; it is deliberately explicit rather than
// derived, so a role cannot silently gain a permission.
//
// credential.attach is the authority to associate an existing credential
// reference with a profile endpoint/scope. It is deliberately administrator-only:
// a configurator can write profiles but must not be able to attach another
// principal's stored secret to a profile it controls (P2L-007).
var rolePermissions = map[Role]map[Permission]bool{
	RoleViewer: {
		PermProfileRead: true,
	},
	RoleOperator: {
		PermProfileRead:    true,
		PermProfileProbe:   true,
		PermSchemaRead:     true,
		PermCredentialRead: true,
	},
	RoleConfigurator: {
		PermProfileRead:    true,
		PermProfileWrite:   true,
		PermProfileProbe:   true,
		PermSchemaRead:     true,
		PermCredentialRead: true,
	},
	RoleAdministrator: {
		PermProfileRead:       true,
		PermProfileWrite:      true,
		PermProfileProbe:      true,
		PermSchemaRead:        true,
		PermCredentialRead:    true,
		PermCredentialWrite:   true,
		PermCredentialAttach:  true,
		PermAuditRead:         true,
		PermCredentialResolve: true,
		PermCanonicalRead:     true,
		PermCanonicalWrite:    true,
		PermMappingRead:       true,
		PermMappingWrite:      true,
	},
}

// KnownRoles returns the roles the model defines, sorted.
func KnownRoles() []Role {
	out := make([]Role, 0, len(rolePermissions))
	for role := range rolePermissions {
		out = append(out, role)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// ValidRole reports whether a role is defined.
func ValidRole(role Role) bool {
	_, ok := rolePermissions[role]
	return ok
}

// PermissionsFor returns the permissions a role grants.
func PermissionsFor(role Role) []Permission {
	granted := rolePermissions[role]
	out := make([]Permission, 0, len(granted))
	for perm := range granted {
		out = append(out, perm)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Principal is an authenticated actor with a set of roles.
type Principal struct {
	// ID is a stable, non-secret identity reference recorded in audit events.
	ID string
	// Roles are the roles granted to the principal.
	Roles []Role
}

// Can reports whether the principal holds a permission. An unknown role grants
// nothing.
func (p Principal) Can(perm Permission) bool {
	for _, role := range p.Roles {
		if rolePermissions[role][perm] {
			return true
		}
	}
	return false
}

// Anonymous reports whether the principal carries no identity.
func (p Principal) Anonymous() bool { return strings.TrimSpace(p.ID) == "" }

// NewPrincipal builds a principal, rejecting an empty identity and unknown
// roles so a caller cannot construct an unintended principal.
func NewPrincipal(id string, roles ...Role) (Principal, error) {
	if strings.TrimSpace(id) == "" {
		return Principal{}, errors.New("a principal requires a non-empty identity")
	}
	for _, role := range roles {
		if !ValidRole(role) {
			return Principal{}, fmt.Errorf("unknown role %q", role)
		}
	}
	return Principal{ID: id, Roles: append([]Role(nil), roles...)}, nil
}

// ErrUnauthenticated indicates no authenticated principal was supplied.
var ErrUnauthenticated = errors.New("authentication required")

// ErrForbidden indicates the principal lacks a required permission.
var ErrForbidden = errors.New("permission denied")

// Authorizer enforces permissions for principals.
type Authorizer struct{}

// NewAuthorizer returns the default authorizer.
func NewAuthorizer() *Authorizer { return &Authorizer{} }

// Authorize returns nil when the principal holds the permission, an
// unauthenticated error for an anonymous principal, and a forbidden error
// otherwise. Denial is the default: an unknown principal or an ungranted
// permission is refused.
func (a *Authorizer) Authorize(principal Principal, perm Permission) error {
	if principal.Anonymous() {
		return ErrUnauthenticated
	}
	if !principal.Can(perm) {
		return ErrForbidden
	}
	return nil
}

// Authenticator resolves an authentication token to a principal. A token that
// does not resolve yields an anonymous principal (and therefore denial), never a
// default privileged identity.
type Authenticator struct {
	mu     sync.RWMutex
	tokens map[string]Principal
}

// NewAuthenticator returns an empty authenticator that denies every token.
func NewAuthenticator() *Authenticator {
	return &Authenticator{tokens: map[string]Principal{}}
}

// RegisterToken maps a token to a principal. It rejects an empty token or an
// anonymous principal so no token can authenticate as "no one with access".
func (a *Authenticator) RegisterToken(token string, principal Principal) error {
	if strings.TrimSpace(token) == "" {
		return errors.New("a token must be non-empty")
	}
	if principal.Anonymous() {
		return errors.New("a principal must have an identity")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.tokens[token] = principal
	return nil
}

// Authenticate resolves a token to a principal. An unknown or empty token
// returns an anonymous principal and false, so the caller denies the request.
func (a *Authenticator) Authenticate(token string) (Principal, bool) {
	if strings.TrimSpace(token) == "" {
		return Principal{}, false
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	principal, ok := a.tokens[token]
	return principal, ok
}

// tokenFile is the on-disk JSON shape of an authentication token file. Tokens
// are secrets, so the file is provisioned outside the application with
// owner-only permissions and is never committed.
type tokenFile struct {
	Tokens []tokenEntry `json:"tokens"`
}

type tokenEntry struct {
	Token string   `json:"token"`
	Actor string   `json:"actor"`
	Roles []string `json:"roles"`
}

// LoadTokenFile reads and validates an authentication token file and returns a
// populated authenticator. It fails closed: a missing, group/other-readable,
// malformed or empty file yields an error and no authenticator, so the backend
// cannot start with an unintended or default identity.
func LoadTokenFile(path string) (*Authenticator, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("authentication token file path is required")
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat authentication token file: %w", err)
	}
	if info.IsDir() {
		return nil, errors.New("authentication token path is a directory, not a file")
	}
	if perm := info.Mode().Perm(); perm&0o077 != 0 {
		return nil, fmt.Errorf("authentication token file permissions %#o are too open; require 0600 (owner-only)", perm)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read authentication token file: %w", err)
	}
	var parsed tokenFile
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("parse authentication token file: %w", err)
	}
	if len(parsed.Tokens) == 0 {
		return nil, errors.New("authentication token file has no tokens")
	}
	authenticator := NewAuthenticator()
	for _, entry := range parsed.Tokens {
		roles := make([]Role, 0, len(entry.Roles))
		for _, role := range entry.Roles {
			roles = append(roles, Role(role))
		}
		principal, err := NewPrincipal(entry.Actor, roles...)
		if err != nil {
			return nil, fmt.Errorf("invalid token entry for actor %q: %w", entry.Actor, err)
		}
		if err := authenticator.RegisterToken(entry.Token, principal); err != nil {
			return nil, err
		}
	}
	return authenticator, nil
}
