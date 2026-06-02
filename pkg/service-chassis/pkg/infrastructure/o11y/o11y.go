package o11y

import (
	"context"
	"log/slog"

	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/metrics"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/tracing"
	"go.opentelemetry.io/otel/trace"
)

type CleanupFunc func()

func Init(ctx context.Context, serviceName, serviceVersion, logfmt string) (context.Context, *slog.Logger, CleanupFunc) {
	ctx, cleanupLogging := logging.Init(ctx, serviceName, serviceVersion)

	ctx, logger := logging.NewLogger(ctx, serviceName, serviceVersion, logfmt)
	logger.Info("starting up ...")

	cleanupMetrics, err := metrics.Init(ctx, logger, serviceName, serviceVersion)
	if err != nil {
		msg := "failed to init metrics"
		logger.Error(msg, "err", err.Error())
		panic(msg)
	}

	cleanupTracing, err := tracing.Init(ctx, logger, serviceName, serviceVersion)
	if err != nil {
		msg := "failed to init tracing"
		logger.Error(msg, "err", err.Error())
		panic(msg)
	}

	cleanup := func() {
		cleanupLogging()
		cleanupMetrics()
		cleanupTracing()
	}

	return ctx, logger, cleanup
}

func AddTraceIDToLoggerAndStoreInContext(span trace.Span, logger *slog.Logger, ctx context.Context) (string, context.Context, *slog.Logger) {
	log := logger
	traceID, ok := tracing.ExtractTraceID(span)

	if ok {
		log = log.With(slog.String("traceID", traceID))
	}

	ctx = logging.NewContextWithLogger(ctx, log)
	return traceID, ctx, log
}
