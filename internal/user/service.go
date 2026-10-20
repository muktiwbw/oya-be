package user

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

type TokenIssuer interface {
	Generate(userID int64, userPublicID, username string) (string, error)
}

type Service struct {
	repo   Repository
	tokens TokenIssuer
}

func NewService(repo Repository, tokens TokenIssuer) *Service {
	return &Service{
		repo:   repo,
		tokens: tokens,
	}
}

func (s *Service) Login(ctx context.Context, username, password string) (string, User, error) {
	u, err := s.repo.GetUserByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return "", User{}, ErrInvalidCredentials
		}
		return "", User{}, fmt.Errorf("lookup user: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)); err != nil {
		return "", User{}, ErrInvalidCredentials
	}

	token, err := s.tokens.Generate(u.ID, u.PublicID, u.Username)
	if err != nil {
		return "", User{}, fmt.Errorf("generate token: %w", err)
	}

	return token, u, nil
}
