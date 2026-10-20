// Package task defines task behavior and the ports used by application services.
package task

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

type Status string

const (
	Todo  Status = "todo"
	Doing Status = "doing"
	Done  Status = "done"
)

func (s Status) Valid() bool { return s == Todo || s == Doing || s == Done }

// Task is transport independent. HTTP representations must omit the internal ID.
type Task struct {
	ID          int64
	PublicID    string
	BoardID     string // Board's public UUID
	Title       string
	Description string
	Status      Status
	Version     int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

type CreateInput struct {
	BoardID     string
	Title       string
	Description string
	Status      Status
}

type ValidationError struct {
	Field  string
	Reason string
}

func (e *ValidationError) Error() string { return fmt.Sprintf("%s: %s", e.Field, e.Reason) }

var (
	ErrNotFound             = errors.New("task not found")
	ErrBoardNotFound        = errors.New("board not found")
	ErrVersionConflict      = errors.New("task version conflict")
	ErrIdempotencyKeyReused = errors.New("idempotency key reused with different input")
	ErrUnavailable          = errors.New("service unavailable")
)

func NormalizeCreate(in CreateInput) (CreateInput, error) {
	in.BoardID = strings.TrimSpace(in.BoardID)
	if in.BoardID == "" {
		return in, &ValidationError{"board_id", "required"}
	}
	if !ValidPublicID(in.BoardID) {
		return in, &ValidationError{"board_id", "must be a canonical UUIDv7"}
	}
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" {
		return in, &ValidationError{"title", "required"}
	}
	if !utf8.ValidString(in.Title) || utf8.RuneCountInString(in.Title) > 200 {
		return in, &ValidationError{"title", "must contain at most 200 Unicode characters"}
	}
	if !utf8.ValidString(in.Description) || utf8.RuneCountInString(in.Description) > 2000 {
		return in, &ValidationError{"description", "must contain at most 2000 Unicode characters"}
	}
	if in.Status == "" {
		in.Status = Todo
	}
	if !in.Status.Valid() {
		return in, &ValidationError{"status", "must be todo, doing, or done"}
	}
	return in, nil
}

type UpdateStatusInput struct {
	Status          Status
	ExpectedVersion *int64
}

func (in UpdateStatusInput) Validate() error {
	if !in.Status.Valid() {
		return &ValidationError{"status", "must be todo, doing, or done"}
	}
	if in.ExpectedVersion == nil {
		return &ValidationError{"expected_version", "required"}
	}
	if *in.ExpectedVersion <= 0 {
		return &ValidationError{"expected_version", "must be positive"}
	}
	return nil
}

// UpdateTaskInput represents a partial title and/or Markdown description edit.
// Nil title or description means that field was omitted from the request.
type UpdateTaskInput struct {
	Title           *string
	Description     *string
	ExpectedVersion *int64
}

func NormalizeUpdate(in UpdateTaskInput) (UpdateTaskInput, error) {
	if in.Title == nil && in.Description == nil {
		return in, &ValidationError{"task", "at least one of title or description is required"}
	}
	if in.ExpectedVersion == nil {
		return in, &ValidationError{"expected_version", "required"}
	}
	if *in.ExpectedVersion <= 0 {
		return in, &ValidationError{"expected_version", "must be positive"}
	}
	if in.Title != nil {
		title := strings.TrimSpace(*in.Title)
		in.Title = &title
		if title == "" {
			return in, &ValidationError{"title", "required"}
		}
		if !utf8.ValidString(title) || utf8.RuneCountInString(title) > 200 {
			return in, &ValidationError{"title", "must contain at most 200 Unicode characters"}
		}
	}
	if in.Description != nil && (!utf8.ValidString(*in.Description) || utf8.RuneCountInString(*in.Description) > 2000) {
		return in, &ValidationError{"description", "must contain at most 2000 Unicode characters"}
	}
	return in, nil
}

func NormalizeTime(t time.Time) time.Time { return t.UTC().Truncate(time.Millisecond) }
func FormatTime(t time.Time) string       { return NormalizeTime(t).Format("2006-01-02T15:04:05.000Z") }
