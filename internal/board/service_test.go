package board_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"oya-be/internal/board"
)

type mockBoardRepo struct {
	boards map[string]board.Board
}

func (m *mockBoardRepo) CreateBoard(ctx context.Context, b board.Board) (board.Board, error) {
	b.ID = int64(len(m.boards) + 1)
	m.boards[b.PublicID] = b
	return b, nil
}

func (m *mockBoardRepo) GetBoardByPublicID(ctx context.Context, publicID string, userID int64) (board.Board, error) {
	b, ok := m.boards[publicID]
	if !ok || b.UserID != userID || b.DeletedAt != nil {
		return board.Board{}, board.ErrBoardNotFound
	}
	return b, nil
}

func (m *mockBoardRepo) GetBoardByPublicIDAnyUser(ctx context.Context, publicID string) (board.Board, error) {
	b, ok := m.boards[publicID]
	if !ok || b.DeletedAt != nil {
		return board.Board{}, board.ErrBoardNotFound
	}
	return b, nil
}

func (m *mockBoardRepo) ListBoardsByUserID(ctx context.Context, userID int64) ([]board.Board, error) {
	var res []board.Board
	for _, b := range m.boards {
		if b.UserID == userID && b.DeletedAt == nil {
			res = append(res, b)
		}
	}
	return res, nil
}

func (m *mockBoardRepo) UpdateBoard(ctx context.Context, publicID string, userID int64, title, description string) (board.Board, error) {
	b, ok := m.boards[publicID]
	if !ok || b.UserID != userID || b.DeletedAt != nil {
		return board.Board{}, board.ErrBoardNotFound
	}
	b.Title = title
	b.Description = description
	m.boards[publicID] = b
	return b, nil
}

func (m *mockBoardRepo) DeleteBoard(ctx context.Context, publicID string, userID int64) error {
	b, ok := m.boards[publicID]
	if !ok || b.UserID != userID || b.DeletedAt != nil {
		return board.ErrBoardNotFound
	}
	now := time.Now()
	b.DeletedAt = &now
	m.boards[publicID] = b
	return nil
}

type mockIDGen struct{}

func (mockIDGen) New() (uuid.UUID, error) {
	return uuid.NewV7()
}

func TestBoardService_CRUD(t *testing.T) {
	repo := &mockBoardRepo{boards: make(map[string]board.Board)}
	svc := board.NewService(repo, mockIDGen{})

	// Create
	b, err := svc.Create(context.Background(), 1, board.CreateInput{Title: "Sprint 1", Description: "First sprint"})
	require.NoError(t, err)
	assert.NotEmpty(t, b.PublicID)
	assert.Equal(t, "Sprint 1", b.Title)

	// Get
	found, err := svc.Get(context.Background(), 1, b.PublicID)
	require.NoError(t, err)
	assert.Equal(t, b.Title, found.Title)

	// Other user cannot get
	_, err = svc.Get(context.Background(), 2, b.PublicID)
	require.ErrorIs(t, err, board.ErrBoardNotFound)

	// Update
	newTitle := "Sprint 1 Revised"
	updated, err := svc.Update(context.Background(), 1, b.PublicID, board.UpdateInput{Title: &newTitle})
	require.NoError(t, err)
	assert.Equal(t, "Sprint 1 Revised", updated.Title)

	// List
	list, err := svc.List(context.Background(), 1)
	require.NoError(t, err)
	assert.Len(t, list, 1)

	// Delete
	err = svc.Delete(context.Background(), 1, b.PublicID)
	require.NoError(t, err)

	// Cannot get after delete
	_, err = svc.Get(context.Background(), 1, b.PublicID)
	require.ErrorIs(t, err, board.ErrBoardNotFound)
}
