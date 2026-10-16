package httpapi

import "oya-be/internal/task"

type CreateTaskRequest struct {
	Title       string      `json:"title"`
	Description string      `json:"description,omitempty"`
	Status      task.Status `json:"status,omitempty"`
}

type UpdateTaskStatusRequest struct {
	Status          task.Status `json:"status"`
	ExpectedVersion *int64      `json:"expected_version"`
}

type UpdateTaskRequest struct {
	Title           *string `json:"title,omitempty"`
	Description     *string `json:"description,omitempty"`
	ExpectedVersion *int64  `json:"expected_version"`
}

type ListTasksRequest struct {
	Status task.Status `query:"status"`
	Limit  int         `query:"limit"`
	Cursor string      `query:"cursor"`
}

type TaskDTO struct {
	PublicID    string      `json:"public_id"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	Status      task.Status `json:"status"`
	Version     int64       `json:"version"`
	CreatedAt   string      `json:"created_at"`
	UpdatedAt   string      `json:"updated_at"`
}

// newTaskDTO converts a domain task into its public HTTP representation.
func newTaskDTO(t task.Task) TaskDTO {
	return TaskDTO{
		PublicID: t.PublicID, Title: t.Title, Description: t.Description,
		Status: t.Status, Version: t.Version,
		CreatedAt: task.FormatTime(t.CreatedAt), UpdatedAt: task.FormatTime(t.UpdatedAt),
	}
}

type TaskResponse struct {
	Data TaskDTO `json:"data"`
}

type ListTasksResponse struct {
	Data       []TaskDTO     `json:"data"`
	Pagination PaginationDTO `json:"pagination"`
}

type PaginationDTO struct {
	Limit      int     `json:"limit"`
	Count      int     `json:"count"`
	HasMore    bool    `json:"has_more"`
	NextCursor *string `json:"next_cursor"`
}
