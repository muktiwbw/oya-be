package user

import (
	"context"
	"errors"
	"time"
)

var (
	ErrUserNotFound   = errors.New("user not found")
	ErrAccessDenied   = errors.New("access denied")
	ErrInvalidSession = errors.New("invalid session")
	ErrGoogleDisabled = errors.New("google sign-in is not configured")
)

type User struct {
	ID            int64
	PublicID      string
	Email         string
	GoogleSubject string
	DisplayName   string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	DeletedAt     *time.Time
}

type AuthSession struct {
	UserID    int64
	TokenHash string
	ExpiresAt time.Time
	RevokedAt *time.Time
}

type OAuthTransaction struct {
	State        string
	Nonce        string
	CodeVerifier string
	ExpiresAt    time.Time
}

type GoogleIdentity struct {
	Subject       string
	Email         string
	EmailVerified bool
	DisplayName   string
}

type GoogleProvider interface {
	AuthorizationURL(state, nonce, codeVerifier string) string
	Exchange(context.Context, string, string, string) (GoogleIdentity, error)
}

type Repository interface {
	GetUserByGoogleSubject(context.Context, string) (User, error)
	GetUserByID(context.Context, int64) (User, error)
	CreateGoogleUser(context.Context, User) (User, error)
	IsUserWhitelisted(context.Context, int64) (bool, error)
	IsEmailWhitelisted(context.Context, string) (bool, error)
	CreateSession(context.Context, AuthSession) error
	GetSession(context.Context, string) (AuthSession, error)
	RotateSession(context.Context, string, AuthSession, time.Time) error
	RevokeSession(context.Context, string) error
	DeleteExpiredSessions(context.Context, time.Time) error
	CreateOAuthTransaction(context.Context, OAuthTransaction) error
	ConsumeOAuthTransaction(context.Context, string, time.Time) (OAuthTransaction, error)
	DeleteExpiredOAuthTransactions(context.Context, time.Time) error
}
