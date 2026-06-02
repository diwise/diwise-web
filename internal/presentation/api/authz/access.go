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

// TenantsWithScopes returns tenants that grant every requested scope.
func TenantsWithScopes(ctx context.Context, scopes ...Scope) []string {
	access, _ := AccessFromContext(ctx)
	tenants := FilterAccessByScopes(access, scopes...)

	result := make([]string, 0, len(tenants))
	for tenant := range tenants {
		result = append(result, tenant)
	}

	return result
}

// FilterAccessByScopes returns only tenants that have all requested scopes.
func FilterAccessByScopes(access AccessMap, scopes ...Scope) AccessMap {
	filteredAccess := AccessMap{}
	for tenant, allowedScopes := range access {
		if hasAllScopes(allowedScopes, scopes...) {
			filteredAccess[tenant] = allowedScopes
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
