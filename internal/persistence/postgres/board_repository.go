package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"oya-be/internal/board"
	"oya-be/internal/persistence/postgres/sqlc"
)

var _ board.Repository = (*Store)(nil)

func toBoardDomain(b sqlc.Board) board.Board {
	var delPtr *timePtr
	_ = delPtr
	res := board.Board{
		ID:          b.ID,
		PublicID:    pgUUIDToString(b.PublicID),
		UserID:      b.UserID,
		Title:       b.Title,
		Description: b.Description,
		CreatedAt:   b.CreatedAt.Time,
		UpdatedAt:   b.UpdatedAt.Time,
	}
	if b.DeletedAt.Valid {
		t := b.DeletedAt.Time
		res.DeletedAt = &t
	}
	return res
}

type timePtr = pgtype.Timestamptz

func (s *Store) CreateBoard(ctx context.Context, b board.Board) (board.Board, error) {
	uuidVal, err := stringToPgUUID(b.PublicID)
	if err != nil {
		return board.Board{}, err
	}
	created, err := s.queries.CreateBoard(ctx, sqlc.CreateBoardParams{
		PublicID:    uuidVal,
		UserID:      b.UserID,
		Title:       b.Title,
		Description: b.Description,
	})
	if err != nil {
		return board.Board{}, err
	}
	return toBoardDomain(created), nil
}

func (s *Store) GetBoardByPublicID(ctx context.Context, publicID string, userID int64) (board.Board, error) {
	uuidVal, err := stringToPgUUID(publicID)
	if err != nil {
		return board.Board{}, board.ErrBoardNotFound
	}
	b, err := s.queries.GetBoardByPublicID(ctx, sqlc.GetBoardByPublicIDParams{
		PublicID: uuidVal,
		UserID:   userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return board.Board{}, board.ErrBoardNotFound
		}
		return board.Board{}, err
	}
	return toBoardDomain(b), nil
}

func (s *Store) GetBoardByPublicIDAnyUser(ctx context.Context, publicID string) (board.Board, error) {
	uuidVal, err := stringToPgUUID(publicID)
	if err != nil {
		return board.Board{}, board.ErrBoardNotFound
	}
	b, err := s.queries.GetBoardByPublicIDAnyUser(ctx, uuidVal)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return board.Board{}, board.ErrBoardNotFound
		}
		return board.Board{}, err
	}
	return toBoardDomain(b), nil
}

func (s *Store) ListBoardsByUserID(ctx context.Context, userID int64) ([]board.Board, error) {
	rows, err := s.queries.ListBoardsByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	boards := make([]board.Board, 0, len(rows))
	for _, row := range rows {
		boards = append(boards, toBoardDomain(row))
	}
	return boards, nil
}

func (s *Store) UpdateBoard(ctx context.Context, publicID string, userID int64, title, description string) (board.Board, error) {
	uuidVal, err := stringToPgUUID(publicID)
	if err != nil {
		return board.Board{}, board.ErrBoardNotFound
	}
	updated, err := s.queries.UpdateBoard(ctx, sqlc.UpdateBoardParams{
		Title:       title,
		Description: description,
		PublicID:    uuidVal,
		UserID:      userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return board.Board{}, board.ErrBoardNotFound
		}
		return board.Board{}, err
	}
	return toBoardDomain(updated), nil
}

func (s *Store) DeleteBoard(ctx context.Context, publicID string, userID int64) error {
	uuidVal, err := stringToPgUUID(publicID)
	if err != nil {
		return board.ErrBoardNotFound
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	qtx := s.queries.WithTx(tx)
	boardID, err := qtx.SoftDeleteBoard(ctx, sqlc.SoftDeleteBoardParams{
		PublicID: uuidVal,
		UserID:   userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return board.ErrBoardNotFound
		}
		return err
	}

	if err := qtx.CascadeSoftDeleteTasksByBoardID(ctx, boardID); err != nil {
		return err
	}

	return tx.Commit(ctx)
}
