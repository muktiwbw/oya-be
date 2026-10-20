package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"oya-be/internal/persistence/postgres/sqlc"
	"oya-be/internal/user"
)

var _ user.Repository = (*Store)(nil)

func toUserDomain(u sqlc.User) user.User {
	var deletedAt *pgtype.Timestamptz
	if u.DeletedAt.Valid {
		deletedAt = &u.DeletedAt
	}
	var delPtr *user.User
	_ = delPtr

	res := user.User{
		ID:          u.ID,
		PublicID:    pgUUIDToString(u.PublicID),
		Email:       u.Email,
		Username:    u.Username,
		Password:    u.Password,
		DisplayName: u.DisplayName,
		CreatedAt:   u.CreatedAt.Time,
		UpdatedAt:   u.UpdatedAt.Time,
	}
	if deletedAt != nil {
		t := deletedAt.Time
		res.DeletedAt = &t
	}
	return res
}

func (s *Store) GetUserByUsername(ctx context.Context, username string) (user.User, error) {
	u, err := s.queries.GetUserByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return user.User{}, user.ErrUserNotFound
		}
		return user.User{}, err
	}
	return toUserDomain(u), nil
}

func (s *Store) GetUserByPublicID(ctx context.Context, publicID string) (user.User, error) {
	uuidVal, err := stringToPgUUID(publicID)
	if err != nil {
		return user.User{}, user.ErrUserNotFound
	}
	u, err := s.queries.GetUserByPublicID(ctx, uuidVal)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return user.User{}, user.ErrUserNotFound
		}
		return user.User{}, err
	}
	return toUserDomain(u), nil
}

func (s *Store) GetUserByID(ctx context.Context, id int64) (user.User, error) {
	u, err := s.queries.GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return user.User{}, user.ErrUserNotFound
		}
		return user.User{}, err
	}
	return toUserDomain(u), nil
}

func (s *Store) CreateUser(ctx context.Context, u user.User) (user.User, error) {
	uuidVal, err := stringToPgUUID(u.PublicID)
	if err != nil {
		return user.User{}, err
	}
	created, err := s.queries.CreateUser(ctx, sqlc.CreateUserParams{
		PublicID:    uuidVal,
		Email:       u.Email,
		Username:    u.Username,
		Password:    u.Password,
		DisplayName: u.DisplayName,
	})
	if err != nil {
		return user.User{}, err
	}
	return toUserDomain(created), nil
}
