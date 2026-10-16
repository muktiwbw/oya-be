package task

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCursorRoundTrip(t *testing.T) {
	p := Position{CreatedAt: time.Date(2026, 10, 16, 0, 0, 0, 123000000, time.UTC), PublicID: "018f4b7e-3b75-7e1f-b5d5-2b2d7b0a7f12"}
	token, err := EncodeCursor(p, Todo)
	require.NoError(t, err)
	got, err := DecodeCursor(token, Todo)
	require.NoError(t, err)
	assert.Equal(t, p, got)
	_, err = DecodeCursor(token, Doing)
	require.ErrorIs(t, err, ErrInvalidCursor)
}

func TestInvalidCursors(t *testing.T) {
	for _, token := range []string{"", "!bad", "e30", "bnVsbA"} {
		_, err := DecodeCursor(token, "")
		require.ErrorIs(t, err, ErrInvalidCursor, "cursor %q", token)
	}
}
