package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/diwise/diwise-web/internal/pkg/application"
	"github.com/diwise/diwise-web/internal/pkg/presentation/api"
	"github.com/diwise/diwise-web/internal/pkg/presentation/api/authz"
	"github.com/diwise/diwise-web/internal/pkg/presentation/api/helpers"
	"github.com/diwise/service-chassis/pkg/infrastructure/buildinfo"
	"github.com/diwise/service-chassis/pkg/infrastructure/env"
	"github.com/diwise/service-chassis/pkg/infrastructure/net/http/authn"
	k8shandlers "github.com/diwise/service-chassis/pkg/infrastructure/net/http/handlers"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
	"github.com/diwise/service-chassis/pkg/infrastructure/servicerunner"

	"github.com/google/uuid"
)

const serviceName string = "diwise-web"

func DefaultFlags() FlagMap {
	return FlagMap{
		listenAddress: "",     // listen on all ipv4 and ipv6 interfaces
		servicePort:   "8080", //
		controlPort:   "",     // control port disabled by default

		devModeEnabled: "false",
	}
}

func main() {
	ctx, flags := parseExternalConfig(context.Background(), DefaultFlags())

	serviceVersion := buildinfo.SourceVersion()
	if serviceVersion == "" || flags[devModeEnabled] == "true" {
		serviceVersion = "develop" + "-" + uuid.NewString()
	}

	// Initialise the observability package
	ctx, logger, cleanup := o11y.Init(ctx, serviceName, serviceVersion, "text")
	defer cleanup()

	ctx = helpers.WithVersion(ctx, serviceVersion)

	cfg, err := newConfig(ctx, flags)
	exitIf(err, logger, "failed to create application config")

	runner, err := initialize(ctx, flags, cfg)
	exitIf(err, logger, "failed to initialize service")

	err = runner.Run(ctx)
	exitIf(err, logger, "service runner failed")
}

func newConfig(_ context.Context, _ FlagMap) (*AppConfig, error) {
	cfg := &AppConfig{}
	return cfg, nil
}

func initialize(ctx context.Context, flags FlagMap, cfg *AppConfig) (servicerunner.Runner[AppConfig], error) {

	devModeEnabled := flags[devModeEnabled] == "true"

	probes := map[string]k8shandlers.ServiceProber{
		"admin":        func(context.Context) (string, error) { return "ok", nil },
		"alarms":       func(context.Context) (string, error) { return "ok", nil },
		"devices":      func(context.Context) (string, error) { return "ok", nil },
		"iam":          func(context.Context) (string, error) { return "ok", nil },
		"measurements": func(context.Context) (string, error) { return "ok", nil },
		"things":       func(context.Context) (string, error) { return "ok", nil },
	}

	_, runner := servicerunner.New(ctx, *cfg,
		ifnot(flags[controlPort] == "",
			webserver("control", listen(flags[listenAddress]), port(flags[controlPort]),
				pprof(), liveness(func() error { return nil }), readiness(probes),
			)),
		webserver("public", listen(flags[listenAddress]), port(flags[servicePort]),
			muxinit(func(ctx context.Context, identifier string, port string, svcCfg *AppConfig, handler *http.ServeMux) (err error) {
				if port != flags[servicePort] {
					flags = changeURLPortNumbers(ctx, flags, flags[servicePort], port)
				}

				if !devModeEnabled {
					cfg.pte, err = authn.NewPhantomTokenExchange(
						authn.WithAppRoot(flags[appRoot]),
						authn.WithClientCredentials(flags[oauth2ClientID], flags[oauth2ClientSecret]),
						authn.WithLogger(logging.GetFromContext(ctx)),
					)
					if err != nil {
						return fmt.Errorf("failed to create phantom token exchange: %s", err.Error())
					}
				}

				svcCfg.app, err = application.New(ctx,
					flags[devMgmtURL], flags[thingsURL], flags[adminURL], flags[alarmsURL], flags[measurementsURL],
				)
				if err != nil {
					return err
				}

				mux := http.NewServeMux()
				middleware := append(
					make([]func(http.Handler) http.Handler, 0, 5),
					api.VersionReloader(helpers.GetVersion(ctx)),
					api.Logger(ctx),
				)

				if devModeEnabled {
					mux = api.InstallDevmodeHandlers(ctx, mux)
					middleware = append(middleware, api.NoLogin, api.NoCache)
				} else {
					svcCfg.pte.InstallHandlers(mux)
					middleware = append(middleware, svcCfg.pte.Middleware)
				}

				middleware = append(middleware, authz.Middleware)

				err = api.RegisterHandlers(ctx, mux, middleware, svcCfg.app, flags[webAssetPath])
				if err != nil {
					return fmt.Errorf("failed to create new api handler: %s", err.Error())
				}

				handler.Handle("GET /", mux)
				handler.Handle("POST /", mux)

				return nil
			}),
		),
		onstarting(func(ctx context.Context, svcCfg *AppConfig) (err error) {
			if svcCfg.pte != nil {
				err = svcCfg.pte.Connect(ctx, flags[oauth2RealmURL])
				if err != nil {
					return fmt.Errorf("failed to connect to iam: %s", err.Error())
				}
			}

			return nil
		}),
		onrunning(func(ctx context.Context, svcCfg *AppConfig) error {
			logging.GetFromContext(ctx).Info("diwise-web is running and waiting for connections", "approot", flags[appRoot])
			return nil
		}),
		onshutdown(func(ctx context.Context, svcCfg *AppConfig) error {
			if svcCfg.pte != nil {
				svcCfg.pte.Shutdown()
			}

			return nil
		}),
	)

	return runner, nil
}

