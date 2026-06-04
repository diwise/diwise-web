package authz

import (
	"context"
	"net/http"
	"strings"
)

type loggedInKey string
type tokenKey string

const AuthToken tokenKey = "jwt-token"
const LoggedIn loggedInKey = "logged-in"

func NewContextFromAuthorizationHeader(ctx context.Context, r *http.Request) (context.Context, error) {
	var found bool
	authHeader := r.Header.Get("Authorization")
	if authHeader, found = strings.CutPrefix(authHeader, "Bearer "); !found {
		authHeader, _ = strings.CutPrefix(authHeader, "bearer ")
	}

	if authHeader != "" {
		ctx = context.WithValue(ctx, LoggedIn, "yes")
		ctx = context.WithValue(ctx, AuthToken, authHeader)
	}

	return ctx, nil
}

func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, err := NewContextFromAuthorizationHeader(r.Context(), r)
		if err == nil {
			r = r.WithContext(ctx)
		}
		next.ServeHTTP(w, r)
	})
}

func IsLoggedIn(ctx context.Context) bool {
	if value, ok := ctx.Value(LoggedIn).(string); ok {
		return value == "yes"
	}
	return false
}

func Token(ctx context.Context) string {
	if token, ok := ctx.Value(AuthToken).(string); ok {
		return token
	}

	return ""
}
