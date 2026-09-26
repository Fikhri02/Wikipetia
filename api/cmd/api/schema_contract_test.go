package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The outline is the builder's only read, and every field it uses must appear in
// the generated schema — that schema is what types the web client. An embedded
// UNEXPORTED struct is promoted by encoding/json at runtime but invisible to
// huma's reflection, so the contract silently omits fields the server sends,
// while additionalProperties:false formally forbids them.
func TestOutlineBlockSchemaCarriesTheBlockFields(t *testing.T) {
	_, api := newAPI(nil, "dev")

	schema, ok := api.OpenAPI().Components.Schemas.Map()["OutlineBlockBody"]
	require.True(t, ok, "OutlineBlockBody schema must exist")

	for _, field := range []string{"id", "module_id", "title", "type", "sort_key", "content", "completion"} {
		_, present := schema.Properties[field]
		assert.True(t, present,
			"OutlineBlockBody must declare %q — the server sends it and the web builder reads it", field)
	}

	// The fields this type adds itself must still be there.
	for _, field := range []string{"topic_ids", "question_count"} {
		_, present := schema.Properties[field]
		assert.True(t, present, "OutlineBlockBody must declare %q", field)
	}
}
