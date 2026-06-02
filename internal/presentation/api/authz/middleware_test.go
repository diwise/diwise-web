package authz

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/matryer/is"
)

func authorizedRequest(access AccessMap) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := withAuthorizationContextLoaded(context.WithValue(req.Context(), loggedInCtxKey, true))
	ctx = withAccess(ctx, access)
	return req.WithContext(ctx)
}

func TestRequireAccessMiddlewareFiltersContextAccess(t *testing.T) {
	is := is.New(t)

	access := AccessMap{
		"tenant-a": map[Scope]struct{}{
			ReadSensors:   struct{}{},
			UpdateSensors: struct{}{},
		},
		"tenant-b": map[Scope]struct{}{
			UpdateSensors: struct{}{},
		},
		"tenant-c": map[Scope]struct{}{
			ReadSensors: struct{}{},
		},
	}

	a := &authorizer{}
	var filteredAccess AccessMap

	handler := a.RequireAccess(ReadSensors)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var ok bool
		filteredAccess, ok = AccessFromContext(r.Context())
		is.True(ok)
		w.WriteHeader(http.StatusNoContent)
	}))

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authorizedRequest(access))

	is.Equal(http.StatusNoContent, response.Code)
	is.Equal(AccessMap{
		"tenant-a": map[Scope]struct{}{
			ReadSensors:   struct{}{},
			UpdateSensors: struct{}{},
		},
		"tenant-c": map[Scope]struct{}{
			ReadSensors: struct{}{},
		},
	}, filteredAccess)
}

func TestRequireAccessMiddlewareDeniesWhenNoTenantMatches(t *testing.T) {
	is := is.New(t)

	access := AccessMap{
		"tenant-a": map[Scope]struct{}{
			UpdateSensors: struct{}{},
		},
	}

	a := &authorizer{}
	called := false

	handler := a.RequireAccess(ReadSensors)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authorizedRequest(access))

	is.Equal(http.StatusForbidden, response.Code)
	is.True(!called)
}
