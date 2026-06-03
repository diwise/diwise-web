package authz

import "context"

type AccessMap map[string]map[Scope]struct{}

// RequireTenantAccess returns nil if tenant has the requested scope.
func RequireTenantAccess(ctx context.Context, tenant string, scope Scope) error {
	access, _ := AccessFromContext(ctx)
	if tenant == "" || !hasAllScopes(access[tenant], scope) {
		return AccessDeniedError{Tenant: tenant, Scope: scope}
	}
	return nil
}

func HasAccess(ctx context.Context, scopes ...Scope) bool {
	filteredAccess := FilterAccessByScopes(ctx, scopes...)
	if len(filteredAccess) == 0 {
		return false
	}
	return true
}

// TenantsWithScopes returns tenants that grant every requested scope.
func TenantsWithScopes(ctx context.Context, scopes ...Scope) []string {
	tenants := FilterAccessByScopes(ctx, scopes...)

	result := make([]string, 0, len(tenants))
	for tenant := range tenants {
		result = append(result, tenant)
	}

	return result
}

// FilterAccessByScopes returns only tenants that have all requested scopes.
func FilterAccessByScopes(ctx context.Context, scopes ...Scope) AccessMap {
	access, _ := AccessFromContext(ctx)
	filteredAccess := AccessMap{}
	for tenant, tenenatScopes := range access {
		if hasAllScopes(tenenatScopes, scopes...) {
			filteredAccess[tenant] = tenenatScopes
		}
	}

	return filteredAccess
}

func hasAllScopes(allowedScopes map[Scope]struct{}, scopes ...Scope) bool {
	if len(scopes) == 0 {
		return false
	}

	for _, requiredScope := range scopes {
		if _, ok := allowedScopes[requiredScope]; !ok {
			return false
		}
	}

	return true
}
