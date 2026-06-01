package authz

type AccessMap map[string]map[Scope]struct{}

// HasTenantAccess reports whether access grants every requested scope for tenant.
func HasTenantAccess(access AccessMap, tenant string, scopes ...Scope) bool {
	if tenant == "" || len(scopes) == 0 {
		return false
	}

	allowedScopes, ok := access[tenant]
	if !ok {
		return false
	}

	for _, requiredScope := range scopes {
		if _, ok := allowedScopes[requiredScope]; !ok {
			return false
		}
	}

	return true
}

// RequireAccess returns nil if any tenant grants every requested scope.
func RequireAccess(access AccessMap, scopes ...Scope) error {
	if len(scopes) == 0 {
		return AccessDeniedError{}
	}

	for _, allowedScopes := range access {
		hasAllScopes := true

		for _, requiredScope := range scopes {
			if _, ok := allowedScopes[requiredScope]; !ok {
				hasAllScopes = false
				break
			}
		}

		if hasAllScopes {
			return nil
		}
	}

	return AccessDeniedError{Scope: scopes[0]}
}

// RequireTenantAccess returns nil if tenant has the requested scope.
func RequireTenantAccess(access AccessMap, tenant string, scope Scope) error {
	if tenant == "" || !HasTenantAccess(access, tenant, scope) {
		return AccessDeniedError{Tenant: tenant, Scope: scope}
	}
	return nil
}

// TenantsWithScopes returns tenants that grant every requested scope.
func TenantsWithScopes(access AccessMap, scopes ...Scope) []string {
	tenants := FilterAccessByScopes(access, scopes...)

	result := make([]string, 0, len(tenants))
	for tenant := range tenants {
		result = append(result, tenant)
	}

	return result
}

// FilterAccessByScopes returns only tenants that have all requested scopes.
func FilterAccessByScopes(access AccessMap, scopes ...Scope) AccessMap {
	if len(scopes) == 0 {
		return AccessMap{}
	}

	filteredAccess := AccessMap{}
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
