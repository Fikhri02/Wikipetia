package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/Fikhri02/wikipetia/internal/platform/auth"
	"github.com/Fikhri02/wikipetia/internal/platform/db/pgtest"
	"github.com/Fikhri02/wikipetia/internal/platform/sqlcgen"
	"github.com/Fikhri02/wikipetia/internal/user"
)

// newTestAPI returns a huma test API whose every request runs as `subject`,
// inside the test's transaction (rolled back on cleanup).
func newTestAPI(t *testing.T, subject string) (humatest.TestAPI, uuid.UUID) {
	t.Helper()
	ctx, runner := pgtest.NewRunner(t)

	store := user.Store{}
	var userID uuid.UUID
	require.NoError(t, runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		var err error
		userID, err = store.EnsureBySubject(ctx, q, "dev|"+subject, subject)
		return err
	}))

	_, api := humatest.New(t, huma.DefaultConfig("test", "0.1.0"))
	api.UseMiddleware(func(hctx huma.Context, next func(huma.Context)) {
		next(huma.WithContext(hctx, auth.ContextWithUser(hctx.Context(), userID)))
	})

	srv := &server{runner: runner}
	srv.registerRoutes(api)

	return api, userID
}

// newCourse creates a course over HTTP and returns its id.
func newCourse(t *testing.T, api humatest.TestAPI, title string) string {
	t.Helper()
	resp := api.Post("/courses", map[string]any{"title": title})
	require.Equal(t, http.StatusCreated, resp.Code, resp.Body.String())
	return idFromBody(t, resp.Body.Bytes())
}

// indexOfTitle reports where a title appears in a JSON body, for order assertions.
func indexOfTitle(body, title string) int {
	return strings.Index(body, `"title":"`+title+`"`)
}

func idFromBody(t *testing.T, body []byte) string {
	t.Helper()
	var parsed struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	require.NotEmpty(t, parsed.ID)
	return parsed.ID
}
