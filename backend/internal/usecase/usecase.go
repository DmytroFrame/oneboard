// Package usecase holds the ports shared by several use case packages.
// Each use case package declares the rest of its own ports (interfaces it consumes);
// adapters implement them and are bound to them in their fx modules.
package usecase

import (
	"context"
	"errors"

	"clodapp/backend/internal/domain"
)

// Transactor runs fn in a single database transaction. Repositories called with the
// ctx passed to fn take part in that transaction.
type Transactor interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// Memberships answers "what is this user's role in this project".
type Memberships interface {
	Role(ctx context.Context, projectID, userID int64) (domain.Role, error)
}

// RequireMember returns the user's role, or NotFound(msg) if they aren't a member —
// outsiders can't tell a foreign project from a missing one.
func RequireMember(ctx context.Context, m Memberships, projectID, userID int64, msg string) (domain.Role, error) {
	role, err := m.Role(ctx, projectID, userID)
	if err != nil {
		return "", err
	}
	if role == domain.RoleNone {
		return "", domain.NotFound(msg)
	}
	return role, nil
}

// RequireOwner is RequireMember plus Forbidden(forbiddenMsg) for non-owners.
func RequireOwner(ctx context.Context, m Memberships, projectID, userID int64, notFoundMsg, forbiddenMsg string) error {
	role, err := RequireMember(ctx, m, projectID, userID, notFoundMsg)
	if err != nil {
		return err
	}
	if role != domain.RoleOwner {
		return domain.Forbidden(forbiddenMsg)
	}
	return nil
}

// NotFoundAs converts a repository ErrNoRows into a user-facing NotFound.
func NotFoundAs(err error, msg string) error {
	if errors.Is(err, domain.ErrNoRows) {
		return domain.NotFound(msg)
	}
	return err
}
