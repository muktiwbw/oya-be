package board

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateInputValidate(t *testing.T) {
	for _, tc := range []struct {
		name    string
		input   CreateInput
		wantErr bool
	}{
		{"valid", CreateInput{Title: "Board 1", Description: "Description"}, false},
		{"empty title", CreateInput{Title: "   "}, true},
		{"title too long", CreateInput{Title: strings.Repeat("a", 201)}, true},
		{"description too long", CreateInput{Title: "Board", Description: strings.Repeat("b", 2001)}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.input.Validate()
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, strings.TrimSpace(tc.input.Title), got.Title)
			}
		})
	}
}

func TestUpdateInputValidate(t *testing.T) {
	emptyTitle := ""
	validTitle := "Updated"
	tooLongTitle := strings.Repeat("x", 201)

	for _, tc := range []struct {
		name    string
		input   UpdateInput
		wantErr bool
	}{
		{"nil both", UpdateInput{}, true},
		{"valid title", UpdateInput{Title: &validTitle}, false},
		{"empty title", UpdateInput{Title: &emptyTitle}, true},
		{"too long title", UpdateInput{Title: &tooLongTitle}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.input.Validate()
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
