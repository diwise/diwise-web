package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/diwise/diwise-web/internal/pkg/application"
	"github.com/diwise/diwise-web/internal/pkg/presentation/api/authz"
	"github.com/diwise/diwise-web/internal/pkg/presentation/api/handlers/components/admin"
	"github.com/diwise/diwise-web/internal/pkg/presentation/api/handlers/components/home"
	"github.com/diwise/diwise-web/internal/pkg/presentation/api/handlers/components/sensors"
	"github.com/diwise/diwise-web/internal/pkg/presentation/api/handlers/components/things"
	"github.com/diwise/diwise-web/internal/pkg/presentation/api/helpers"

	"github.com/diwise/frontend-toolkit/pkg/assets"
	"github.com/diwise/frontend-toolkit/pkg/locale"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
)

type writerMiddleware struct {
	rw      http.ResponseWriter
	nocache bool

	contentLength int
	statusCode    int
}

func (w *writerMiddleware) Header() http.Header {
	return w.rw.Header()
}

func (w *writerMiddleware) Write(data []byte) (int, error) {
	if w.statusCode == 0 {
		fmt.Println("write wo header!")
	}

	if w.nocache && w.contentLength == 0 {
		w.rw.Header()["Cache-Control"] = []string{"no-store"}
	}

	count, err := w.rw.Write(data)
	if err == nil {
		w.contentLength += count
	}
	return count, err
}

func (w *writerMiddleware) WriteHeader(statusCode int) {
	if w.statusCode != 0 {
		return
	}

	if w.nocache {
		w.rw.Header()["Cache-Control"] = []string{"no-store"}
	}

	w.statusCode = statusCode
	w.rw.WriteHeader(statusCode)
}

func Logger(ctx context.Context) func(http.Handler) http.Handler {
	log := logging.GetFromContext(ctx)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			wmw := &writerMiddleware{rw: w}
			start := time.Now()

			ctx := logging.NewContextWithLogger(r.Context(), log)
			r = r.WithContext(ctx)

			next.ServeHTTP(wmw, r)
			duration := time.Since(start)

			if wmw.statusCode < http.StatusBadRequest {
				log.Info("served http request", "method", r.Method, "path", r.URL.Path, "status", wmw.statusCode, "duration", duration.String())
			} else if wmw.statusCode < http.StatusInternalServerError {
				log.Warn("served http request", "method", r.Method, "path", r.URL.Path, "status", wmw.statusCode, "duration", duration.String())
			} else {
				log.Error("served http request", "method", r.Method, "path", r.URL.Path, "status", wmw.statusCode, "duration", duration.String())
			}
		})
	}
}

func RequireHX(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		isHxRequest := r.Header.Get("HX-Request")
		if isHxRequest != "true" {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		next.ServeHTTP(w, r)
	}
}

