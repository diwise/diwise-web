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

	ReadAdmin Scope = "admin"
)

var AnyScope Scope = Scope("any")

type ContextLoader interface {
	WithAuthorizationContext(http.Handler) http.Handler
}

type Authorizer interface {
	RequireAccess(scopes ...Scope) func(http.Handler) http.Handler
}

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

type Option func(*authorizer)

func WithDeniedHandler(handler DeniedHandler) Option {
	return func(a *authorizer) {
		if handler != nil {
			a.deniedHandler = handler
		}
	}
}

func defaultDeniedHandler(w http.ResponseWriter, _ *http.Request, denial Denial) {
	http.Error(w, http.StatusText(denial.Status), denial.Status)
}

type authorizer struct {
	resolver      accessResolver
	deniedHandler DeniedHandler
}

func NewAuthorizer(ctx context.Context, policies io.Reader, opts ...Option) (*authorizer, error) {
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
		resolver:      &opaAccessResolver{query: query},
		deniedHandler: defaultDeniedHandler,
	}

	for _, opt := range opts {
		opt(a)
	}

	return a, nil
}
