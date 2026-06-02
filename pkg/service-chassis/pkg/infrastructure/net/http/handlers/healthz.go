package handlers

import (
	"context"
	"net/http"

	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
)

func NewHealthHandler(ctx context.Context, probes map[string]ServiceProber) http.HandlerFunc {

	next := NewReadinessHandler(ctx, probes)

	return func(w http.ResponseWriter, r *http.Request) {
		logging.GetFromContext(ctx).Debug("deprecated /health endpoint was called instead of /readyz")
		next(w, r)
	}
}
