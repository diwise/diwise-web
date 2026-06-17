package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/diwise/diwise-web/internal/presentation/api"
	"github.com/diwise/diwise-web/internal/presentation/api/auth"
	"github.com/matryer/is"
)

func TestAuthPoliciesUsesDevModePolicyWithoutOpeningConfiguredFile(t *testing.T) {
	is := is.New(t)
	flags := DefaultFlags()
	flags[devModeEnabled] = "true"
	flags[policiesFile] = filepath.Join(t.TempDir(), "missing.rego")

	policies, err := authPolicies(flags)
	is.NoErr(err)
	defer policies.Close()

	body, err := io.ReadAll(policies)
	is.NoErr(err)
	is.True(strings.Contains(string(body), `input.token == "devmode"`))
}

func TestAuthPoliciesRequiresConfiguredFileOutsideDevMode(t *testing.T) {
	is := is.New(t)
	flags := DefaultFlags()
	flags[policiesFile] = filepath.Join(t.TempDir(), "missing.rego")

	policies, err := authPolicies(flags)
	is.True(err != nil)
	if policies != nil {
		policies.Close()
	}
}

func TestDevModeAuthzPolicyAllowsProtectedRouteScopes(t *testing.T) {
	is := is.New(t)
	authenticator, err := auth.NewAuthenticator(
		context.Background(),
		strings.NewReader(devModeAuthzPolicy),
		auth.WithAccessObjectAuthorization(true),
	)
	is.NoErr(err)

	for _, scope := range []auth.Scope{
		api.ReadSensors,
		api.UpdateSensors,
		api.ReadThings,
		api.CreateThings,
		api.UpdateThings,
		api.DeleteThings,
		api.Admin,
	} {
		t.Run(string(scope), func(t *testing.T) {
			is := is.New(t)
			handler := authenticator.RequireAccess(scope)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			}))

			req := httptest.NewRequest(http.MethodGet, "/devmode", nil)
			req.Header.Set("Authorization", "Bearer devmode")
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			is.Equal(rec.Code, http.StatusNoContent)
		})
	}
}
