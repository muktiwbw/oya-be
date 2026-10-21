package board

import (
	"context"
	"fmt"
	"time"
)

type Service struct {
	repo Repository
	ids  PublicIDGenerator
}

func NewService(repo Repository, ids PublicIDGenerator) *Service {
	return &Service{
		repo: repo,
		ids:  ids,
	}
}

func (s *Service) Create(ctx context.Context, userID int64, in CreateInput) (Board, error) {
	validated, err := in.Validate()
	if err != nil {
		return Board{}, err
	}
	publicID, err := s.ids.New()
	if err != nil {
		return Board{}, fmt.Errorf("generate board public id: %w", err)
	}
	now := time.Now().UTC()
	b := Board{
		PublicID:    publicID.String(),
		UserID:      userID,
		Title:       validated.Title,
		Description: validated.Description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	return s.repo.CreateBoard(ctx, b)
}

func (s *Service) Get(ctx context.Context, userID int64, publicID string) (Board, error) {
	return s.repo.GetBoardByPublicID(ctx, publicID, userID)
}

func (s *Service) List(ctx context.Context, userID int64) ([]Board, error) {
	return s.repo.ListBoardsByUserID(ctx, userID)
}

func (s *Service) Update(ctx context.Context, userID int64, publicID string, in UpdateInput) (Board, error) {
	validated, err := in.Validate()
	if err != nil {
		return Board{}, err
	}
	existing, err := s.repo.GetBoardByPublicID(ctx, publicID, userID)
	if err != nil {
		return Board{}, err
	}
	title := existing.Title
	if validated.Title != nil {
		title = *validated.Title
	}
	desc := existing.Description
	if validated.Description != nil {
		desc = *validated.Description
	}
	return s.repo.UpdateBoard(ctx, publicID, userID, title, desc)
}

func (s *Service) Delete(ctx context.Context, userID int64, publicID string) error {
	return s.repo.DeleteBoard(ctx, publicID, userID)
}
