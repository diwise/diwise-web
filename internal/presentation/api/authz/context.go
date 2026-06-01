package authz

import (
	"context"
	"errors"
	"fmt"
)

type AccessMap map[string]map[Scope]struct{}

var ErrAccessDenied = errors.New("access denied")

type AccessDeniedError struct {
	Tenant string
	Scope  Scope
}

func (e AccessDeniedError) Error() string {
	if e.Tenant == "" {
		return fmt.Sprintf("access denied for scope %q", e.Scope)
	}
	return fmt.Sprintf("access denied for tenant %q and scope %q", e.Tenant, e.Scope)
}

func (e AccessDeniedError) Is(target error) bool {
	return target == ErrAccessDenied
}

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

func HasTenantAccess(ctx context.Context, tenant string, scopes ...Scope) bool {
	if tenant == "" || len(scopes) == 0 {
		return false
	}

	access, ok := ctx.Value(accessCtxKey).(AccessMap)
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

func RequireAccessForContext(ctx context.Context, scopes ...Scope) error {
	if len(scopes) == 0 {
		return AccessDeniedError{}
	}

	access, ok := ctx.Value(accessCtxKey).(AccessMap)
	if !ok {
		return AccessDeniedError{Scope: scopes[0]}
	}

	for _, allowedScopes := range access {
		hasAllScopes := true

		for _, requiredScope := range scopes {
			if _, ok := allowedScopes[requiredScope]; !ok {
				hasAllScopes = false
				break
			}
		}

		if hasAllScopes {
			return nil
		}
	}

	return AccessDeniedError{Scope: scopes[0]}
}

func RequireTenantAccessForContext(ctx context.Context, tenant string, scope Scope) error {
	if tenant == "" || !HasTenantAccess(ctx, tenant, scope) {
		return AccessDeniedError{Tenant: tenant, Scope: scope}
	}
	return nil
}

// GetTenantsWithAllowedScopes extracts the names of allowed tenants, if any, from the provided context
func GetTenantsWithAllowedScopes(ctx context.Context, scopes ...Scope) []string {
	access, ok := ctx.Value(accessCtxKey).(AccessMap)
	requiredScopeCount := len(scopes)

	if !ok || requiredScopeCount == 0 {
		return []string{}
	}

	// If the required scope is AnyScope we set the scope count to
	// 0 to disable the scope checking below
	if requiredScopeCount == 1 && scopes[0] == AnyScope {
		requiredScopeCount = 0
	}

	tenants := make([]string, 0, len(access))

	for t, allowedScopes := range access {
		idx := 0

		for idx < requiredScopeCount {
			if _, ok := allowedScopes[scopes[idx]]; !ok {
				break
			}
			idx++
		}

		if idx == requiredScopeCount {
			tenants = append(tenants, t)
		}
	}

	return tenants
}

// AccessWithRequiredScopes returns only tenants that have all requested scopes.
func AccessWithRequiredScopes(ctx context.Context, scopes ...Scope) AccessMap {
	access, ok := ctx.Value(accessCtxKey).(AccessMap)
	if !ok || len(scopes) == 0 {
		return AccessMap{}
	}

	filteredAccess := AccessMap{}
	for tenant, allowedScopes := range access {
		hasAllScopes := true

		for _, requiredScope := range scopes {
			if _, ok := allowedScopes[requiredScope]; !ok {
				hasAllScopes = false
				break
			}
		}

		if hasAllScopes {
			filteredAccess[tenant] = allowedScopes
		}
	}

	return filteredAccess
}

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
