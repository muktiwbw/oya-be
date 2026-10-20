package board

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

var (
	ErrBoardNotFound = errors.New("board not found")
)

type ValidationError struct {
	Field  string
	Reason string
}

func (e *ValidationError) Error() string { return fmt.Sprintf("%s: %s", e.Field, e.Reason) }

type Board struct {
	ID          int64
	PublicID    string
	UserID      int64
	Title       string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

type CreateInput struct {
	Title       string
	Description string
}

func (in CreateInput) Validate() (CreateInput, error) {
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" {
		return in, &ValidationError{Field: "title", Reason: "must not be blank"}
	}
	if utf8.RuneCountInString(in.Title) > 200 {
		return in, &ValidationError{Field: "title", Reason: "must not exceed 200 characters"}
	}
	in.Description = strings.TrimSpace(in.Description)
	if utf8.RuneCountInString(in.Description) > 2000 {
		return in, &ValidationError{Field: "description", Reason: "must not exceed 2000 characters"}
	}
	return in, nil
}

type UpdateInput struct {
	Title       *string
	Description *string
}

func (in UpdateInput) Validate() (UpdateInput, error) {
	if in.Title == nil && in.Description == nil {
		return in, &ValidationError{Field: "body", Reason: "at least one field must be provided"}
	}
	if in.Title != nil {
		trimmed := strings.TrimSpace(*in.Title)
		if trimmed == "" {
			return in, &ValidationError{Field: "title", Reason: "must not be blank"}
		}
		if utf8.RuneCountInString(trimmed) > 200 {
			return in, &ValidationError{Field: "title", Reason: "must not exceed 200 characters"}
		}
		in.Title = &trimmed
	}
	if in.Description != nil {
		trimmed := strings.TrimSpace(*in.Description)
		if utf8.RuneCountInString(trimmed) > 2000 {
			return in, &ValidationError{Field: "description", Reason: "must not exceed 2000 characters"}
		}
		in.Description = &trimmed
	}
	return in, nil
}

type PublicIDGenerator interface {
	New() (uuid.UUID, error)
}

type Repository interface {
	CreateBoard(ctx context.Context, b Board) (Board, error)
	GetBoardByPublicID(ctx context.Context, publicID string, userID int64) (Board, error)
	GetBoardByPublicIDAnyUser(ctx context.Context, publicID string) (Board, error)
	ListBoardsByUserID(ctx context.Context, userID int64) ([]Board, error)
	UpdateBoard(ctx context.Context, publicID string, userID int64, title, description string) (Board, error)
	DeleteBoard(ctx context.Context, publicID string, userID int64) error
}
