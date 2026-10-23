package user

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type TokenIssuer interface {
	Generate(userID int64, userPublicID string) (string, error)
}

type Service struct {
	repo       Repository
	tokens     TokenIssuer
	google     GoogleProvider
	refreshTTL time.Duration
}

func NewService(repo Repository, tokens TokenIssuer, google GoogleProvider, refreshTTL time.Duration) *Service {
	return &Service{repo: repo, tokens: tokens, google: google, refreshTTL: refreshTTL}
}

func (s *Service) StartGoogle(ctx context.Context) (string, error) {
	if s.google == nil {
		return "", ErrGoogleDisabled
	}
	if err := s.repo.DeleteExpiredOAuthTransactions(ctx, time.Now().UTC()); err != nil {
		return "", fmt.Errorf("delete expired authorization transactions: %w", err)
	}
	state, err := randomToken(32)
	if err != nil {
		return "", err
	}
	nonce, err := randomToken(32)
	if err != nil {
		return "", err
	}
	verifier, err := randomToken(48)
	if err != nil {
		return "", err
	}
	if err := s.repo.CreateOAuthTransaction(ctx, OAuthTransaction{State: state, Nonce: nonce, CodeVerifier: verifier, ExpiresAt: time.Now().UTC().Add(10 * time.Minute)}); err != nil {
		return "", err
	}
	return s.google.AuthorizationURL(state, nonce, verifier), nil
}

func (s *Service) CompleteGoogle(ctx context.Context, state, code string) (string, string, User, error) {
	if s.google == nil {
		return "", "", User{}, ErrGoogleDisabled
	}
	txn, err := s.repo.ConsumeOAuthTransaction(ctx, state, time.Now().UTC())
	if err != nil {
		return "", "", User{}, ErrAccessDenied
	}
	identity, err := s.google.Exchange(ctx, code, txn.CodeVerifier, txn.Nonce)
	if err != nil || !identity.EmailVerified || strings.TrimSpace(identity.Subject) == "" {
		return "", "", User{}, ErrAccessDenied
	}
	email := strings.ToLower(strings.TrimSpace(identity.Email))
	allowed, err := s.repo.IsEmailWhitelisted(ctx, email)
	if err != nil {
		return "", "", User{}, err
	}
	if !allowed {
		return "", "", User{}, ErrAccessDenied
	}
	u, err := s.repo.GetUserByGoogleSubject(ctx, identity.Subject)
	if err != nil {
		if !errors.Is(err, ErrUserNotFound) {
			return "", "", User{}, err
		}
		publicID, idErr := uuid.NewV7()
		if idErr != nil {
			return "", "", User{}, idErr
		}
		u, err = s.repo.CreateGoogleUser(ctx, User{PublicID: publicID.String(), Email: email, GoogleSubject: identity.Subject, DisplayName: strings.TrimSpace(identity.DisplayName)})
		if err != nil {
			return "", "", User{}, err
		}
	}
	access, refresh, _, err := s.createSession(ctx, u)
	return access, refresh, u, err
}

func (s *Service) Refresh(ctx context.Context, raw string) (string, string, User, error) {
	hash := hashToken(raw)
	session, err := s.repo.GetSession(ctx, hash)
	if err != nil || session.RevokedAt != nil || !session.ExpiresAt.After(time.Now().UTC()) {
		return "", "", User{}, ErrInvalidSession
	}
	u, err := s.repo.GetUserByID(ctx, session.UserID)
	if err != nil {
		return "", "", User{}, ErrInvalidSession
	}
	allowed, err := s.repo.IsUserWhitelisted(ctx, u.ID)
	if err != nil || !allowed {
		return "", "", User{}, ErrAccessDenied
	}
	refresh, err := randomToken(48)
	if err != nil {
		return "", "", User{}, err
	}
	access, err := s.tokens.Generate(u.ID, u.PublicID)
	if err != nil {
		return "", "", User{}, fmt.Errorf("generate access token: %w", err)
	}
	if err := s.repo.RotateSession(ctx, hash, AuthSession{UserID: u.ID, TokenHash: hashToken(refresh), ExpiresAt: time.Now().UTC().Add(s.refreshTTL)}, time.Now().UTC()); err != nil {
		return "", "", User{}, err
	}
	return access, refresh, u, nil
}

func (s *Service) Logout(ctx context.Context, userID int64, raw string) error {
	session, err := s.repo.GetSession(ctx, hashToken(raw))
	if err != nil || session.UserID != userID {
		return ErrInvalidSession
	}
	return s.repo.RevokeSession(ctx, session.TokenHash)
}

func (s *Service) GetUser(ctx context.Context, id int64) (User, error) {
	return s.repo.GetUserByID(ctx, id)
}
func (s *Service) IsWhitelisted(ctx context.Context, id int64) (bool, error) {
	return s.repo.IsUserWhitelisted(ctx, id)
}

func (s *Service) createSession(ctx context.Context, u User) (string, string, User, error) {
	if err := s.repo.DeleteExpiredSessions(ctx, time.Now().UTC()); err != nil {
		return "", "", User{}, fmt.Errorf("delete expired sessions: %w", err)
	}
	refresh, err := randomToken(48)
	if err != nil {
		return "", "", User{}, err
	}
	if err := s.repo.CreateSession(ctx, AuthSession{UserID: u.ID, TokenHash: hashToken(refresh), ExpiresAt: time.Now().UTC().Add(s.refreshTTL)}); err != nil {
		return "", "", User{}, err
	}
	access, err := s.tokens.Generate(u.ID, u.PublicID)
	if err != nil {
		return "", "", User{}, fmt.Errorf("generate access token: %w", err)
	}
	return access, refresh, u, nil
}

func randomToken(size int) (string, error) {
	b := make([]byte, size)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
