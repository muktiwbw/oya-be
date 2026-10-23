package user_test

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"oya-be/internal/user"
)

type repository struct {
	allowed      bool
	users        map[string]user.User
	sessions     map[string]user.AuthSession
	transactions map[string]user.OAuthTransaction
}

func newRepository(allowed bool) *repository {
	return &repository{allowed: allowed, users: map[string]user.User{}, sessions: map[string]user.AuthSession{}, transactions: map[string]user.OAuthTransaction{}}
}

func (r *repository) GetUserByGoogleSubject(_ context.Context, subject string) (user.User, error) {
	u, ok := r.users[subject]
	if !ok {
		return user.User{}, user.ErrUserNotFound
	}
	return u, nil
}

func (r *repository) GetUserByID(_ context.Context, id int64) (user.User, error) {
	for _, u := range r.users {
		if u.ID == id {
			return u, nil
		}
	}
	return user.User{}, user.ErrUserNotFound
}

func (r *repository) CreateGoogleUser(_ context.Context, u user.User) (user.User, error) {
	u.ID = int64(len(r.users) + 1)
	r.users[u.GoogleSubject] = u
	return u, nil
}

func (r *repository) IsUserWhitelisted(context.Context, int64) (bool, error) { return r.allowed, nil }
func (r *repository) IsEmailWhitelisted(context.Context, string) (bool, error) {
	return r.allowed, nil
}

func (r *repository) CreateSession(_ context.Context, session user.AuthSession) error {
	r.sessions[session.TokenHash] = session
	return nil
}

func (r *repository) GetSession(_ context.Context, hash string) (user.AuthSession, error) {
	session, ok := r.sessions[hash]
	if !ok {
		return user.AuthSession{}, user.ErrInvalidSession
	}
	return session, nil
}

func (r *repository) RotateSession(_ context.Context, previousHash string, next user.AuthSession, now time.Time) error {
	previous, ok := r.sessions[previousHash]
	if !ok || previous.RevokedAt != nil || !previous.ExpiresAt.After(now) {
		return user.ErrInvalidSession
	}
	previous.RevokedAt = &now
	r.sessions[previousHash] = previous
	r.sessions[next.TokenHash] = next
	return nil
}

func (r *repository) RevokeSession(_ context.Context, hash string) error {
	session, ok := r.sessions[hash]
	if !ok {
		return user.ErrInvalidSession
	}
	now := time.Now().UTC()
	session.RevokedAt = &now
	r.sessions[hash] = session
	return nil
}

func (r *repository) DeleteExpiredSessions(context.Context, time.Time) error { return nil }

func (r *repository) CreateOAuthTransaction(_ context.Context, transaction user.OAuthTransaction) error {
	r.transactions[transaction.State] = transaction
	return nil
}

func (r *repository) ConsumeOAuthTransaction(_ context.Context, state string, now time.Time) (user.OAuthTransaction, error) {
	transaction, ok := r.transactions[state]
	if !ok || !transaction.ExpiresAt.After(now) {
		return user.OAuthTransaction{}, user.ErrAccessDenied
	}
	delete(r.transactions, state)
	return transaction, nil
}

func (r *repository) DeleteExpiredOAuthTransactions(context.Context, time.Time) error { return nil }

type tokens struct{}

func (tokens) Generate(int64, string) (string, error) { return "access", nil }

type googleProvider struct{ identity user.GoogleIdentity }

func (p googleProvider) AuthorizationURL(state, _, _ string) string {
	return "https://accounts.google.test/auth?state=" + url.QueryEscape(state)
}

func (p googleProvider) Exchange(context.Context, string, string, string) (user.GoogleIdentity, error) {
	return p.identity, nil
}

func TestGoogleDisabled(t *testing.T) {
	svc := user.NewService(newRepository(true), tokens{}, nil, time.Hour)
	_, err := svc.StartGoogle(context.Background())
	require.ErrorIs(t, err, user.ErrGoogleDisabled)
}

func TestCompleteGoogleCreatesSessionForWhitelistedEmail(t *testing.T) {
	repository := newRepository(true)
	svc := user.NewService(repository, tokens{}, googleProvider{identity: user.GoogleIdentity{Subject: "google-subject", Email: "Trusted@Example.com", EmailVerified: true, DisplayName: "Trusted User"}}, time.Hour)

	_, err := svc.StartGoogle(context.Background())
	require.NoError(t, err)
	var state string
	for state = range repository.transactions {
		break
	}

	access, refresh, signedInUser, err := svc.CompleteGoogle(context.Background(), state, "authorization-code")
	require.NoError(t, err)
	require.Equal(t, "access", access)
	require.NotEmpty(t, refresh)
	require.Equal(t, "trusted@example.com", signedInUser.Email)
	require.Equal(t, "google-subject", signedInUser.GoogleSubject)
	require.Len(t, repository.sessions, 1)
}

func TestCompleteGoogleRejectsNonWhitelistedEmail(t *testing.T) {
	repository := newRepository(false)
	svc := user.NewService(repository, tokens{}, googleProvider{identity: user.GoogleIdentity{Subject: "google-subject", Email: "denied@example.com", EmailVerified: true}}, time.Hour)
	_, err := svc.StartGoogle(context.Background())
	require.NoError(t, err)
	var state string
	for state = range repository.transactions {
		break
	}

	_, _, _, err = svc.CompleteGoogle(context.Background(), state, "authorization-code")
	require.ErrorIs(t, err, user.ErrAccessDenied)
	require.Empty(t, repository.users)
	require.Empty(t, repository.sessions)
}

func TestRefreshRejectsRevokedSessionAfterWhitelistIsDisabled(t *testing.T) {
	repository := newRepository(true)
	svc := user.NewService(repository, tokens{}, googleProvider{identity: user.GoogleIdentity{Subject: "google-subject", Email: "trusted@example.com", EmailVerified: true}}, time.Hour)
	access, refresh, _, err := svc.Refresh(context.Background(), "missing")
	require.ErrorIs(t, err, user.ErrInvalidSession)
	require.Empty(t, access)
	require.Empty(t, refresh)

	_, err = svc.StartGoogle(context.Background())
	require.NoError(t, err)
	var state string
	for state = range repository.transactions {
		break
	}
	_, refresh, _, err = svc.CompleteGoogle(context.Background(), state, "authorization-code")
	require.NoError(t, err)
	repository.allowed = false
	_, _, _, err = svc.Refresh(context.Background(), refresh)
	require.ErrorIs(t, err, user.ErrAccessDenied)
}
