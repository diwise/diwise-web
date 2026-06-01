package authz

import (
	"context"
	"fmt"

	"github.com/open-policy-agent/opa/v1/rego"
)

type accessResolver interface {
	ResolveAccess(ctx context.Context, token string) (AccessMap, error)
}

type opaAccessResolver struct {
	query rego.PreparedEvalQuery
}

func (r *opaAccessResolver) ResolveAccess(ctx context.Context, token string) (AccessMap, error) {
	results, err := r.query.Eval(ctx, rego.EvalInput(map[string]any{
		"token": token,
	}))
	if err != nil {
		return nil, fmt.Errorf("opa eval failed: %w", err)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("opa query returned no result")
	}

	binding, ok := results[0].Bindings["authz_result"]
	if !ok {
		return nil, fmt.Errorf("opa query did not bind authz_result")
	}

	return accessMapFromPolicyBinding(binding)
}

// accessMapFromPolicyBinding converts OPA's generic map/list result into Go types.
//
// OPA returns map[string]any and []any because policy data is dynamic.
// This helper validates the shape and converts strings into Scope values.
func accessMapFromPolicyBinding(binding any) (AccessMap, error) {
	result, ok := binding.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("expected authz policy to return object, got %T", binding)
	}

	anyAccess, ok := result["access"]
	if !ok {
		return nil, fmt.Errorf("authz policy response missing access")
	}

	access, ok := anyAccess.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("authz policy access has unexpected type %T", anyAccess)
	}

	accessObj := AccessMap{}
	for tenant, anyScopes := range access {
		scopeSlice, ok := anyScopes.([]any)
		if !ok {
			return nil, fmt.Errorf("authz policy scopes for tenant %q have unexpected type %T", tenant, anyScopes)
		}

		tenantScopes := map[Scope]struct{}{}
		for _, rawScope := range scopeSlice {
			scopeName, ok := rawScope.(string)
			if !ok {
				return nil, fmt.Errorf("authz policy scope for tenant %q has unexpected type %T", tenant, rawScope)
			}

			tenantScopes[Scope(scopeName)] = struct{}{}
		}

		accessObj[tenant] = tenantScopes
	}

	return accessObj, nil
}