func changeURLPortNumbers(_ context.Context, flags FlagMap, from, to string) FlagMap {
	for _, flag := range []FlagType{appRoot, adminURL, alarmsURL, devMgmtURL, measurementsURL, thingsURL} {
		flags[flag] = strings.Replace(flags[flag], ":"+from, ":"+to, 1)
	}
	return flags
}

func parseExternalConfig(ctx context.Context, flags FlagMap) (context.Context, FlagMap) {

	// Allow environment variables to override certain defaults
	envOrDef := env.GetVariableOrDefault
	flags[servicePort] = envOrDef(ctx, "SERVICE_PORT", flags[servicePort])
	flags[controlPort] = envOrDef(ctx, "CONTROL_PORT", flags[controlPort])
	flags[webAssetPath] = envOrDef(ctx, "DIWISEWEB_ASSET_PATH", "/opt/diwise/assets")

	defaultAppRoot := fmt.Sprintf("http://localhost:%s", flags[servicePort])
	flags[appRoot] = envOrDef(ctx, "APP_ROOT", defaultAppRoot)

	if flags[appRoot] == defaultAppRoot {
		logging.GetFromContext(ctx).Warn("environment variable APP_ROOT not set, using default (" + defaultAppRoot + ")")
	}

	apply := func(f FlagType) func(string) error {
		return func(value string) error {
			flags[f] = value
			return nil
		}
	}

	// Allow command line arguments to override defaults and environment variables
	flag.BoolFunc("devmode", "enable devmode with fake backend data", apply(devModeEnabled))
	flag.Func("web-assets", "path to web assets folder", apply(webAssetPath))
	flag.Parse()

	if flags[devModeEnabled] != "true" {
		flags[oauth2RealmURL] = env.GetVariableOrDie(ctx, "OAUTH2_REALM_URL", "a valid oauth2 realm URL")
		flags[oauth2ClientID] = env.GetVariableOrDie(ctx, "OAUTH2_CLIENT_ID", "a valid oauth2 client id")
		flags[oauth2ClientSecret] = env.GetVariableOrDie(ctx, "OAUTH2_CLIENT_SECRET", "a valid oauth2 client secret")

		flags[devMgmtURL] = env.GetVariableOrDie(ctx, "DEV_MGMT_URL", "a valid device management URL")
		flags[thingsURL] = env.GetVariableOrDie(ctx, "THINGS_URL", "a valid things URL")
		flags[measurementsURL] = env.GetVariableOrDie(ctx, "MEASUREMENTS_URL", "a valid measurements URL")
	} else {
		appRoot := flags[appRoot]
		flags[devMgmtURL] = appRoot + api.DevModePrefix + "/devices"
		flags[thingsURL] = appRoot + api.DevModePrefix + "/things"
		flags[measurementsURL] = appRoot + api.DevModePrefix + "/measurements"
	}

	flags[adminURL] = strings.Replace(flags[devMgmtURL], "devices", "admin", 1)
	flags[alarmsURL] = strings.Replace(flags[devMgmtURL], "devices", "alarms", 1)

	return ctx, flags
}

func exitIf(err error, logger *slog.Logger, msg string, args ...any) {
	if err != nil {
		logger.With(args...).Error(msg, "err", err.Error())
		os.Exit(1)
	}
}
