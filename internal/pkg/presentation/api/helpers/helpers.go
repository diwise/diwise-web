package helpers

import (
	"context"
	"net/http"
)

func Decorate(ctx context.Context, kv ...any) context.Context {

	for kvidx := range len(kv) / 2 {
		ctx = context.WithValue(ctx, kv[kvidx*2], kv[(kvidx*2)+1])
	}

	return ctx
}

func IsHxRequest(r *http.Request) bool {
	isHxRequest := r.Header.Get("HX-Request")
	return isHxRequest == "true"
}

func UrlParamOrDefault(r *http.Request, param, defaultValue string) string {
	value := r.URL.Query().Get(param)
	if value == "" {
		return defaultValue
	}
	return value
}
