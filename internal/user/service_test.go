package user_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"oya-be/internal/user"
)

type mockUserRepo struct {
	users map[string]user.User
}

func (m *mockUserRepo) GetUserByUsername(ctx context.Context, username string) (user.User, error) {
	if u, ok := m.users[username]; ok {
		return u, nil
	}
	return user.User{}, user.ErrUserNotFound
}

func (m *mockUserRepo) GetUserByPublicID(ctx context.Context, publicID string) (user.User, error) {
	for _, u := range m.users {
		if u.PublicID == publicID {
			return u, nil
		}
	}
	return user.User{}, user.ErrUserNotFound
}

func (m *mockUserRepo) GetUserByID(ctx context.Context, id int64) (user.User, error) {
	for _, u := range m.users {
		if u.ID == id {
			return u, nil
		}
	}
	return user.User{}, user.ErrUserNotFound
}

func (m *mockUserRepo) CreateUser(ctx context.Context, u user.User) (user.User, error) {
	m.users[u.Username] = u
	return u, nil
}

type mockTokens struct{}

func (mockTokens) Generate(userID int64, userPublicID, username string) (string, error) {
	return "mock-jwt-token-" + username, nil
}

func TestUserService_Login(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("secret123"), bcrypt.DefaultCost)
	require.NoError(t, err)

	repo := &mockUserRepo{
		users: map[string]user.User{
			"cosmicg": {
				ID:          1,
				PublicID:    "user-uuid-1",
				Username:    "cosmicg",
				Password:    string(hash),
				DisplayName: "Cosmic G",
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
		},
	}

	svc := user.NewService(repo, mockTokens{})

	// Successful login
	token, u, err := svc.Login(context.Background(), "cosmicg", "secret123")
	require.NoError(t, err)
	assert.Equal(t, "mock-jwt-token-cosmicg", token)
	assert.Equal(t, "user-uuid-1", u.PublicID)

	// Invalid password
	_, _, err = svc.Login(context.Background(), "cosmicg", "wrongpass")
	require.ErrorIs(t, err, user.ErrInvalidCredentials)

	// Unknown user
	_, _, err = svc.Login(context.Background(), "unknown", "secret123")
	require.ErrorIs(t, err, user.ErrInvalidCredentials)
}
