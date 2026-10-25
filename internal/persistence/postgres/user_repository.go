package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"oya-be/internal/persistence/postgres/sqlc"
	"oya-be/internal/user"
)

var _ user.Repository = (*Store)(nil)

func toUser(id int64, publicID, email, subject, displayName string, createdAt, updatedAt time.Time, deletedAt *time.Time) user.User {
	return user.User{ID: id, PublicID: publicID, Email: email, GoogleSubject: subject, DisplayName: displayName, CreatedAt: createdAt, UpdatedAt: updatedAt, DeletedAt: deletedAt}
}

func (s *Store) GetUserByGoogleSubject(ctx context.Context, subject string) (user.User, error) {
	row, err := s.queries.GetUserByGoogleSubject(ctx, subject)
	if errors.Is(err, pgx.ErrNoRows) {
		return user.User{}, user.ErrUserNotFound
	}
	if err != nil {
		return user.User{}, err
	}
	return toUser(row.ID, pgUUIDToString(row.PublicID), row.Email, row.GoogleSubject, row.DisplayName, row.CreatedAt.Time, row.UpdatedAt.Time, nil), nil
}

func (s *Store) GetUserByID(ctx context.Context, id int64) (user.User, error) {
	row, err := s.queries.GetUserByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return user.User{}, user.ErrUserNotFound
	}
	if err != nil {
		return user.User{}, err
	}
	return toUser(row.ID, pgUUIDToString(row.PublicID), row.Email, row.GoogleSubject, row.DisplayName, row.CreatedAt.Time, row.UpdatedAt.Time, nil), nil
}

func (s *Store) CreateGoogleUser(ctx context.Context, u user.User) (user.User, error) {
	publicID, err := stringToPgUUID(u.PublicID)
	if err != nil {
		return user.User{}, err
	}
	row, err := s.queries.CreateGoogleUser(ctx, sqlc.CreateGoogleUserParams{PublicID: publicID, Email: u.Email, GoogleSubject: u.GoogleSubject, DisplayName: u.DisplayName})
	if err != nil {
		return user.User{}, err
	}
	return toUser(row.ID, pgUUIDToString(row.PublicID), row.Email, row.GoogleSubject, row.DisplayName, row.CreatedAt.Time, row.UpdatedAt.Time, nil), nil
}

func (s *Store) IsUserWhitelisted(ctx context.Context, id int64) (bool, error) {
	return s.queries.IsUserWhitelisted(ctx, id)
}
func (s *Store) IsEmailWhitelisted(ctx context.Context, email string) (bool, error) {
	return s.queries.IsEmailWhitelisted(ctx, email)
}

func (s *Store) CreateSession(ctx context.Context, in user.AuthSession) error {
	_, err := s.queries.CreateAuthSession(ctx, sqlc.CreateAuthSessionParams{UserID: in.UserID, TokenHash: in.TokenHash, ExpiresAt: timeToTimestamptz(in.ExpiresAt)})
	return err
}

func (s *Store) GetSession(ctx context.Context, hash string) (user.AuthSession, error) {
	row, err := s.queries.GetAuthSessionByTokenHash(ctx, hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return user.AuthSession{}, user.ErrInvalidSession
	}
	if err != nil {
		return user.AuthSession{}, err
	}
	var revokedAt *time.Time
	if row.RevokedAt.Valid {
		value := row.RevokedAt.Time
		revokedAt = &value
	}
	return user.AuthSession{UserID: row.UserID, TokenHash: row.TokenHash, ExpiresAt: row.ExpiresAt.Time, RevokedAt: revokedAt}, nil
}

func (s *Store) RotateSession(ctx context.Context, previousHash string, next user.AuthSession, now time.Time) error {
	_, err := s.queries.RotateAuthSession(ctx, sqlc.RotateAuthSessionParams{
		PreviousTokenHash: previousHash,
		NewTokenHash:      next.TokenHash,
		NewExpiresAt:      timeToTimestamptz(next.ExpiresAt),
		Now:               timeToTimestamptz(now),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return user.ErrInvalidSession
	}
	return err
}

func (s *Store) RevokeSession(ctx context.Context, hash string) error {
	return s.queries.RevokeAuthSessionByTokenHash(ctx, hash)
}

func (s *Store) DeleteExpiredSessions(ctx context.Context, now time.Time) error {
	return s.queries.DeleteExpiredAuthSessions(ctx, timeToTimestamptz(now))
}

func (s *Store) CreateOAuthTransaction(ctx context.Context, in user.OAuthTransaction) error {
	_, err := s.queries.CreateOAuthTransaction(ctx, sqlc.CreateOAuthTransactionParams{State: in.State, Nonce: in.Nonce, CodeVerifier: in.CodeVerifier, ExpiresAt: timeToTimestamptz(in.ExpiresAt)})
	return err
}

func (s *Store) ConsumeOAuthTransaction(ctx context.Context, state string, now time.Time) (user.OAuthTransaction, error) {
	row, err := s.queries.ConsumeOAuthTransaction(ctx, sqlc.ConsumeOAuthTransactionParams{State: state, ExpiresAt: timeToTimestamptz(now)})
	if errors.Is(err, pgx.ErrNoRows) {
		return user.OAuthTransaction{}, user.ErrAccessDenied
	}
	if err != nil {
		return user.OAuthTransaction{}, err
	}
	return user.OAuthTransaction{State: row.State, Nonce: row.Nonce, CodeVerifier: row.CodeVerifier, ExpiresAt: row.ExpiresAt.Time}, nil
}

func (s *Store) DeleteExpiredOAuthTransactions(ctx context.Context, now time.Time) error {
	return s.queries.DeleteExpiredOAuthTransactions(ctx, timeToTimestamptz(now))
}
