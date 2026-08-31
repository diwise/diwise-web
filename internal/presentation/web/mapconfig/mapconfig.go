package mapconfig

import (
	"context"
	"net/http"
)

type apiKeyContextKey struct{}

// Middleware adds the basemap API key to each request context so map
// components can use it without passing presentation configuration through
// every handler and view model.
func Middleware(apiKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), apiKeyContextKey{}, apiKey)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func APIKey(ctx context.Context) string {
	apiKey, _ := ctx.Value(apiKeyContextKey{}).(string)
	return apiKey
}
