package httpapi

import (
	"oya-be/internal/board"
	"oya-be/internal/task"
)

type CreateBoardRequest struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
}

type UpdateBoardRequest struct {
	Title       *string `json:"title,omitempty"`
	Description *string `json:"description,omitempty"`
}

type BoardDTO struct {
	PublicID    string `json:"public_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

func newBoardDTO(b board.Board) BoardDTO {
	return BoardDTO{
		PublicID:    b.PublicID,
		Title:       b.Title,
		Description: b.Description,
		CreatedAt:   task.FormatTime(b.CreatedAt),
		UpdatedAt:   task.FormatTime(b.UpdatedAt),
	}
}

type BoardResponse struct {
	Data BoardDTO `json:"data"`
}

type ListBoardsResponse struct {
	Data []BoardDTO `json:"data"`
}
