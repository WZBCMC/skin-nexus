// Package auth defines identity sources. SkinNexus separates "where an
// identity comes from" from sessions it issues, so phase-2 auth sources
// (Microsoft OAuth binding) slot in behind a feature flag without touching
// the Yggdrasil layer (blueprint §5 decision 2).
package auth

import (
	"context"
	"errors"
)

// ErrBadCredentials is returned when verification fails. Callers must not
// distinguish "user not found" from "wrong password" at the API boundary.
var ErrBadCredentials = errors.New("auth: invalid credentials")

// Identity is the canonical user record an AuthSource resolves to.
type Identity struct {
	UserID   int64
	Email    string // may be empty (legacy accounts)
	Nickname string
}

// Credentials is phase-1's only credential shape (password login).
type Credentials struct {
	Identifier string // email or nickname
	Password   string
}

// AuthSource resolves credentials to an Identity. Implementations:
//   - PasswordSource (phase 1, in-tree)
//   - MicrosoftSource (phase 2, feature flag `auth.microsoft.enabled`)
type AuthSource interface {
	// Name is the stable config/feature-flag key, e.g. "password", "microsoft".
	Name() string
	// Enabled reports whether the source is active (feature flag).
	Enabled() bool
	// Authenticate resolves credentials or returns ErrBadCredentials.
	Authenticate(ctx context.Context, cred Credentials) (Identity, error)
}

// Registry holds the ordered source list. The first enabled source that
// authenticates wins; failed sources fall through.
type Registry struct {
	sources []AuthSource
}

func NewRegistry(sources ...AuthSource) *Registry {
	return &Registry{sources: sources}
}

func (r *Registry) Authenticate(ctx context.Context, cred Credentials) (Identity, error) {
	for _, s := range r.sources {
		if !s.Enabled() {
			continue
		}
		id, err := s.Authenticate(ctx, cred)
		if err == nil {
			return id, nil
		}
		if !errors.Is(err, ErrBadCredentials) {
			return Identity{}, err
		}
	}
	return Identity{}, ErrBadCredentials
}
