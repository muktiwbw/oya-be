package task

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type Service struct {
	repo  TaskRepository
	clock Clock
	ids   PublicIDGenerator
	ttl   time.Duration
}

func NewService(repo TaskRepository, clock Clock, ids PublicIDGenerator, ttl time.Duration) *Service {
	return &Service{repo, clock, ids, ttl}
}

func (s *Service) CreateTask(ctx context.Context, userID int64, in CreateInput, key string) (CreateResult, error) {
	normalized, err := NormalizeCreate(in)
	if err != nil {
		return CreateResult{}, err
	}
	if len(key) > 128 || (key != "" && strings.TrimSpace(key) == "") {
		return CreateResult{}, &ValidationError{"idempotency_key", "must be non-empty and at most 128 bytes"}
	}
	id, err := s.ids.New()
	if err != nil {
		return CreateResult{}, fmt.Errorf("generate public ID: %w", err)
	}
	now := NormalizeTime(s.clock.Now())
	// A struct establishes a deterministic field order for the normalized hash.
	body, _ := json.Marshal(normalized)
	hash := sha256.Sum256(body)
	return s.repo.CreateTask(ctx, CreateRequest{
		UserID: userID,
		Task: Task{
			PublicID:    id.String(),
			BoardID:     normalized.BoardID,
			Title:       normalized.Title,
			Description: normalized.Description,
			Status:      normalized.Status,
			Version:     1,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		IdempotencyKey: key,
		RequestHash:    hex.EncodeToString(hash[:]),
		ExpiresBefore:  now.Add(-s.ttl),
	})
}

func (s *Service) GetTask(ctx context.Context, userID int64, id string) (Task, error) {
	if !ValidPublicID(id) {
		return Task{}, &ValidationError{"public_id", "must be a canonical UUIDv7"}
	}
	return s.repo.GetTaskByPublicID(ctx, id, userID)
}

func (s *Service) ListTasksByBoard(ctx context.Context, userID int64, boardID string, status Status, limit int, cursor string) (Page, error) {
	if !ValidPublicID(boardID) {
		return Page{}, &ValidationError{"board_id", "must be a canonical UUIDv7"}
	}
	if status != "" && !status.Valid() {
		return Page{}, &ValidationError{"status", "must be todo, doing, or done"}
	}
	if limit < 1 || limit > 100 {
		return Page{}, &ValidationError{"limit", "must be between 1 and 100"}
	}
	q := ListQuery{Status: status, Limit: limit + 1}
	if cursor != "" {
		p, err := DecodeCursor(cursor, status)
		if err != nil {
			return Page{}, err
		}
		q.After = &p
	}
	rows, err := s.repo.ListTasksByBoard(ctx, boardID, userID, q)
	if err != nil {
		return Page{}, err
	}
	p := Page{Tasks: rows, Limit: limit}
	if p.Tasks == nil {
		p.Tasks = []Task{}
	}
	if len(rows) > limit {
		p.HasMore = true
		p.Tasks = rows[:limit]
		last := p.Tasks[len(p.Tasks)-1]
		token, err := EncodeCursor(Position{last.CreatedAt, last.PublicID}, status)
		if err != nil {
			return Page{}, err
		}
		p.NextCursor = &token
	}
	return p, nil
}

func (s *Service) UpdateTaskStatus(ctx context.Context, userID int64, id string, in UpdateStatusInput) (Task, error) {
	if !ValidPublicID(id) {
		return Task{}, &ValidationError{"public_id", "must be a canonical UUIDv7"}
	}
	if err := in.Validate(); err != nil {
		return Task{}, err
	}
	return s.repo.UpdateTaskStatusByPublicID(ctx, id, userID, in, NormalizeTime(s.clock.Now()))
}

func (s *Service) UpdateTask(ctx context.Context, userID int64, id string, in UpdateTaskInput) (Task, error) {
	if !ValidPublicID(id) {
		return Task{}, &ValidationError{"public_id", "must be a canonical UUIDv7"}
	}
	normalized, err := NormalizeUpdate(in)
	if err != nil {
		return Task{}, err
	}
	return s.repo.UpdateTaskByPublicID(ctx, id, userID, normalized, NormalizeTime(s.clock.Now()))
}

func (s *Service) DeleteTask(ctx context.Context, userID int64, id string) error {
	if !ValidPublicID(id) {
		return &ValidationError{"public_id", "must be a canonical UUIDv7"}
	}
	return s.repo.DeleteTaskByPublicID(ctx, id, userID)
}
