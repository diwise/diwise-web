package authz

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/open-policy-agent/opa/v1/rego"
)

type Scope string

const (
	ReadSensors   Scope = "sensors.read"
	UpdateSensors Scope = "sensors.update"

	ReadThings   Scope = "things.read"
	CreateThings Scope = "things.create"
	UpdateThings Scope = "things.update"
	DeleteThings Scope = "things.delete"

	Admin Scope = "admin"
)

type DenialReason string

const (
	DenialReasonUnauthenticated DenialReason = "unauthenticated"
	DenialReasonForbidden       DenialReason = "forbidden"
)

type Denial struct {
	Status         int
	Reason         DenialReason
	RequiredScopes []Scope
}

type DeniedHandler func(http.ResponseWriter, *http.Request, Denial)

type AuthenticationBypass func(*http.Request) bool

// TenantResolver resolves the tenant for a request before tenant-scoped
// authorization is evaluated.
type TenantResolver func(context.Context, *http.Request) (string, error)

type Authorizer interface {
	RequireAccess(scopes ...Scope) func(http.Handler) http.Handler
	RequireTenantAccess(scope Scope, resolve TenantResolver) func(http.Handler) http.Handler
}

type Option func(*authorizer)

func WithDeniedHandler(handler DeniedHandler) Option {
	return func(a *authorizer) {
		if handler != nil {
			a.deniedHandler = handler
		}
	}
}

func NewAuthorizer(ctx context.Context, policies io.Reader, opts ...Option) (Authorizer, error) {
	module, err := io.ReadAll(policies)
	if err != nil {
		return nil, fmt.Errorf("unable to read authz policies: %w", err)
	}

	query, err := rego.New(
		rego.Query("authz_result = data.example.authz.allow"),
		rego.Module("example.rego", string(module)),
	).PrepareForEval(ctx)
	if err != nil {
		return nil, err
	}

	a := &authorizer{
		accessMapResolver: &opaAccessResolver{query: query},
		deniedHandler:     defaultDeniedHandler,
	}

	for _, opt := range opts {
		opt(a)
	}

	return a, nil
}

type authorizer struct {
	accessMapResolver accessResolver
	deniedHandler     DeniedHandler
}

func defaultDeniedHandler(w http.ResponseWriter, _ *http.Request, denial Denial) {
	http.Error(w, http.StatusText(denial.Status), denial.Status)
}
