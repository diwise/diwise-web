package client

import (
	"context"
	"sync/atomic"

	"github.com/diwise/diwise-web/internal/presentation/api/auth"
)

type accessDeniedTrackerKey struct{}

type AccessDeniedType int32

const (
	AccessDeniedNone       AccessDeniedType = 0
	AccessDeniedAuth       AccessDeniedType = 401
	AccessDeniedPermission AccessDeniedType = 403
)

type accessDeniedTracker struct {
	denied atomic.Int32
}

func WithAccessDeniedTracker(ctx context.Context) context.Context {
	return context.WithValue(ctx, accessDeniedTrackerKey{}, &accessDeniedTracker{})
}

func markAccessDenied(ctx context.Context, deniedType AccessDeniedType) {
	if auth.Token(ctx) == "" {
		return
	}

	tracker, ok := ctx.Value(accessDeniedTrackerKey{}).(*accessDeniedTracker)
	if !ok {
		return
	}

	tracker.denied.Store(int32(deniedType))
}

func MarkAuthDenied(ctx context.Context) {
	markAccessDenied(ctx, AccessDeniedAuth)
}

func MarkPermissionDenied(ctx context.Context) {
	markAccessDenied(ctx, AccessDeniedPermission)
}

func AccessDenied(ctx context.Context) AccessDeniedType {
	tracker, ok := ctx.Value(accessDeniedTrackerKey{}).(*accessDeniedTracker)
	if !ok {
		return AccessDeniedNone
	}

	return AccessDeniedType(tracker.denied.Load())
}