func VersionReloader(version string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if helpers.IsHxRequest(r) && strings.HasPrefix(r.URL.Path, "/version") {
				if strings.Compare(r.URL.Path, "/version/"+version) != 0 {
					currentURL := r.Header.Get("HX-Current-URL")
					if currentURL == "" {
						currentURL = "/"
					}
					w.Header().Set("HX-Redirect", currentURL)
				}

				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func RegisterHandlers(ctx context.Context, mux *http.ServeMux, middleware []func(http.Handler) http.Handler, app *application.App, assetPath string) error {

	r := http.NewServeMux()

	assetLoader, _ := assets.NewLoader(ctx, assets.BasePath(assetPath), assets.Logger(logging.GetFromContext(ctx)))

	l10n := locale.NewLocalizer(assetPath, "sv", "en")
	// home
	r.HandleFunc("GET /", func() http.HandlerFunc {
		// GET / catches ALL routes that no other handler matches, so we need to make sure that
		// we only serve the homepage when the path actually IS / (or /home as handled below).
		next := home.NewHomePage(ctx, l10n, assetLoader.Load, app)
		return func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/" {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			next(w, r)
		}
	}())
	r.HandleFunc("GET /home", home.NewHomePage(ctx, l10n, assetLoader.Load, app))
	r.HandleFunc("GET /components/home/statistics", RequireHX(home.NewOverviewCardsHandler(ctx, l10n, assetLoader.Load, app)))
	r.HandleFunc("GET /components/home/usage", RequireHX(home.NewUsageHandler(ctx, l10n, assetLoader.Load, app)))
	r.HandleFunc("GET /components/tables/alarms", RequireHX(home.NewAlarmsTable(ctx, l10n, assetLoader.Load, app)))

	// things
	r.HandleFunc("GET /things", things.NewThingsPage(ctx, l10n, assetLoader.Load, app))
	r.HandleFunc("POST /things", things.NewCreateThingComponentHandler(ctx, l10n, assetLoader.Load, app))
	r.HandleFunc("GET /things/{id}", things.NewThingDetailsPage(ctx, l10n, assetLoader.Load, app))
	r.HandleFunc("DELETE /things/{id}", things.DeleteThingComponentHandler(ctx, l10n, assetLoader.Load, app))

	//things - components
	r.HandleFunc("GET /components/things", RequireHX(things.NewThingComponentHandler(ctx, l10n, assetLoader.Load, app)))
	r.HandleFunc("GET /components/things/{id}", RequireHX(things.NewThingDetailsComponentHandler(ctx, l10n, assetLoader.Load, app)))
	r.HandleFunc("POST /components/things/{id}", RequireHX(things.NewThingDetailsComponentHandler(ctx, l10n, assetLoader.Load, app)))
	r.HandleFunc("DELETE /components/things/{id}", RequireHX(things.NewThingDetailsComponentHandler(ctx, l10n, assetLoader.Load, app)))

	r.HandleFunc("GET /components/tables/things", RequireHX(things.NewThingsTable(ctx, l10n, assetLoader.Load, app)))
	r.HandleFunc("GET /components/things/list", RequireHX(things.NewThingsDataList(ctx, l10n, assetLoader.Load, app)))

	// sensors
	r.HandleFunc("GET /sensors", sensors.NewSensorsPage(ctx, l10n, assetLoader.Load, app))
	r.HandleFunc("GET /sensors/{id}", sensors.NewSensorDetailsPage(ctx, l10n, assetLoader.Load, app))
	r.HandleFunc("GET /components/sensors/details", RequireHX(sensors.NewSensorDetailsComponentHandler(ctx, l10n, assetLoader.Load, app)))
	r.HandleFunc("GET /components/sensors/{id}/batterylevel", RequireHX(sensors.NewBatteryLevelComponentHandler(ctx, l10n, assetLoader.Load, app)))
	r.HandleFunc("POST /components/sensors/details", sensors.NewSaveSensorDetailsComponentHandler(ctx, l10n, assetLoader.Load, app))
	r.HandleFunc("GET /components/tables/sensors", RequireHX(sensors.NewSensorsTable(ctx, l10n, assetLoader.Load, app)))
	r.HandleFunc("GET /components/sensors/list", RequireHX(sensors.NewSensorsDataList(ctx, l10n, assetLoader.Load, app)))
	//measurements
	r.HandleFunc("GET /components/measurements", RequireHX(sensors.NewMeasurementComponentHandler(ctx, l10n, assetLoader.Load, app)))
	r.HandleFunc("GET /components/things/measurements/{id}", RequireHX(things.NewMeasurementComponentHandler(ctx, l10n, assetLoader.Load, app)))
	// admin
	r.HandleFunc("GET /components/admin/types", RequireHX(admin.NewMeasurementTypesComponentHandler(ctx, l10n, assetLoader.Load, app)))
	r.HandleFunc("GET /admin/token", func(w http.ResponseWriter, r *http.Request) {
		log := logging.GetFromContext(r.Context())
		token := authz.Token(r.Context())
		log.Debug("current token", slog.String("token", token))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(token))
	})

	// Handle requests for leaflet images /assets/<leafletcss-sha>/images/<image>.png
	leafletSHA := assetLoader.Load("/css/leaflet.css").SHA256()

	assets.RegisterEndpoints(ctx, assetLoader, assets.WithMux(r),
		assets.WithRedirect("/favicon.ico", "/icons/favicon.ico", http.StatusFound),
		assets.WithRedirect(
			fmt.Sprintf("/assets/%s/images/{img}", leafletSHA), "/images/leaflet-{img}", http.StatusMovedPermanently,
		),
	)

	var handler http.Handler = r

	// wrap the mux with any passed in middleware handlers
	for _, mw := range slices.Backward(middleware) {
		handler = mw(handler)
	}

	mux.Handle("GET /", handler)
	mux.Handle("POST /", handler)
	mux.Handle("DELETE /", handler)

	return nil
}
