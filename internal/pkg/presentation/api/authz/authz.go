package authz

import (
	"context"
	"net/http"

	base "github.com/diwise/diwise-web/internal/presentation/api/authz"
)

func NewContextFromAuthorizationHeader(ctx context.Context, r *http.Request) (context.Context, error) {
	return base.NewContextFromAuthorizationHeader(ctx, r)
}

func Middleware(next http.Handler) http.Handler {
	return base.Middleware(next)
}

func IsLoggedIn(ctx context.Context) bool {
	return base.IsLoggedIn(ctx)
}

func Token(ctx context.Context) string {
	return base.Token(ctx)
}

