package auth

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/open-policy-agent/opa/v1/rego"
)

type tokenContextKey struct{ name string }

var tokenCtxKey = &tokenContextKey{"jwt-token"}

type OptionalEnticator interface {
	Enticator
	OptionalAccess(scopes ...Scope) func(http.Handler) http.Handler
}

func NewOptionalAccess(ctx context.Context, policies io.Reader, opts ...Option) (OptionalEnticator, error) {
	module, err := io.ReadAll(policies)
	if err != nil {
		return nil, fmt.Errorf("unable to read authz policies: %s", err.Error())
	}

	authOptions := options{}
	for _, apply := range opts {
		apply(&authOptions)
	}

	query, err := rego.New(
		rego.Query("x = data.example.authz.allow"),
		rego.Module("example.rego", string(module)),
	).PrepareForEval(ctx)

	if err != nil {
		return nil, err
	}

	return &impl{query: query, accessObjectAuthz: authOptions.accessObjectAuthz}, nil
}

func (a *impl) OptionalAccess(scopes ...Scope) func(http.Handler) http.Handler {
	requiredScopes := normalizeRequiredScopes(scopes...)
	validateScopes := scopesAsStrings(requiredScopes)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, found := bearerToken(r)
			if token == "" || !found {
				next.ServeHTTP(w, r)
				return
			}

			accessObj, ok := a.optionalAccessFromToken(r, token, requiredScopes, validateScopes)
			if !ok {
				next.ServeHTTP(w, r)
				return
			}

			ctx := WithToken(r.Context(), token)
			ctx = WithAccess(ctx, accessObj)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func LoggedIn(ctx context.Context) bool {
	access, ok := ctx.Value(accessCtxKey).(accessMap)
	return ok && len(access) > 0
}

func (a *impl) optionalAccessFromToken(r *http.Request, token string, requiredScopes []Scope, validateScopes []string) (accessMap, bool) {
	path := strings.Split(r.URL.Path, "/")

	input := map[string]any{
		"method": r.Method,
		"path":   path[1:],
		"token":  token,
		"scopes": validateScopes,
	}

	results, err := a.query.Eval(r.Context(), rego.EvalInput(input))
	if err != nil || len(results) == 0 {
		return nil, false
	}

	binding := results[0].Bindings["x"]
	if allowed, ok := binding.(bool); ok && !allowed {
		return nil, false
	}

	result, ok := binding.(map[string]any)
	if !ok {
		return nil, false
	}

	accessObj, err := a.accessFromResult(result, requiredScopes)
	if err != nil || len(accessObj) == 0 {
		return nil, false
	}

	return accessObj, true
}
