package main

import (
	"context"
	"net/http"
	"os"

	"github.com/diwise/diwise-web/internal/pkg/application"
	"github.com/diwise/diwise-web/internal/pkg/presentation/api"
	"github.com/diwise/service-chassis/pkg/infrastructure/buildinfo"
	"github.com/diwise/service-chassis/pkg/infrastructure/env"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
)

const serviceName string = "diwise-web"

func main() {
	serviceVersion := buildinfo.SourceVersion()
	if serviceVersion == "" {
		serviceVersion = "develop"
	}

	ctx, logger, cleanup := o11y.Init(context.Background(), serviceName, serviceVersion)
	defer cleanup()

	assetPath := env.GetVariableOrDefault(ctx, "DIWISEWEB_ASSET_PATH", "/opt/diwise/assets")

	webapi, _, err := initialize(ctx, serviceVersion, assetPath)
	if err != nil {
		fatal(ctx, "failed to initialize service", err)
	}

	apiPort := env.GetVariableOrDefault(ctx, "SERVICE_PORT", "8080")
	logger.Info("starting to listen for incoming connections", "port", apiPort)
	err = http.ListenAndServe(":"+apiPort, webapi.Router())

	if err != nil {
		fatal(ctx, "failed to start request router", err)
	}
}

func initialize(ctx context.Context, version, assetPath string) (api.Api, application.WebApp, error) {
	app, err := application.New(ctx)
	if err != nil {
		return nil, nil, err
	}

	api_, err := api.New(ctx, app, version, assetPath)
	if err != nil {
		return nil, nil, err
	}

	return api_, app, nil
}

func fatal(ctx context.Context, msg string, err error) {
	logger := logging.GetFromContext(ctx)
	logger.Error(msg, "err", err.Error())
	os.Exit(1)
}
