// Package usecase holds the ports shared by several use case packages.
// Each use case package declares the rest of its own ports (interfaces it consumes);
// adapters implement them and are bound to them in their fx modules.
package usecase

import (
	"context"
	"errors"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
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

// RequireMember returns the user's role, or notFound if they aren't a member —
// outsiders can't tell a foreign project from a missing one.
func RequireMember(ctx context.Context, m Memberships, projectID, userID int64, notFound error) (domain.Role, error) {
	role, err := m.Role(ctx, projectID, userID)
	if err != nil {
		return "", err
	}
	if role == domain.RoleNone {
		return "", notFound
	}
	return role, nil
}

// RequireOwner is RequireMember plus domain.ErrOwnerOnly for non-owners.
func RequireOwner(ctx context.Context, m Memberships, projectID, userID int64, notFound error) error {
	role, err := RequireMember(ctx, m, projectID, userID, notFound)
	if err != nil {
		return err
	}
	if role != domain.RoleOwner {
		return domain.ErrOwnerOnly
	}
	return nil
}

// NotFoundAs converts a repository ErrNoRows into the given business error.
func NotFoundAs(err error, notFound error) error {
	if errors.Is(err, domain.ErrNoRows) {
		return notFound
	}
	return err
}

// BoardEvents broadcasts committed board changes to everyone viewing the board.
// Publishing is best-effort: the database is the source of truth.
type BoardEvents interface {
	Publish(ctx context.Context, e domain.BoardEvent)
}

type originKey struct{}

// WithOrigin marks the request as coming from a real-time connection (a browser tab), so board events
// caused by it can be skipped by that tab, which already shows the change, and applied by every other.
func WithOrigin(ctx context.Context, connID string) context.Context {
	return context.WithValue(ctx, originKey{}, connID)
}

// OriginFrom returns the connection id set by WithOrigin, or "".
func OriginFrom(ctx context.Context) string {
	s, _ := ctx.Value(originKey{}).(string)
	return s
}

// UserEvents notifies users (in every tab they have open) about changes to their access.
// Best-effort, like BoardEvents.
type UserEvents interface {
	PublishUser(ctx context.Context, e domain.UserEvent)
}
