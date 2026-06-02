package logging

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
)

type loggerContextKey string

var loggerCtxKey loggerContextKey = "logger"
var logLevel = new(slog.LevelVar)

type CleanupFunc func()

func Init(ctx context.Context, serviceName, serviceVersion string) (context.Context, CleanupFunc) {

	cleanup := func() {}

	exporterEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if exporterEndpoint != "" {

		res, err := newResource(serviceName, serviceVersion)
		if err != nil {
			panic(err)
		}

		loggerProvider, err := newLoggerProvider(ctx, res)
		if err != nil {
			panic(err)
		}

		cleanup = func() { loggerProvider.Shutdown(ctx) }

		// Register as global logger provider so that it can be accessed via global.LoggerProvider.
		// Most log bridges use the global logger provider as the default.
		// If the global logger provider is not set then a no-op implementation
		// is used, which fails to generate data.
		global.SetLoggerProvider(loggerProvider)
	}

	return ctx, cleanup
}

func NewLogger(ctx context.Context, serviceName, serviceVersion, logFormat string) (context.Context, *slog.Logger) {
	logLevel.Set(slog.LevelDebug)
	opts := &slog.HandlerOptions{Level: logLevel}

	var handler slog.Handler

	switch logFormat {
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, opts)
	case "text":
		handler = slog.NewTextHandler(os.Stdout, opts)
	default:
		panic("log format " + logFormat + " not supported")
	}

	handler = handler.WithAttrs([]slog.Attr{
		slog.String("service", strings.ToLower(serviceName)),
		slog.String("version", serviceVersion),
	})

	var logger *slog.Logger

	exporterEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")

	if exporterEndpoint != "" {
		otelLogHandler := otelslog.NewHandler(serviceName)
		logger = slog.New(slog.NewMultiHandler(handler, otelLogHandler))
	} else {
		logger = slog.New(handler)
	}

	return NewContextWithLogger(ctx, logger), logger
}

func NewContextWithLogger(ctx context.Context, logger *slog.Logger, args ...any) context.Context {
	logger = logger.With(args...)
	return context.WithValue(ctx, loggerCtxKey, logger)
}

func GetFromContext(ctx context.Context) *slog.Logger {
	logger, ok := ctx.Value(loggerCtxKey).(*slog.Logger)

	if !ok {
		return slog.Default()
	}

	return logger
}

func LogLevel() slog.Level {
	return logLevel.Level()
}

func SetLogLevel(level slog.Level) {
	logLevel.Set(level)
}

func newResource(serviceName, serviceVersion string) (*resource.Resource, error) {
	return resource.Merge(resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion),
		))
}

func newLoggerProvider(ctx context.Context, res *resource.Resource) (*log.LoggerProvider, error) {
	exporter, err := otlploghttp.New(ctx)
	if err != nil {
		return nil, err
	}

	processor := log.NewBatchProcessor(
		exporter,
		log.WithExportInterval(1*time.Second),
	)

	provider := log.NewLoggerProvider(
		log.WithResource(res),
		log.WithProcessor(processor),
	)
	return provider, nil
}
