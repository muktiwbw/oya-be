package httpapi

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/require"
)

func TestOpenAPISpecValidation(t *testing.T) {
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile(filepath.Join("..", "..", "..", "resources", "api", "spec.yaml"))
	require.NoError(t, err)
	require.NoError(t, doc.Validate(context.Background()))

	taskEnvelope := map[string]any{
		"data": map[string]any{
			"public_id":   "018f4b7e-3b75-7e1f-b5d5-2b2d7b0a7f12",
			"board_id":    "018f4b7e-3b75-7e1f-b5d5-2b2d7b0a7f13",
			"title":       "Test Task",
			"description": "Task description",
			"status":      "todo",
			"version":     1,
			"created_at":  "2026-10-16T10:15:30.000Z",
			"updated_at":  "2026-10-16T10:15:30.000Z",
		},
	}
	schema := doc.Components.Schemas["TaskEnvelope"].Value
	require.NoError(t, schema.VisitJSON(taskEnvelope))

	boardEnvelope := map[string]any{
		"data": map[string]any{
			"public_id":   "018f4b7e-3b75-7e1f-b5d5-2b2d7b0a7f13",
			"title":       "Main Board",
			"description": "Board description",
			"created_at":  "2026-10-16T10:15:30.000Z",
			"updated_at":  "2026-10-16T10:15:30.000Z",
		},
	}
	boardSchema := doc.Components.Schemas["BoardEnvelope"].Value
	require.NoError(t, boardSchema.VisitJSON(boardEnvelope))
}
