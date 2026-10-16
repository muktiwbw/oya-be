package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"oya-be/internal/task"
)

func TestCursorQuerySeeksIntoIndex(t *testing.T) {
	s := openTest(t)
	for _, status := range []task.Status{"", task.Todo} {
		query, args := listTasksSQL(task.ListQuery{Status: status, Limit: 51, After: &task.Position{CreatedAt: time.Now(), PublicID: "018f4b7e-3b75-7e1f-b5d5-2b2d7b0a7f12"}})
		rows, err := s.db.Query("EXPLAIN QUERY PLAN "+query, args...)
		if err != nil {
			t.Fatal(err)
		}
		plan := ""
		for rows.Next() {
			var id, parent, unused int
			var detail string
			if err = rows.Scan(&id, &parent, &unused, &detail); err != nil {
				t.Fatal(err)
			}
			plan += detail
		}
		rows.Close()
		if !strings.Contains(plan, "SEARCH tasks USING INDEX idx_tasks_") || strings.Contains(plan, "TEMP B-TREE") {
			t.Fatalf("cursor query must seek using ordered index: %s", plan)
		}
	}
}

func TestConcurrentConditionalUpdates(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	in := sample()
	_, err := s.CreateTask(ctx, task.CreateRequest{Task: in})
	require.NoError(t, err)
	var wg sync.WaitGroup
	out := make(chan error, 2)
	for _, status := range []task.Status{task.Doing, task.Done} {
		wg.Add(1)
		go func(status task.Status) {
			defer wg.Done()
			v := int64(1)
			_, err := s.UpdateTaskStatusByPublicID(ctx, in.PublicID, task.UpdateStatusInput{Status: status, ExpectedVersion: &v}, in.CreatedAt.Add(time.Second))
			out <- err
		}(status)
	}
	wg.Wait()
	close(out)
	success, conflicts := 0, 0
	for err := range out {
		if err == nil {
			success++
		} else if errors.Is(err, task.ErrVersionConflict) {
			conflicts++
		} else {
			require.NoError(t, err)
		}
	}
	assert.Equal(t, 1, success)
	assert.Equal(t, 1, conflicts)
}

func TestConcurrentTaskEditsHaveOneWinner(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	in := sample()
	_, err := s.CreateTask(ctx, task.CreateRequest{Task: in})
	require.NoError(t, err)
	var wg sync.WaitGroup
	out := make(chan error, 2)
	for _, title := range []string{"first", "second"} {
		wg.Add(1)
		go func(title string) {
			defer wg.Done()
			v := int64(1)
			_, err := s.UpdateTaskByPublicID(ctx, in.PublicID, task.UpdateTaskInput{Title: &title, ExpectedVersion: &v}, in.CreatedAt.Add(time.Second))
			out <- err
		}(title)
	}
	wg.Wait()
	close(out)
	success, conflicts := 0, 0
	for err := range out {
		if err == nil {
			success++
		} else if errors.Is(err, task.ErrVersionConflict) {
			conflicts++
		} else {
			require.NoError(t, err)
		}
	}
	assert.Equal(t, 1, success)
	assert.Equal(t, 1, conflicts)
}

func TestConcurrentRetryCreates(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	now := task.NormalizeTime(time.Now())
	var wg sync.WaitGroup
	out := make(chan task.CreateResult, 8)
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			in := sample()
			got, err := s.CreateTask(ctx, task.CreateRequest{Task: in, IdempotencyKey: "same", RequestHash: "same", ExpiresBefore: now.Add(-time.Hour)})
			out <- got
			errs <- err
		}()
	}
	wg.Wait()
	close(out)
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	id := ""
	originals := 0
	for result := range out {
		if id == "" {
			id = result.Task.PublicID
		}
		assert.Equal(t, id, result.Task.PublicID, "same key created multiple tasks")
		if !result.Replayed {
			originals++
		}
	}
	assert.Equal(t, 1, originals)
}

func TestEmptyListAndFilteredCursorBoundaries(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	now := task.NormalizeTime(time.Now())
	empty, err := s.ListTasks(ctx, task.ListQuery{Limit: 3})
	require.NoError(t, err)
	assert.Empty(t, empty)
	for i := 0; i < 4; i++ {
		in := sample()
		in.CreatedAt = now.Add(-time.Duration(i) * time.Millisecond)
		in.UpdatedAt = in.CreatedAt
		if i == 1 {
			in.Status = task.Done
		}
		_, err := s.CreateTask(ctx, task.CreateRequest{Task: in})
		require.NoError(t, err)
	}
	first, err := s.ListTasks(ctx, task.ListQuery{Status: task.Todo, Limit: 2})
	require.NoError(t, err)
	require.Len(t, first, 2)
	last := first[1]
	after := task.Position{CreatedAt: last.CreatedAt, PublicID: last.PublicID}
	next, err := s.ListTasks(ctx, task.ListQuery{Status: task.Todo, Limit: 2, After: &after})
	require.NoError(t, err)
	require.Len(t, next, 1)
	assert.True(t, next[0].CreatedAt.Before(last.CreatedAt), "wrong continuation order")
}

func TestFailedMigrationIsNotRecorded(t *testing.T) {
	path := filepath.Join(t.TempDir(), "broken.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec("CREATE TABLE tasks (id INTEGER PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}
	if s, err := Open(context.Background(), path); err == nil {
		s.Close()
		t.Fatal("startup accepted failed migration")
	}
	var count int
	if err = db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("failed migration recorded as applied")
	}
}

