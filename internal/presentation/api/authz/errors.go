package authz

import (
	"errors"
	"fmt"
)

var ErrAccessDenied = errors.New("access denied")

type AccessDeniedError struct {
	Tenant string
	Scope  Scope
}

func (e AccessDeniedError) Error() string {
	if e.Tenant == "" {
		return fmt.Sprintf("access denied for scope %q", e.Scope)
	}
	return fmt.Sprintf("access denied for tenant %q and scope %q", e.Tenant, e.Scope)
}

func (e AccessDeniedError) Is(target error) bool {
	return target == ErrAccessDenied
}
