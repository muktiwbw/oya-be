package task

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validBoardID() string {
	id, err := uuid.NewV7()
	if err != nil {
		panic(err)
	}
	return id.String()
}

func TestNormalizeCreate(t *testing.T) {
	boardID := validBoardID()
	got, err := NormalizeCreate(CreateInput{BoardID: boardID, Title: "  Buy milk  "})
	require.NoError(t, err)
	assert.Equal(t, CreateInput{BoardID: boardID, Title: "Buy milk", Status: Todo}, got)
	for _, status := range []Status{Todo, Doing, Done} {
		got, err := NormalizeCreate(CreateInput{BoardID: boardID, Title: "task", Description: "  unchanged  ", Status: status})
		require.NoError(t, err)
		assert.Equal(t, status, got.Status)
		assert.Equal(t, "  unchanged  ", got.Description)
	}
}

func TestCreateValidation(t *testing.T) {
	boardID := validBoardID()
	for _, tc := range []struct {
		name  string
		input CreateInput
		valid bool
	}{
		{"missing board_id", CreateInput{Title: "task"}, false},
		{"invalid board_id", CreateInput{BoardID: "invalid-uuid", Title: "task"}, false},
		{"missing title", CreateInput{BoardID: boardID}, false},
		{"whitespace title", CreateInput{BoardID: boardID, Title: " \t\n"}, false},
		{"unicode title boundary", CreateInput{BoardID: boardID, Title: strings.Repeat("界", 200)}, true},
		{"unicode title too long", CreateInput{BoardID: boardID, Title: strings.Repeat("界", 201)}, false},
		{"description boundary", CreateInput{BoardID: boardID, Title: "task", Description: strings.Repeat("界", 2000)}, true},
		{"description too long", CreateInput{BoardID: boardID, Title: "task", Description: strings.Repeat("界", 2001)}, false},
		{"unsupported status", CreateInput{BoardID: boardID, Title: "task", Status: "TODO"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NormalizeCreate(tc.input)
			if tc.valid {
				require.NoError(t, err)
			} else {
				var ve *ValidationError
				require.ErrorAs(t, err, &ve)
			}
		})
	}
}

func TestUpdateValidation(t *testing.T) {
	for _, s := range []Status{Todo, Doing, Done} {
		v := int64(1)
		require.NoError(t, (UpdateStatusInput{Status: s, ExpectedVersion: &v}).Validate())
	}
	for _, s := range []Status{"", "TODO", "blocked"} {
		require.Error(t, (UpdateStatusInput{Status: s}).Validate())
	}
	for _, v := range []int64{-1, 0} {
		require.Error(t, (UpdateStatusInput{Status: Doing, ExpectedVersion: &v}).Validate())
	}
	v := int64(1)
	require.NoError(t, (UpdateStatusInput{Status: Doing, ExpectedVersion: &v}).Validate())
}

func TestNormalizeUpdate(t *testing.T) {
	v := int64(1)
	title, description := "  revised  ", "# Markdown"
	got, err := NormalizeUpdate(UpdateTaskInput{Title: &title, Description: &description, ExpectedVersion: &v})
	require.NoError(t, err)
	assert.Equal(t, "revised", *got.Title)
	assert.Equal(t, "# Markdown", *got.Description)

	for _, in := range []UpdateTaskInput{
		{ExpectedVersion: &v},
		{Title: &title},
		{Title: ptr(" \t "), ExpectedVersion: &v},
		{Title: ptr(strings.Repeat("界", 201)), ExpectedVersion: &v},
		{Description: ptr(strings.Repeat("界", 2001)), ExpectedVersion: &v},
	} {
		require.Error(t, func() error { _, err := NormalizeUpdate(in); return err }())
	}
	empty := ""
	_, err = NormalizeUpdate(UpdateTaskInput{Description: &empty, ExpectedVersion: &v})
	require.NoError(t, err)
}

func ptr(s string) *string { return &s }

func TestTimestampNormalization(t *testing.T) {
	input := time.Date(2026, 10, 16, 20, 0, 0, 123456789, time.FixedZone("WIB", 7*3600))
	got := NormalizeTime(input)
	assert.Equal(t, time.UTC, got.Location())
	assert.Equal(t, 123000000, got.Nanosecond())
	assert.Equal(t, 13, got.Hour())
	assert.Equal(t, "2026-10-16T13:00:00.123Z", FormatTime(got))
	assert.Equal(t, "2026-01-01T00:00:00.000Z", FormatTime(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)))
}
