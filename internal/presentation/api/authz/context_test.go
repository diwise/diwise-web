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

	got, ok := AccessFromContext(withAccess(context.Background(), access))

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

	err := RequireTenantAccess(access, "tenant-b", UpdateSensors)

	is.True(errors.Is(err, ErrAccessDenied))
	is.Equal(AccessDeniedError{Tenant: "tenant-b", Scope: UpdateSensors}, err)
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

	filtered := FilterAccessByScopes(access, ReadSensors, UpdateSensors)

	is.Equal(AccessMap{
		"tenant-a": map[Scope]struct{}{
			ReadSensors:   struct{}{},
			UpdateSensors: struct{}{},
		},
	}, filtered)
}
