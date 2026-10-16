package task_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"oya-be/internal/task"
	"oya-be/internal/task/mocks"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestServiceCreate(t *testing.T) {
	ctx := context.Background()
	repo := mocks.NewMockTaskRepository(t)
	clock := mocks.NewMockClock(t)
	ids := mocks.NewMockPublicIDGenerator(t)
	now := time.Date(2026, 10, 16, 0, 0, 0, 123456789, time.UTC)
	id := uuid.MustParse("018f4b7e-3b75-7e1f-b5d5-2b2d7b0a7f12")
	clock.EXPECT().Now().Return(now).Twice()
	ids.EXPECT().New().Return(id, nil).Twice()
	var captured []task.CreateRequest
	repo.EXPECT().CreateTask(ctx, mock.Anything).RunAndReturn(func(_ context.Context, in task.CreateRequest) (task.CreateResult, error) {
		captured = append(captured, in)
		return task.CreateResult{Task: in.Task}, nil
	}).Twice()
	s := task.NewService(repo, clock, ids, 24*time.Hour)
	out, err := s.CreateTask(ctx, task.CreateInput{Title: "  task  "}, "retry-key")
	require.NoError(t, err)
	assert.Equal(t, "task", out.Task.Title)
	assert.Equal(t, id.String(), out.Task.PublicID)
	assert.Equal(t, int64(1), out.Task.Version)
	assert.Equal(t, task.Todo, out.Task.Status)
	assert.Equal(t, task.NormalizeTime(now), out.Task.CreatedAt)
	assert.Equal(t, out.Task.CreatedAt, out.Task.UpdatedAt)
	require.Len(t, captured, 1)
	assert.NotEmpty(t, captured[0].RequestHash)
	assert.Equal(t, "retry-key", captured[0].IdempotencyKey)
	assert.Equal(t, task.NormalizeTime(now).Add(-24*time.Hour), captured[0].ExpiresBefore)
	_, err = s.CreateTask(ctx, task.CreateInput{Title: "task", Status: task.Todo}, "another-key")
	require.NoError(t, err)
	require.Len(t, captured, 2)
	assert.Equal(t, captured[0].RequestHash, captured[1].RequestHash, "equivalent normalized requests must have the same hash")
}

func TestServiceRejectsInvalidInputBeforePersistence(t *testing.T) {
	// No expectations: any call to persistence, time, or IDs fails this test.
	s := task.NewService(mocks.NewMockTaskRepository(t), mocks.NewMockClock(t), mocks.NewMockPublicIDGenerator(t), time.Hour)
	_, err := s.CreateTask(context.Background(), task.CreateInput{}, "key")
	var validation *task.ValidationError
	require.ErrorAs(t, err, &validation)
	assert.Equal(t, "title", validation.Field)
	_, err = s.GetTask(context.Background(), "bad")
	require.ErrorAs(t, err, &validation)
	assert.Equal(t, "public_id", validation.Field)
	_, err = s.ListTasks(context.Background(), "", 101, "")
	require.ErrorAs(t, err, &validation)
	assert.Equal(t, "limit", validation.Field)
}

func TestServiceIDFailure(t *testing.T) {
	repo := mocks.NewMockTaskRepository(t)
	clock := mocks.NewMockClock(t)
	ids := mocks.NewMockPublicIDGenerator(t)
	want := errors.New("entropy unavailable")
	ids.EXPECT().New().Return(uuid.Nil, want).Once()
	s := task.NewService(repo, clock, ids, time.Hour)
	_, err := s.CreateTask(context.Background(), task.CreateInput{Title: "task"}, "")
	require.ErrorIs(t, err, want)
}

func TestServiceGetTaskPropagatesNotFound(t *testing.T) {
	ctx := context.Background()
	repo := mocks.NewMockTaskRepository(t)
	id := "018f4b7e-3b75-7e1f-b5d5-2b2d7b0a7f12"
	repo.EXPECT().GetTaskByPublicID(ctx, id).Return(task.Task{}, task.ErrNotFound).Once()
	s := task.NewService(repo, mocks.NewMockClock(t), mocks.NewMockPublicIDGenerator(t), time.Hour)
	_, err := s.GetTask(ctx, id)
	require.ErrorIs(t, err, task.ErrNotFound)
}

func TestServiceCursorPage(t *testing.T) {
	ctx := context.Background()
	repo := mocks.NewMockTaskRepository(t)
	row := task.Task{PublicID: "018f4b7e-3b75-7e1f-b5d5-2b2d7b0a7f12", CreatedAt: time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC)}
	repo.EXPECT().ListTasks(ctx, task.ListQuery{Status: task.Todo, Limit: 2}).Return([]task.Task{row, row}, nil).Once()
	s := task.NewService(repo, mocks.NewMockClock(t), mocks.NewMockPublicIDGenerator(t), time.Hour)
	page, err := s.ListTasks(ctx, task.Todo, 1, "")
	require.NoError(t, err)
	assert.Equal(t, []task.Task{row}, page.Tasks)
	assert.True(t, page.HasMore)
	require.NotNil(t, page.NextCursor)
	position, err := task.DecodeCursor(*page.NextCursor, task.Todo)
	require.NoError(t, err)
	assert.Equal(t, task.Position{CreatedAt: row.CreatedAt, PublicID: row.PublicID}, position)
}

func TestServiceUpdatePropagatesConflict(t *testing.T) {
	ctx := context.Background()
	repo := mocks.NewMockTaskRepository(t)
	clock := mocks.NewMockClock(t)
	now := time.Date(2026, 10, 16, 0, 0, 0, 123456789, time.UTC)
	id := "018f4b7e-3b75-7e1f-b5d5-2b2d7b0a7f12"
	v := int64(1)
	in := task.UpdateStatusInput{Status: task.Doing, ExpectedVersion: &v}
	clock.EXPECT().Now().Return(now).Once()
	repo.EXPECT().UpdateTaskStatusByPublicID(ctx, id, in, task.NormalizeTime(now)).Return(task.Task{}, task.ErrVersionConflict).Once()
	s := task.NewService(repo, clock, mocks.NewMockPublicIDGenerator(t), time.Hour)
	_, err := s.UpdateTaskStatus(ctx, id, in)
	require.ErrorIs(t, err, task.ErrVersionConflict)
}

func TestServiceUpdateTaskNormalizesAndUsesClock(t *testing.T) {
	ctx := context.Background()
	repo := mocks.NewMockTaskRepository(t)
	clock := mocks.NewMockClock(t)
	now := time.Date(2026, 10, 16, 0, 0, 0, 123456789, time.UTC)
	id := "018f4b7e-3b75-7e1f-b5d5-2b2d7b0a7f12"
	v := int64(1)
	title := "  task  "
	in := task.UpdateTaskInput{Title: &title, ExpectedVersion: &v}
	want := task.UpdateTaskInput{Title: ptr("task"), ExpectedVersion: &v}
	clock.EXPECT().Now().Return(now).Once()
	repo.EXPECT().UpdateTaskByPublicID(ctx, id, want, task.NormalizeTime(now)).Return(task.Task{}, task.ErrVersionConflict).Once()
	s := task.NewService(repo, clock, mocks.NewMockPublicIDGenerator(t), time.Hour)
	_, err := s.UpdateTask(ctx, id, in)
	require.ErrorIs(t, err, task.ErrVersionConflict)
}

func ptr(s string) *string { return &s }
