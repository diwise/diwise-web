package client

import (
	"context"
	"sync/atomic"

	"github.com/diwise/diwise-web/internal/presentation/api/auth"
)

type accessDeniedTrackerKey struct{}

type accessDeniedTracker struct {
	denied atomic.Bool
}

func WithAccessDeniedTracker(ctx context.Context) context.Context {
	return context.WithValue(ctx, accessDeniedTrackerKey{}, &accessDeniedTracker{})
}

func MarkAccessDenied(ctx context.Context) {
	if auth.Token(ctx) == "" {
		return
	}

	tracker, ok := ctx.Value(accessDeniedTrackerKey{}).(*accessDeniedTracker)
	if !ok {
		return
	}

	tracker.denied.Store(true)
}

func AccessDenied(ctx context.Context) bool {
	tracker, ok := ctx.Value(accessDeniedTrackerKey{}).(*accessDeniedTracker)
	if !ok {
		return false
	}

	return tracker.denied.Load()
}
