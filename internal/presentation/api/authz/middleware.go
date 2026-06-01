package authz

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

func (a *authorizer) RequireAuthentication(bypass AuthenticationBypass) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			req, status, err := a.ensureAuthorizationContext(r)
			if err != nil {
				http.Error(w, http.StatusText(status), status)
				return
			}

			if IsLoggedIn(req.Context()) || (bypass != nil && bypass(req)) {
				next.ServeHTTP(w, req)
				return
			}

			a.denyUnauthenticated(w, req)
		})
	}
}

func (a *authorizer) RequireAccess(scopes ...Scope) func(http.Handler) http.Handler {
	requiredScopes := append([]Scope(nil), scopes...)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			req, status, err := a.ensureAuthorizationContext(r)
			if err != nil {
				http.Error(w, http.StatusText(status), status)
				return
			}

			if !IsLoggedIn(req.Context()) {
				a.denyUnauthenticated(w, req)
				return
			}

			if err := RequireAccessForContext(req.Context(), requiredScopes...); err != nil {
				a.deny(w, req, Denial{
					Status:         http.StatusForbidden,
					Reason:         DenialReasonForbidden,
					RequiredScopes: requiredScopes,
				})
				return
			}

			next.ServeHTTP(w, req)
		})
	}
}

func (a *authorizer) RequireTenantAccess(scope Scope, resolve TenantResolver) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			req, status, err := a.ensureAuthorizationContext(r)
			if err != nil {
				http.Error(w, http.StatusText(status), status)
				return
			}

			if !IsLoggedIn(req.Context()) {
				a.denyUnauthenticated(w, req)
				return
			}

			tenant, err := resolve(req.Context(), req)
			if err != nil {
				http.Error(w, "could not resolve tenant", http.StatusInternalServerError)
				return
			}

			if err := RequireTenantAccessForContext(req.Context(), tenant, scope); err != nil {
				a.deny(w, req, Denial{
					Status:         http.StatusForbidden,
					Reason:         DenialReasonForbidden,
					RequiredScopes: []Scope{scope},
				})
				return
			}

			next.ServeHTTP(w, req)
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

func (a *authorizer) denyUnauthenticated(w http.ResponseWriter, r *http.Request) {
	a.deny(w, r, Denial{
		Status: http.StatusUnauthorized,
		Reason: DenialReasonUnauthenticated,
	})
}

// ensureAuthorizationContext enriches request context from Authorization once per request.
// It is not an access gate. Route authorization belongs in RequireAccess.
func (a *authorizer) ensureAuthorizationContext(r *http.Request) (*http.Request, int, error) {
	if authorizationContextLoaded(r.Context()) {
		return r, 0, nil
	}

	ctx := withAuthorizationContextLoaded(context.WithValue(r.Context(), loggedInCtxKey, false))

	token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok || strings.TrimSpace(token) == "" {
		return r.WithContext(ctx), 0, nil
	}

	ctx = context.WithValue(ctx, loggedInCtxKey, true)
	ctx = context.WithValue(ctx, authTokenCtxKey, token)

	access, err := a.accessMapResolver.ResolveAccess(ctx, token)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("resolve access: %w", err)
	}

	ctx = withAccess(ctx, access)
	return r.WithContext(ctx), 0, nil
}
