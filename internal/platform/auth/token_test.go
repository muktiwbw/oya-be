package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenManager_GenerateAndValidate(t *testing.T) {
	tm := NewTokenManager("test-secret-key-that-is-long-enough", time.Hour)
	token, err := tm.Generate(42, "user-uuid-123")
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	claims, err := tm.Validate(token)
	require.NoError(t, err)
	assert.Equal(t, int64(42), claims.UserID)
	assert.Equal(t, "user-uuid-123", claims.UserPublicID)
	assert.NotEmpty(t, claims.ID)
}

func TestTokenManager_ExpiredToken(t *testing.T) {
	tm := NewTokenManager("test-secret-key-that-is-long-enough", -time.Hour)
	token, err := tm.Generate(42, "user-uuid-123")
	require.NoError(t, err)

	_, err = tm.Validate(token)
	require.ErrorIs(t, err, ErrInvalidToken)
}

func TestTokenManager_InvalidSecret(t *testing.T) {
	tm1 := NewTokenManager("secret-one-1234567890", time.Hour)
	tm2 := NewTokenManager("secret-two-1234567890", time.Hour)

	token, err := tm1.Generate(42, "user-uuid-123")
	require.NoError(t, err)

	_, err = tm2.Validate(token)
	require.ErrorIs(t, err, ErrInvalidToken)
}
