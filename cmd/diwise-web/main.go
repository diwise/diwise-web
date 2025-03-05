package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/diwise/diwise-web/internal/pkg/application"
	"github.com/diwise/diwise-web/internal/pkg/presentation/api"
	"github.com/diwise/diwise-web/internal/pkg/presentation/api/authz"
	"github.com/diwise/diwise-web/internal/pkg/presentation/api/helpers"
	"github.com/diwise/frontend-toolkit/pkg/middleware"
	"github.com/diwise/frontend-toolkit/pkg/middleware/csp"
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

		devModeEnabled:        "false",
		contentSecurityPolicy: "strict",
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

	ctx, cfg.cancelContext = context.WithCancel(ctx)

	runner, err := initialize(ctx, flags, cfg)
	exitIf(err, logger, "failed to initialize service")

	err = runner.Run(ctx)
	exitIf(err, logger, "service runner failed")

	logger.Info("shutting down")
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
					opts := append(
						make([]authn.PhantomTokenOption, 0, 5),
						authn.WithAppRoot(flags[appRoot]),
						authn.WithClientCredentials(flags[oauth2ClientID], flags[oauth2ClientSecret]),
						authn.WithLogger(logging.GetFromContext(ctx)))

					if flags[oauth2SkipVerify] == "true" {
						opts = append(opts, authn.WithInsecureSkipVerify())
					}

					svcCfg.pte, err = authn.NewPhantomTokenExchange(opts...)
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
				middlewares := append(
					make([]func(http.Handler) http.Handler, 0, 10),
					api.VersionReloader(helpers.GetVersion(ctx)),
					api.Logger(ctx),
				)

				if flags[contentSecurityPolicy] != "off" {
					if flags[contentSecurityPolicy] != "report" {
						// default fallback is a strict csp unless one of the other modes are explicitly set
						middlewares = append(middlewares,
							csp.NewContentSecurityPolicy(csp.StrictDynamic()),
						)
					} else {
						middlewares = append(middlewares,
							csp.NewContentSecurityPolicy(csp.ReportOnly(), csp.StrictDynamic()),
						)
					}
				}

				if devModeEnabled {
					mux = api.InstallDevmodeHandlers(ctx, mux)
					middlewares = append(middlewares, api.NoLogin, api.NoCache)
				} else {
					svcCfg.pte.InstallHandlers(mux)
					middlewares = append(middlewares,
						svcCfg.pte.Middleware,
						middleware.StrictTransportSecurity(24*time.Hour),
					)
				}

				middlewares = append(middlewares, authz.Middleware)

				err = api.RegisterHandlers(ctx, mux, middlewares, svcCfg.app, flags[webAssetPath])
				if err != nil {
					return fmt.Errorf("failed to create new api handler: %s", err.Error())
				}

				handler.Handle("GET /", mux)
				handler.Handle("POST /", mux)
				handler.Handle("DELETE /", mux)

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
			if svcCfg.cancelContext != nil {
				svcCfg.cancelContext()
				svcCfg.cancelContext = nil
			}

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
	flags[contentSecurityPolicy] = envOrDef(ctx, "CONTENT_SECURITY_POLICY", flags[contentSecurityPolicy])

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
	flag.Func("csp", "set content security policy to strict, report or off", apply(contentSecurityPolicy))
	flag.Func("web-assets", "path to web assets folder", apply(webAssetPath))
	flag.Parse()

	if flags[devModeEnabled] != "true" {
		flags[oauth2RealmURL] = env.GetVariableOrDie(ctx, "OAUTH2_REALM_URL", "oauth2 realm URL")
		flags[oauth2ClientID] = env.GetVariableOrDie(ctx, "OAUTH2_CLIENT_ID", "oauth2 client id")
		flags[oauth2ClientSecret] = env.GetVariableOrDie(ctx, "OAUTH2_CLIENT_SECRET", "oauth2 client secret")
		flags[oauth2SkipVerify] = env.GetVariableOrDefault(ctx, "OAUTH2_REALM_INSECURE", "false")

		flags[devMgmtURL] = env.GetVariableOrDie(ctx, "DEV_MGMT_URL", "device management URL")
		flags[thingsURL] = env.GetVariableOrDie(ctx, "THINGS_URL", "things URL")
		flags[measurementsURL] = env.GetVariableOrDie(ctx, "MEASUREMENTS_URL", "measurements URL")
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
