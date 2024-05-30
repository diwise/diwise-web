package helpers

import (
	"context"
	"net/http"
	"strconv"
)

func Decorate(ctx context.Context, kv ...any) context.Context {
	for kvidx := range len(kv) / 2 {
		ctx = context.WithValue(ctx, kv[kvidx*2], kv[(kvidx*2)+1])
	}
	return ctx
}

func FromCtxInt(ctx context.Context, key any, defaultValue int) int {
	v := ctx.Value(key)
	if v == nil {
		return defaultValue
	}
	if i, ok := v.(int); ok {
		return i
	}
	if s, ok := v.(string); ok {
		i, _ := strconv.Atoi(s)
		return i
	}
	return defaultValue
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

func GetOffsetAndLimit(r *http.Request) (offset, limit int) {
	pageIndex := UrlParamOrDefault(r, "page", "1")
	pageSize := UrlParamOrDefault(r, "limit", "15")

	limit, _ = strconv.Atoi(pageSize)
	index, _ := strconv.Atoi(pageIndex)

	if index == 1 {
		offset = 0
		return
	}

	offset = index * limit
	return
}
