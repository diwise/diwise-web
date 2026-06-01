package authz

import (
	"context"
)

type loggedInKey struct{}
type tokenKey struct{}
type accessContextKey struct{}
type authorizationContextLoadedKey struct{}

var (
	loggedInCtxKey                   loggedInKey
	authTokenCtxKey                  tokenKey
	accessCtxKey                     accessContextKey
	authorizationContextLoadedCtxKey authorizationContextLoadedKey
)

func IsLoggedIn(ctx context.Context) bool {
	value, _ := ctx.Value(loggedInCtxKey).(bool)
	return value
}

func Token(ctx context.Context) string {
	token, ok := ctx.Value(authTokenCtxKey).(string)
	if !ok {
		return ""
	}
	return token
}

// AccessFromContext returns authorization access stored on ctx, if any.
func AccessFromContext(ctx context.Context) (AccessMap, bool) {
	access, ok := ctx.Value(accessCtxKey).(AccessMap)
	return access, ok
}

func authorizationContextLoaded(ctx context.Context) bool {
	loaded, _ := ctx.Value(authorizationContextLoadedCtxKey).(bool)
	return loaded
}

func withAuthorizationContextLoaded(ctx context.Context) context.Context {
	return context.WithValue(ctx, authorizationContextLoadedCtxKey, true)
}

func withAccess(ctx context.Context, access AccessMap) context.Context {
	return context.WithValue(ctx, accessCtxKey, access)
}
