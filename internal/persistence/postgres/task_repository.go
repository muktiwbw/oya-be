package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"oya-be/internal/persistence/postgres/sqlc"
	"oya-be/internal/task"
)

var _ task.TaskRepository = (*Store)(nil)

func (s *Store) CreateTask(ctx context.Context, in task.CreateRequest) (task.CreateResult, error) {
	boardUUID, err := stringToPgUUID(in.Task.BoardID)
	if err != nil {
		return task.CreateResult{}, task.ErrBoardNotFound
	}
	taskUUID, err := stringToPgUUID(in.Task.PublicID)
	if err != nil {
		return task.CreateResult{}, fmt.Errorf("invalid task public id: %w", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return task.CreateResult{}, err
	}
	defer tx.Rollback(ctx)

	// Verify board exists and belongs to user
	var boardID int64
	err = tx.QueryRow(ctx, "SELECT id FROM boards WHERE public_id = $1 AND user_id = $2 AND deleted_at IS NULL", boardUUID, in.UserID).Scan(&boardID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return task.CreateResult{}, task.ErrBoardNotFound
		}
		return task.CreateResult{}, err
	}

	qtx := s.queries.WithTx(tx)

	if in.IdempotencyKey != "" {
		if err := qtx.DeleteExpiredIdempotencyKeys(ctx, timeToTimestamptz(in.ExpiresBefore)); err != nil {
			return task.CreateResult{}, err
		}
		existing, err := qtx.GetIdempotencyKey(ctx, in.IdempotencyKey)
		if err == nil {
			if existing.RequestHash != in.RequestHash {
				return task.CreateResult{}, task.ErrIdempotencyKeyReused
			}
			var original task.Task
			if err := json.Unmarshal([]byte(existing.OriginalTask), &original); err != nil {
				return task.CreateResult{}, err
			}
			return task.CreateResult{Task: original, Replayed: true}, tx.Commit(ctx)
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return task.CreateResult{}, err
		}
	}

	t := in.Task
	created, err := qtx.CreateTask(ctx, sqlc.CreateTaskParams{
		PublicID:    taskUUID,
		BoardID:     boardID,
		Title:       t.Title,
		Description: t.Description,
		Status:      string(t.Status),
		Version:     t.Version,
		CreatedAt:   timeToTimestamptz(t.CreatedAt),
		UpdatedAt:   timeToTimestamptz(t.UpdatedAt),
	})
	if err != nil {
		return task.CreateResult{}, err
	}
	t.ID = created.ID

	if in.IdempotencyKey != "" {
		snapshot, err := json.Marshal(t)
		if err != nil {
			return task.CreateResult{}, err
		}
		if err := qtx.CreateIdempotencyKey(ctx, sqlc.CreateIdempotencyKeyParams{
			IdempotencyKey: in.IdempotencyKey,
			RequestHash:    in.RequestHash,
			TaskID:         t.ID,
			OriginalTask:   string(snapshot),
			CreatedAt:      timeToTimestamptz(t.CreatedAt),
		}); err != nil {
			return task.CreateResult{}, err
		}
	}

	return task.CreateResult{Task: t}, tx.Commit(ctx)
}

func (s *Store) GetTaskByPublicID(ctx context.Context, id string, userID int64) (task.Task, error) {
	taskUUID, err := stringToPgUUID(id)
	if err != nil {
		return task.Task{}, task.ErrNotFound
	}
	row, err := s.queries.GetTaskByPublicIDAndUserID(ctx, sqlc.GetTaskByPublicIDAndUserIDParams{
		PublicID: taskUUID,
		UserID:   userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return task.Task{}, task.ErrNotFound
		}
		return task.Task{}, err
	}
	return task.Task{
		ID:          row.ID,
		PublicID:    pgUUIDToString(row.PublicID),
		BoardID:     pgUUIDToString(row.BoardPublicID),
		Title:       row.Title,
		Description: row.Description,
		Status:      task.Status(row.Status),
		Version:     row.Version,
		CreatedAt:   row.CreatedAt.Time,
		UpdatedAt:   row.UpdatedAt.Time,
	}, nil
}

