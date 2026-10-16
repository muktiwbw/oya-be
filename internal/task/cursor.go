package task

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrInvalidCursor = errors.New("invalid cursor")

type Position struct {
	CreatedAt time.Time
	PublicID  string
}

type cursorPayload struct {
	Version   int    `json:"v"`
	CreatedAt string `json:"created_at"`
	PublicID  string `json:"public_id"`
	Status    Status `json:"status"`
}

func ValidPublicID(value string) bool {
	id, err := uuid.Parse(value)
	return err == nil && id.Version() == 7 && id.Variant() == uuid.RFC4122 && id.String() == value
}

func EncodeCursor(p Position, filter Status) (string, error) {
	if !ValidPublicID(p.PublicID) || p.CreatedAt.IsZero() || (filter != "" && !filter.Valid()) {
		return "", ErrInvalidCursor
	}
	b, err := json.Marshal(cursorPayload{1, FormatTime(p.CreatedAt), p.PublicID, filter})
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func DecodeCursor(token string, filter Status) (Position, error) {
	var p Position
	if len(token) > 2048 || token == "" {
		return p, ErrInvalidCursor
	}
	b, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return p, ErrInvalidCursor
	}
	var payload cursorPayload
	if err = json.Unmarshal(b, &payload); err != nil || payload.Version != 1 || payload.Status != filter || (filter != "" && !filter.Valid()) || !ValidPublicID(payload.PublicID) {
		return p, ErrInvalidCursor
	}
	t, err := time.Parse("2006-01-02T15:04:05.000Z", payload.CreatedAt)
	if err != nil || t.IsZero() || FormatTime(t) != payload.CreatedAt {
		return p, ErrInvalidCursor
	}
	return Position{t, payload.PublicID}, nil
}
