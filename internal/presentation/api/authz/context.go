package authz

import "context"

type loggedInKey string
type tokenKey string
type accessContextKey struct{ name string }

const AuthToken tokenKey = "jwt-token"
const LoggedIn loggedInKey = "logged-in"

var accessCtxKey = &accessContextKey{"access"}

type accessMap map[string]map[Scope]struct{}

func IsLoggedIn(ctx context.Context) bool {
	value, ok := ctx.Value(LoggedIn).(string)
	return ok && value == "yes"
}

func Token(ctx context.Context) string {
	token, ok := ctx.Value(AuthToken).(string)
	if !ok {
		return ""
	}
	return token
}

func WithAccess(ctx context.Context, access accessMap) context.Context {
	return context.WithValue(ctx, accessCtxKey, access)
}

func HasTenantAccess(ctx context.Context, tenant string, scopes ...Scope) bool {
	if tenant == "" || len(scopes) == 0 {
		return false
	}

	access, ok := ctx.Value(accessCtxKey).(accessMap)
	if !ok {
		return false
	}

	allowedScopes, ok := access[tenant]
	if !ok {
		return false
	}

	for _, requiredScope := range scopes {
		if _, ok := allowedScopes[requiredScope]; !ok {
			return false
		}
	}

	return true
}
