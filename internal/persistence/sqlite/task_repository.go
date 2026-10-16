package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"oya-be/internal/task"
)

var _ task.TaskRepository = (*Store)(nil)

const taskColumns = "id,public_id,title,description,status,version,created_at,updated_at"

type taskScanner interface{ Scan(...any) error }

func scanTask(row taskScanner) (task.Task, error) {
	var t task.Task
	var created, updated string
	if err := row.Scan(&t.ID, &t.PublicID, &t.Title, &t.Description, &t.Status, &t.Version, &created, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return t, task.ErrNotFound
		}
		return t, err
	}
	var err error
	if t.CreatedAt, err = time.Parse(time.RFC3339Nano, created); err != nil {
		return t, err
	}
	t.UpdatedAt, err = time.Parse(time.RFC3339Nano, updated)
	return t, err
}

func (s *Store) CreateTask(ctx context.Context, in task.CreateRequest) (task.CreateResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return task.CreateResult{}, err
	}
	defer tx.Rollback()
	if in.IdempotencyKey != "" {
		if _, err = tx.ExecContext(ctx, "DELETE FROM idempotency_keys WHERE created_at <= ?", task.FormatTime(in.ExpiresBefore)); err != nil {
			return task.CreateResult{}, err
		}
		var hash, snapshot string
		err = tx.QueryRowContext(ctx, "SELECT request_hash,original_task FROM idempotency_keys WHERE idempotency_key=?", in.IdempotencyKey).Scan(&hash, &snapshot)
		if err == nil {
			if hash != in.RequestHash {
				return task.CreateResult{}, task.ErrIdempotencyKeyReused
			}
			var original task.Task
			if err = json.Unmarshal([]byte(snapshot), &original); err != nil {
				return task.CreateResult{}, err
			}
			return task.CreateResult{Task: original, Replayed: true}, tx.Commit()
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return task.CreateResult{}, err
		}
	}
	t := in.Task
	result, err := tx.ExecContext(ctx, "INSERT INTO tasks(public_id,title,description,status,version,created_at,updated_at) VALUES (?,?,?,?,?,?,?)", t.PublicID, t.Title, t.Description, t.Status, t.Version, task.FormatTime(t.CreatedAt), task.FormatTime(t.UpdatedAt))
	if err != nil {
		return task.CreateResult{}, err
	}
	if t.ID, err = result.LastInsertId(); err != nil {
		return task.CreateResult{}, err
	}
	if in.IdempotencyKey != "" {
		snapshot, err := json.Marshal(t)
		if err != nil {
			return task.CreateResult{}, err
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO idempotency_keys(idempotency_key,request_hash,task_id,original_task,created_at) VALUES (?,?,?,?,?)", in.IdempotencyKey, in.RequestHash, t.ID, string(snapshot), task.FormatTime(t.CreatedAt)); err != nil {
			return task.CreateResult{}, err
		}
	}
	return task.CreateResult{Task: t}, tx.Commit()
}

func (s *Store) GetTaskByPublicID(ctx context.Context, id string) (task.Task, error) {
	return scanTask(s.db.QueryRowContext(ctx, "SELECT "+taskColumns+" FROM tasks WHERE public_id=?", id))
}

func listTasksSQL(q task.ListQuery) (string, []any) {
	query := "SELECT " + taskColumns + " FROM tasks WHERE 1=1"
	args := []any{}
	if q.Status != "" {
		query += " AND status=?"
		args = append(args, q.Status)
	}
	if q.After != nil {
		query += " AND created_at <= ? AND (created_at < ? OR (created_at = ? AND public_id > ?))"
		stamp := task.FormatTime(q.After.CreatedAt)
		args = append(args, stamp, stamp, stamp, q.After.PublicID)
	}
	query += " ORDER BY created_at DESC,public_id ASC LIMIT ?"
	return query, append(args, q.Limit)
}

func (s *Store) ListTasks(ctx context.Context, q task.ListQuery) ([]task.Task, error) {
	query, args := listTasksSQL(q)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []task.Task{}
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, t)
	}
	return result, rows.Err()
}

func (s *Store) UpdateTaskStatusByPublicID(ctx context.Context, id string, in task.UpdateStatusInput, now time.Time) (task.Task, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return task.Task{}, err
	}
	defer tx.Rollback()
	t, err := scanTask(tx.QueryRowContext(ctx, "SELECT "+taskColumns+" FROM tasks WHERE public_id=?", id))
	if err != nil {
		return t, err
	}
	if t.Version != *in.ExpectedVersion {
		return task.Task{}, task.ErrVersionConflict
	}
	if t.Status != in.Status {
		result, err := tx.ExecContext(ctx, "UPDATE tasks SET status=?,version=version+1,updated_at=? WHERE public_id=? AND version=?", in.Status, task.FormatTime(now), id, t.Version)
		if err != nil {
			return task.Task{}, err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return task.Task{}, err
		}
		if n != 1 {
			return task.Task{}, task.ErrVersionConflict
		}
		t.Status, t.Version, t.UpdatedAt = in.Status, t.Version+1, now
	}
	return t, tx.Commit()
}

func (s *Store) UpdateTaskByPublicID(ctx context.Context, id string, in task.UpdateTaskInput, now time.Time) (task.Task, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return task.Task{}, err
	}
	defer tx.Rollback()
	t, err := scanTask(tx.QueryRowContext(ctx, "SELECT "+taskColumns+" FROM tasks WHERE public_id=?", id))
	if err != nil {
		return t, err
	}
	if t.Version != *in.ExpectedVersion {
		return task.Task{}, task.ErrVersionConflict
	}
	title, description := t.Title, t.Description
	if in.Title != nil {
		title = *in.Title
	}
	if in.Description != nil {
		description = *in.Description
	}
	if title != t.Title || description != t.Description {
		result, err := tx.ExecContext(ctx, "UPDATE tasks SET title=?,description=?,version=version+1,updated_at=? WHERE public_id=? AND version=?", title, description, task.FormatTime(now), id, t.Version)
		if err != nil {
			return task.Task{}, err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return task.Task{}, err
		}
		if n != 1 {
			return task.Task{}, task.ErrVersionConflict
		}
		t.Title, t.Description, t.Version, t.UpdatedAt = title, description, t.Version+1, now
	}
	return t, tx.Commit()
}
