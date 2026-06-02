package requestlogger

import (
	"log/slog"
	"net/http"

	"go.opentelemetry.io/otel/trace"

	"github.com/diwise/service-chassis/pkg/infrastructure/o11y"
)

func New(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			_, ctx, _ = o11y.AddTraceIDToLoggerAndStoreInContext(
				trace.SpanFromContext(ctx),
				logger,
				ctx)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
