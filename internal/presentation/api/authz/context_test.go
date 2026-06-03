package authz

import (
	"context"
	"errors"
	"testing"

	"github.com/matryer/is"
)

func TestAccessFromContextReturnsStoredAccess(t *testing.T) {
	is := is.New(t)

	access := AccessMap{
		"tenant-a": map[Scope]struct{}{
			ReadSensors: struct{}{},
		},
	}

	got, ok := AccessFromContext(WithAccess(context.Background(), access))

	is.True(ok)
	is.Equal(access, got)
}

func TestRequireTenantAccessDeniesMissingTenant(t *testing.T) {
	is := is.New(t)

	access := AccessMap{
		"tenant-a": map[Scope]struct{}{
			UpdateSensors: struct{}{},
		},
	}

	err := RequireTenantAccess(WithAccess(context.Background(), access), "tenant-b", UpdateSensors)

	is.True(errors.Is(err, ErrAccessDenied))
	is.Equal(AccessDeniedError{Tenant: "tenant-b", Scope: UpdateSensors}, err)
}

func TestTenantsWithScopesReturnsTenantsFromContext(t *testing.T) {
	is := is.New(t)

	access := AccessMap{
		"tenant-a": map[Scope]struct{}{
			UpdateSensors: struct{}{},
		},
		"tenant-b": map[Scope]struct{}{
			ReadSensors: struct{}{},
		},
	}

	tenants := TenantsWithScopes(WithAccess(context.Background(), access), UpdateSensors)

	is.Equal([]string{"tenant-a"}, tenants)
}

func TestFilterAccessByScopesKeepsTenantsWithAllScopes(t *testing.T) {
	is := is.New(t)

	access := AccessMap{
		"tenant-a": map[Scope]struct{}{
			ReadSensors:   struct{}{},
			UpdateSensors: struct{}{},
		},
		"tenant-b": map[Scope]struct{}{
			ReadSensors: struct{}{},
		},
	}

	filtered := FilterAccessByScopes(WithAccess(context.Background(), access), ReadSensors, UpdateSensors)

	is.Equal(AccessMap{
		"tenant-a": map[Scope]struct{}{
			ReadSensors:   struct{}{},
			UpdateSensors: struct{}{},
		},
	}, filtered)
}
