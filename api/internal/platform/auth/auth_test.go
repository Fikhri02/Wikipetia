package auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Fikhri02/wikipetia/internal/platform/auth"
)

func TestDevStubUsesHeaderThenDefault(t *testing.T) {
	stub := auth.DevStub{DefaultName: "dev"}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Dev-User", "irfan")
	subject, name, err := stub.Subject(r)
	require.NoError(t, err)
	assert.Equal(t, "dev|irfan", subject)
	assert.Equal(t, "irfan", name)

	plain := httptest.NewRequest(http.MethodGet, "/", nil)
	subject, name, err = stub.Subject(plain)
	require.NoError(t, err)
	assert.Equal(t, "dev|dev", subject)
	assert.Equal(t, "dev", name)
}

func TestMiddlewarePutsUserOnContext(t *testing.T) {
	want := uuid.New()
	resolve := func(ctx context.Context, subject, name string) (uuid.UUID, error) {
		assert.Equal(t, "dev|irfan", subject)
		return want, nil
	}

	var got uuid.UUID
	var ok bool
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, ok = auth.UserFromContext(r.Context())
	})

	h := auth.Middleware(inner, auth.DevStub{DefaultName: "dev"}, resolve)
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Dev-User", "irfan")
	h.ServeHTTP(httptest.NewRecorder(), r)

	require.True(t, ok, "user id must be on the context")
	assert.Equal(t, want, got)
}

func TestMiddlewareReturns401WhenResolveFails(t *testing.T) {
	resolve := func(ctx context.Context, subject, name string) (uuid.UUID, error) {
		return uuid.Nil, assert.AnError
	}
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("inner handler must not run")
	})

	rec := httptest.NewRecorder()
	auth.Middleware(inner, auth.DevStub{DefaultName: "dev"}, resolve).
		ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestUserFromContextEmptyWhenAbsent(t *testing.T) {
	_, ok := auth.UserFromContext(context.Background())
	assert.False(t, ok)
}
