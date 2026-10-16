package ids

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGeneratorCreatesUniqueUUIDv7(t *testing.T) {
	g := Generator{}
	seen := map[uuid.UUID]bool{}
	for i := 0; i < 100; i++ {
		id, err := g.New()
		require.NoError(t, err)
		assert.Equal(t, uuid.Version(7), id.Version())
		assert.Equal(t, uuid.RFC4122, id.Variant())
		assert.False(t, seen[id], "duplicate UUID: %s", id)
		seen[id] = true
	}
}
