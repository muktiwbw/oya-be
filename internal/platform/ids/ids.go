// Package ids supplies UUIDv7 public identifiers.
package ids

import "github.com/google/uuid"

type Generator struct{}

func (Generator) New() (uuid.UUID, error) { return uuid.NewV7() }
