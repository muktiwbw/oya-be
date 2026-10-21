package task

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Clock interface{ Now() time.Time }
type PublicIDGenerator interface{ New() (uuid.UUID, error) }

// ListQuery contains validated values; persistence never decodes opaque tokens.
type ListQuery struct {
	Status Status
	Limit  int
	After  *Position
}

type Page struct {
	Tasks      []Task
	Limit      int
	HasMore    bool
	NextCursor *string
}

type CreateResult struct {
	Task     Task
	Replayed bool
}

// CreateRequest makes task insertion and optional retry tracking one atomic operation.
type CreateRequest struct {
	UserID         int64
	Task           Task
	IdempotencyKey string
	RequestHash    string
	ExpiresBefore  time.Time
}

type TaskRepository interface {
	CreateTask(context.Context, CreateRequest) (CreateResult, error)
	GetTaskByPublicID(context.Context, string, int64) (Task, error)
	ListTasksByBoard(context.Context, string, int64, ListQuery) ([]Task, error)
	UpdateTaskStatusByPublicID(context.Context, string, int64, UpdateStatusInput, time.Time) (Task, error)
	UpdateTaskByPublicID(context.Context, string, int64, UpdateTaskInput, time.Time) (Task, error)
	DeleteTaskByPublicID(context.Context, string, int64) error
}
