package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/diwise/diwise-web/internal/pkg/application"
	"github.com/diwise/diwise-web/internal/pkg/presentation/api"
	"github.com/diwise/diwise-web/internal/pkg/presentation/api/helpers"
	"github.com/diwise/service-chassis/pkg/infrastructure/buildinfo"
	"github.com/diwise/service-chassis/pkg/infrastructure/env"
	"github.com/diwise/service-chassis/pkg/infrastructure/net/http/authn"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
	"github.com/google/uuid"
)

const serviceName string = "diwise-web"

var webAssetPath string

func main() {

	serviceVersion := buildinfo.SourceVersion()
	if serviceVersion == "" {
		serviceVersion = "develop" + "-" + uuid.NewString()
	}

	// Initialise the observability package
	ctx, logger, cleanup := o11y.Init(context.Background(), serviceName, serviceVersion)
	defer cleanup()

	ctx = helpers.WithVersion(ctx, serviceVersion)

	// Get path to web assets folder via environment variable (or a default) ...
	webAssetPath = env.GetVariableOrDefault(ctx, "DIWISEWEB_ASSET_PATH", "/opt/diwise/assets")

	// ... but allow an override using a command line argument
	flag.StringVar(&webAssetPath, "web-assets", webAssetPath, "path to web assets folder")
	flag.Parse()

	mux := http.NewServeMux()

	realmURL := env.GetVariableOrDie(ctx, "OAUTH2_REALM_URL", "a valid oauth2 realm URL")
	clientID := env.GetVariableOrDie(ctx, "OAUTH2_CLIENT_ID", "a valid oauth2 client id")
	clientSecret := env.GetVariableOrDie(ctx, "OAUTH2_CLIENT_SECRET", "a valid oauth2 client secret")

	apiPort := env.GetVariableOrDefault(ctx, "SERVICE_PORT", "8094")
	defaultAppRoot := fmt.Sprintf("http://localhost:%s", apiPort)

	appRoot := env.GetVariableOrDefault(ctx, "APP_ROOT", defaultAppRoot)
	if appRoot == defaultAppRoot {
		logger.Warn("environment variable APP_ROOT not set, using default (" + defaultAppRoot + ")")
	}

	pte, err := authn.NewPhantomTokenExchange(
		authn.WithAppRoot(appRoot),
		authn.WithClientCredentials(clientID, clientSecret),
		authn.WithLogger(logger),
	)
	if err != nil {
		fatal(ctx, "failed to create phantom token exchange", err)
	}
	defer pte.Shutdown()

	err = pte.Connect(ctx, realmURL)
	if err != nil {
		fatal(ctx, "failed to connect to iam", err)
	}

	pte.InstallHandlers(mux)

	webapi, _, err := initialize(ctx, mux, pte, webAssetPath)
	if err != nil {
		fatal(ctx, "failed to initialize service", err)
	}

	webServer := &http.Server{Addr: ":" + apiPort, Handler: webapi.Router()}

	logger.Info("starting to listen for incoming connections", "port", apiPort)
	err = webServer.ListenAndServe()

	if err != nil {
		fatal(ctx, "failed to start request router", err)
	}
}

func initialize(ctx context.Context, mux *http.ServeMux, pte authn.PhantomTokenExchange, assetPath string) (api_ api.Api, app *application.App, err error) {
	app, err = application.New(ctx)
	if err != nil {
		return
	}

	api_, err = api.New(ctx, mux, pte, app, assetPath)
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
