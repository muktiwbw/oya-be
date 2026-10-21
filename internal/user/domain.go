package user

import (
	"context"
	"errors"
	"time"
)

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrInvalidCredentials = errors.New("invalid credentials")
)

type User struct {
	ID          int64
	PublicID    string
	Email       string
	Username    string
	Password    string
	DisplayName string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

type Repository interface {
	GetUserByUsername(ctx context.Context, username string) (User, error)
	GetUserByPublicID(ctx context.Context, publicID string) (User, error)
	GetUserByID(ctx context.Context, id int64) (User, error)
	CreateUser(ctx context.Context, u User) (User, error)
}
