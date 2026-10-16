package httpapi

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/require"
)

func TestOpenAPIAndCreateResponse(t *testing.T) {
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile(filepath.Join("..", "..", "..", "resources", "api", "spec.yaml"))
	require.NoError(t, err)
	require.NoError(t, doc.Validate(context.Background()))
	w := request(testHandler(t), "POST", "/api/v1/tasks", `{"title":"contract example"}`, "")
	var body any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	schema := doc.Components.Schemas["TaskEnvelope"].Value
	require.NoError(t, schema.VisitJSON(body))
}