func openTest(t *testing.T) *Store {
	t.Helper()
	s, err := Open(context.Background(), filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
func sample() task.Task {
	now := task.NormalizeTime(time.Now())
	return task.Task{PublicID: uuid.Must(uuid.NewV7()).String(), Title: "task", Status: task.Todo, Version: 1, CreatedAt: now, UpdatedAt: now}
}

func TestPersistenceAndStatusTransitions(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	for _, from := range []task.Status{task.Todo, task.Doing, task.Done} {
		for _, to := range []task.Status{task.Todo, task.Doing, task.Done} {
			in := sample()
			in.Status = from
			created, err := s.CreateTask(ctx, task.CreateRequest{Task: in})
			if err != nil {
				t.Fatal(err)
			}
			if created.Task.ID <= 0 {
				t.Fatal("missing internal ID")
			}
			got, err := s.GetTaskByPublicID(ctx, in.PublicID)
			if err != nil || got.PublicID != in.PublicID {
				t.Fatalf("read: %+v %v", got, err)
			}
			v := int64(1)
			out, err := s.UpdateTaskStatusByPublicID(ctx, in.PublicID, task.UpdateStatusInput{Status: to, ExpectedVersion: &v}, in.CreatedAt.Add(time.Second))
			if err != nil {
				t.Fatal(err)
			}
			if out.Status != to {
				t.Fatal("status not updated")
			}
			if from == to && (out.Version != 1 || !out.UpdatedAt.Equal(in.UpdatedAt)) {
				t.Fatal("no-op mutated task")
			}
			if from != to && (out.Version != 2 || !out.UpdatedAt.After(in.UpdatedAt)) {
				t.Fatal("change did not update metadata")
			}
			v = 99
			if _, err := s.UpdateTaskStatusByPublicID(ctx, in.PublicID, task.UpdateStatusInput{Status: to, ExpectedVersion: &v}, in.CreatedAt); !errors.Is(err, task.ErrVersionConflict) {
				t.Fatalf("expected conflict, got %v", err)
			}
		}
	}
	if _, err := s.GetTaskByPublicID(ctx, uuid.NewString()); !errors.Is(err, task.ErrNotFound) {
		t.Fatal(err)
	}
}

func TestUpdateTaskFieldsAndNoOp(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	in := sample()
	in.Description = "old"
	_, err := s.CreateTask(ctx, task.CreateRequest{Task: in})
	require.NoError(t, err)
	v := int64(1)
	title := "new"
	out, err := s.UpdateTaskByPublicID(ctx, in.PublicID, task.UpdateTaskInput{Title: &title, ExpectedVersion: &v}, in.CreatedAt.Add(time.Second))
	require.NoError(t, err)
	assert.Equal(t, "new", out.Title)
	assert.Equal(t, "old", out.Description)
	assert.Equal(t, int64(2), out.Version)
	v = 2
	empty := ""
	out, err = s.UpdateTaskByPublicID(ctx, in.PublicID, task.UpdateTaskInput{Description: &empty, ExpectedVersion: &v}, in.CreatedAt.Add(2*time.Second))
	require.NoError(t, err)
	assert.Empty(t, out.Description)
	assert.Equal(t, int64(3), out.Version)
	updatedAt := out.UpdatedAt
	v = 3
	out, err = s.UpdateTaskByPublicID(ctx, in.PublicID, task.UpdateTaskInput{Description: &empty, ExpectedVersion: &v}, in.CreatedAt.Add(3*time.Second))
	require.NoError(t, err)
	assert.Equal(t, int64(3), out.Version)
	assert.Equal(t, updatedAt, out.UpdatedAt)
	v = 1
	_, err = s.UpdateTaskByPublicID(ctx, in.PublicID, task.UpdateTaskInput{Title: &title, ExpectedVersion: &v}, time.Now())
	require.ErrorIs(t, err, task.ErrVersionConflict)
}

func TestListCursorAndUniqueness(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	in := sample()
	for i := 0; i < 5; i++ {
		in.PublicID = uuid.Must(uuid.NewV7()).String()
		if _, err := s.CreateTask(ctx, task.CreateRequest{Task: in}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.CreateTask(ctx, task.CreateRequest{Task: in}); err == nil {
		t.Fatal("accepted duplicate public ID")
	}
	rows, err := s.ListTasks(ctx, task.ListQuery{Limit: 2})
	if err != nil || len(rows) != 2 {
		t.Fatalf("list: %v %v", rows, err)
	}
	after := task.Position{CreatedAt: rows[1].CreatedAt, PublicID: rows[1].PublicID}
	next, err := s.ListTasks(ctx, task.ListQuery{Limit: 10, After: &after, Status: task.Todo})
	if err != nil || len(next) != 3 {
		t.Fatalf("cursor: %v %v", next, err)
	}
	for _, row := range next {
		if row.PublicID <= after.PublicID {
			t.Fatal("cursor repeated row")
		}
	}
}

func TestIdempotencyAndRestart(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "tasks.db")
	s, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	in := sample()
	req := task.CreateRequest{Task: in, IdempotencyKey: "key", RequestHash: "hash", ExpiresBefore: in.CreatedAt.Add(-24 * time.Hour)}
	first, err := s.CreateTask(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	req.Task = sample()
	again, err := s.CreateTask(ctx, req)
	if err != nil || !again.Replayed || first.Task.PublicID != again.Task.PublicID {
		t.Fatalf("replay: %+v %v", again, err)
	}
	req.RequestHash = "different"
	if _, err := s.CreateTask(ctx, req); !errors.Is(err, task.ErrIdempotencyKeyReused) {
		t.Fatal(err)
	}
	req.ExpiresBefore = in.CreatedAt.Add(time.Millisecond)
	out, err := s.CreateTask(ctx, req)
	if err != nil || out.Replayed || out.Task.PublicID == first.Task.PublicID {
		t.Fatalf("expiry: %+v %v", out, err)
	}
}
