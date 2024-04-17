package main

import (
	"context"
	"flag"
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

var webAssetPath string

func main() {

	serviceVersion := buildinfo.SourceVersion()
	if serviceVersion == "" {
		serviceVersion = "develop"
	}

	// Initialise the observability package
	ctx, logger, cleanup := o11y.Init(context.Background(), serviceName, serviceVersion)
	defer cleanup()

	// Get path to web assets folder via environment variable (or a default) ...
	webAssetPath = env.GetVariableOrDefault(ctx, "DIWISEWEB_ASSET_PATH", "/opt/diwise/assets")

	// ... but allow an override using a command line argument
	flag.StringVar(&webAssetPath, "web-assets", webAssetPath, "path to web assets folder")
	flag.Parse()

	mux := http.NewServeMux()

	webapi, _, err := initialize(ctx, serviceVersion, mux, webAssetPath)
	if err != nil {
		fatal(ctx, "failed to initialize service", err)
	}

	apiPort := env.GetVariableOrDefault(ctx, "SERVICE_PORT", "8080")

	webServer := &http.Server{Addr: ":" + apiPort, Handler: webapi.Router()}

	logger.Info("starting to listen for incoming connections", "port", apiPort)
	err = webServer.ListenAndServe()

	if err != nil {
		fatal(ctx, "failed to start request router", err)
	}
}

func initialize(ctx context.Context, version string, mux *http.ServeMux, assetPath string) (api_ api.Api, app application.WebApp, err error) {
	app, err = application.New(ctx)
	if err != nil {
		return
	}

	api_, err = api.New(ctx, mux, app, version, assetPath)
	if err != nil {
		return
	}

	return api_, app, nil
}

func fatal(ctx context.Context, msg string, err error) {
	logger := logging.GetFromContext(ctx)
	logger.Error(msg, "err", err.Error())
	os.Exit(1)
}
