package authz

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
)

func (a *authorizer) WithAuthorizationContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req, status, err := a.requestWithAuthorizationContext(r)
		if err != nil {
			http.Error(w, http.StatusText(status), status)
			return
		}

		next.ServeHTTP(w, req)
	})
}

func (a *authorizer) RequireAccess(scopes ...Scope) func(http.Handler) http.Handler {
	requiredScopes := append([]Scope(nil), scopes...)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !IsLoggedIn(r.Context()) {
				a.deny(w, r, Denial{
					Status:         http.StatusUnauthorized,
					Reason:         DenialReasonUnauthenticated,
					RequiredScopes: requiredScopes,
				})
				return
			}

			filteredAccess := accessWithRequiredScopes(r.Context(), requiredScopes...)
			if len(filteredAccess) == 0 {
				a.deny(w, r, Denial{
					Status:         http.StatusForbidden,
					Reason:         DenialReasonForbidden,
					RequiredScopes: requiredScopes,
				})
				return
			}

			next.ServeHTTP(w, r.WithContext(WithAccess(r.Context(), filteredAccess)))
		})
	}
}

func (a *authorizer) deny(w http.ResponseWriter, r *http.Request, denial Denial) {
	handler := a.deniedHandler
	if handler == nil {
		handler = defaultDeniedHandler
	}

	handler(w, r, denial)
}

// requestWithAuthorizationContext enriches request context from Authorization.
// It is not an access gate. Route authorization belongs in RequireAccess.
func (a *authorizer) requestWithAuthorizationContext(r *http.Request) (*http.Request, int, error) {
	ctx := context.WithValue(r.Context(), LoggedIn, "no")

	token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok || strings.TrimSpace(token) == "" {
		return r.WithContext(ctx), 0, nil
	}

	ctx = context.WithValue(ctx, LoggedIn, "yes")
	ctx = context.WithValue(ctx, AuthToken, token)

	access, err := a.resolver.ResolveAccess(ctx, token)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("resolve access: %w", err)
	}

	data, _ := json.Marshal(accessObjectForLog(access))

	logging.GetFromContext(ctx).Info(
		"resolved authorization access",
		"access", string(data),
	)

	ctx = WithAccess(ctx, access)
	return r.WithContext(ctx), 0, nil
}

// accessWithRequiredScopes returns only tenants that have all requested scopes.
func accessWithRequiredScopes(ctx context.Context, scopes ...Scope) accessMap {
	access, ok := ctx.Value(accessCtxKey).(accessMap)
	if !ok || len(scopes) == 0 {
		return accessMap{}
	}

	filteredAccess := accessMap{}
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

func accessObjectForLog(access accessMap) map[string][]string {
	accessObject := make(map[string][]string, len(access))

	for tenant, scopes := range access {
		tenantScopes := make([]string, 0, len(scopes))
		for scope := range scopes {
			tenantScopes = append(tenantScopes, string(scope))
		}

		slices.Sort(tenantScopes)
		accessObject[tenant] = tenantScopes
	}

	return accessObject
}