func (s *Store) ListTasksByBoard(ctx context.Context, boardID string, userID int64, q task.ListQuery) ([]task.Task, error) {
	boardUUID, err := stringToPgUUID(boardID)
	if err != nil {
		return nil, task.ErrBoardNotFound
	}

	var exists int
	err = s.pool.QueryRow(ctx, "SELECT 1 FROM boards WHERE public_id = $1 AND user_id = $2 AND deleted_at IS NULL", boardUUID, userID).Scan(&exists)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, task.ErrBoardNotFound
		}
		return nil, err
	}

	query := `SELECT t.id, t.public_id, b.public_id AS board_public_id, t.title, t.description, t.status, t.version, t.created_at, t.updated_at
              FROM tasks t
              JOIN boards b ON b.id = t.board_id
              WHERE b.public_id = $1 AND b.user_id = $2 AND t.deleted_at IS NULL AND b.deleted_at IS NULL`
	args := []any{boardUUID, userID}
	argIdx := 3
	if q.Status != "" {
		query += fmt.Sprintf(" AND t.status = $%d", argIdx)
		args = append(args, string(q.Status))
		argIdx++
	}
	if q.After != nil {
		query += fmt.Sprintf(" AND t.created_at <= $%d AND (t.created_at < $%d OR (t.created_at = $%d AND t.public_id > $%d))", argIdx, argIdx+1, argIdx+2, argIdx+3)
		stamp := q.After.CreatedAt
		afterUUID, err := stringToPgUUID(q.After.PublicID)
		if err != nil {
			return nil, task.ErrInvalidCursor
		}
		args = append(args, stamp, stamp, stamp, afterUUID)
		argIdx += 4
	}
	query += fmt.Sprintf(" ORDER BY t.created_at DESC, t.public_id ASC LIMIT $%d", argIdx)
	args = append(args, q.Limit)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []task.Task
	for rows.Next() {
		var t task.Task
		var tPublicID, bPublicID pgtype.UUID
		var statusStr string
		var createdAt, updatedAt pgtype.Timestamptz
		if err := rows.Scan(&t.ID, &tPublicID, &bPublicID, &t.Title, &t.Description, &statusStr, &t.Version, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		t.PublicID = pgUUIDToString(tPublicID)
		t.BoardID = pgUUIDToString(bPublicID)
		t.Status = task.Status(statusStr)
		t.CreatedAt = createdAt.Time
		t.UpdatedAt = updatedAt.Time
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

func (s *Store) UpdateTaskStatusByPublicID(ctx context.Context, id string, userID int64, in task.UpdateStatusInput, now time.Time) (task.Task, error) {
	taskUUID, err := stringToPgUUID(id)
	if err != nil {
		return task.Task{}, task.ErrNotFound
	}
	existing, err := s.GetTaskByPublicID(ctx, id, userID)
	if err != nil {
		return task.Task{}, err
	}
	if in.ExpectedVersion != nil && existing.Version != *in.ExpectedVersion {
		return task.Task{}, task.ErrVersionConflict
	}

	query := `UPDATE tasks t
              SET status = $1, version = version + 1, updated_at = $2
              FROM boards b
              WHERE t.board_id = b.id AND t.public_id = $3 AND b.user_id = $4 AND t.version = $5 AND t.deleted_at IS NULL AND b.deleted_at IS NULL
              RETURNING t.id, t.public_id, b.public_id AS board_public_id, t.title, t.description, t.status, t.version, t.created_at, t.updated_at`

	var t task.Task
	var tPublicID, bPublicID pgtype.UUID
	var statusStr string
	var createdAt, updatedAt pgtype.Timestamptz
	err = s.pool.QueryRow(ctx, query, string(in.Status), now, taskUUID, userID, existing.Version).Scan(
		&t.ID, &tPublicID, &bPublicID, &t.Title, &t.Description, &statusStr, &t.Version, &createdAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return task.Task{}, task.ErrVersionConflict
		}
		return task.Task{}, err
	}
	t.PublicID = pgUUIDToString(tPublicID)
	t.BoardID = pgUUIDToString(bPublicID)
	t.Status = task.Status(statusStr)
	t.CreatedAt = createdAt.Time
	t.UpdatedAt = updatedAt.Time
	return t, nil
}

func (s *Store) UpdateTaskByPublicID(ctx context.Context, id string, userID int64, in task.UpdateTaskInput, now time.Time) (task.Task, error) {
	taskUUID, err := stringToPgUUID(id)
	if err != nil {
		return task.Task{}, task.ErrNotFound
	}
	existing, err := s.GetTaskByPublicID(ctx, id, userID)
	if err != nil {
		return task.Task{}, err
	}
	if in.ExpectedVersion != nil && existing.Version != *in.ExpectedVersion {
		return task.Task{}, task.ErrVersionConflict
	}

	title := existing.Title
	if in.Title != nil {
		title = *in.Title
	}
	desc := existing.Description
	if in.Description != nil {
		desc = *in.Description
	}

	query := `UPDATE tasks t
              SET title = $1, description = $2, version = version + 1, updated_at = $3
              FROM boards b
              WHERE t.board_id = b.id AND t.public_id = $4 AND b.user_id = $5 AND t.version = $6 AND t.deleted_at IS NULL AND b.deleted_at IS NULL
              RETURNING t.id, t.public_id, b.public_id AS board_public_id, t.title, t.description, t.status, t.version, t.created_at, t.updated_at`

	var t task.Task
	var tPublicID, bPublicID pgtype.UUID
	var statusStr string
	var createdAt, updatedAt pgtype.Timestamptz
	err = s.pool.QueryRow(ctx, query, title, desc, now, taskUUID, userID, existing.Version).Scan(
		&t.ID, &tPublicID, &bPublicID, &t.Title, &t.Description, &statusStr, &t.Version, &createdAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return task.Task{}, task.ErrVersionConflict
		}
		return task.Task{}, err
	}
	t.PublicID = pgUUIDToString(tPublicID)
	t.BoardID = pgUUIDToString(bPublicID)
	t.Status = task.Status(statusStr)
	t.CreatedAt = createdAt.Time
	t.UpdatedAt = updatedAt.Time
	return t, nil
}

func (s *Store) DeleteTaskByPublicID(ctx context.Context, id string, userID int64) error {
	taskUUID, err := stringToPgUUID(id)
	if err != nil {
		return task.ErrNotFound
	}
	query := `UPDATE tasks t
              SET deleted_at = NOW(), updated_at = NOW()
              FROM boards b
              WHERE t.board_id = b.id AND t.public_id = $1 AND b.user_id = $2 AND t.deleted_at IS NULL AND b.deleted_at IS NULL`
	tag, err := s.pool.Exec(ctx, query, taskUUID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return task.ErrNotFound
	}
	return nil
}
