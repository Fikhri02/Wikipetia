// Package auth resolves a request to one of our own user ids.
//
// Slice 1 ships a development stub. A real provider (JWT verification) replaces
// DevStub behind the Identity interface without any other package changing,
// because everything downstream keys off our own users row, never the provider's
// subject. Per the spec, the real provider must land before slice 3.
package auth

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

type Identity interface {
	Subject(r *http.Request) (subject string, name string, err error)
}

type DevStub struct {
	DefaultName string
}

func (d DevStub) Subject(r *http.Request) (string, string, error) {
	name := r.Header.Get("X-Dev-User")
	if name == "" {
		name = d.DefaultName
	}
	return "dev|" + name, name, nil
}

type ctxKey struct{}

type ResolveFunc func(ctx context.Context, subject, name string) (uuid.UUID, error)

// ContextWithUser attaches a resolved user id. Middleware uses it in production;
// tests use it to run a request as a given user.
func ContextWithUser(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

func Middleware(next http.Handler, ident Identity, resolve ResolveFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}
		subject, name, err := ident.Subject(r)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		userID, err := resolve(r.Context(), subject, name)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(ContextWithUser(r.Context(), userID)))
	})
}

func UserFromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(ctxKey{}).(uuid.UUID)
	return id, ok
}

// MustUser is for handlers, which only run behind Middleware.
func MustUser(ctx context.Context) uuid.UUID {
	id, _ := UserFromContext(ctx)
	return id
}
