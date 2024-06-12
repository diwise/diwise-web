package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/diwise/diwise-web/internal/pkg/application"
	"github.com/diwise/diwise-web/internal/pkg/presentation/api/authz"
	"github.com/diwise/diwise-web/internal/pkg/presentation/api/handlers/components/admin"
	"github.com/diwise/diwise-web/internal/pkg/presentation/api/handlers/components/sensors"
	"github.com/diwise/diwise-web/internal/pkg/presentation/api/handlers/pages"
	"github.com/diwise/diwise-web/internal/pkg/presentation/api/helpers"
	"github.com/diwise/diwise-web/internal/pkg/presentation/locale"
	"github.com/diwise/diwise-web/internal/pkg/presentation/web/assets"
	"github.com/diwise/service-chassis/pkg/infrastructure/net/http/authn"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
)

type Api interface {
	Router() *http.ServeMux
}

type impl struct {
	webapp        *application.App
	router        *http.ServeMux
	tokenExchange authn.PhantomTokenExchange

	version string
}

type writerMiddleware struct {
	rw http.ResponseWriter

	contentLength int
	statusCode    int
}

func (w *writerMiddleware) Header() http.Header {
	return w.rw.Header()
}

func (w *writerMiddleware) Write(data []byte) (int, error) {
	count, err := w.rw.Write(data)
	if err == nil {
		w.contentLength += count
	}
	return count, err
}

func (w *writerMiddleware) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.rw.WriteHeader(statusCode)
}

func logger(ctx context.Context, next http.Handler) http.Handler {
	log := logging.GetFromContext(ctx)

	fn := func(w http.ResponseWriter, r *http.Request) {
		wmw := &writerMiddleware{rw: w}
		start := time.Now()
		next.ServeHTTP(wmw, r)
		duration := time.Since(start)

		if wmw.statusCode < http.StatusBadRequest {
			log.Info("served http request", "method", r.Method, "path", r.URL.Path, "status", wmw.statusCode, "duration", duration.String())
		} else if wmw.statusCode < http.StatusInternalServerError {
			log.Warn("served http request", "method", r.Method, "path", r.URL.Path, "status", wmw.statusCode, "duration", duration.String())
		} else {
			log.Error("served http request", "method", r.Method, "path", r.URL.Path, "status", wmw.statusCode, "duration", duration.String())
		}
	}

	return http.HandlerFunc(fn)
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

func New(ctx context.Context, mux *http.ServeMux, pte authn.PhantomTokenExchange, app *application.App, assetPath string) (Api, error) {
	version := helpers.GetVersion(ctx)

	mux.HandleFunc("GET /version/{v}", func(w http.ResponseWriter, r *http.Request) {
		if helpers.IsHxRequest(r) {
			if r.PathValue("v") != version {
				currentURL := r.Header.Get("HX-Current-URL")
				if currentURL == "" {
					currentURL = "/"
				}
				w.Header().Set("HX-Redirect", currentURL)
			}
		}

		w.WriteHeader(http.StatusNoContent)
	})

	r := http.NewServeMux()

	assetLoader, _ := assets.NewLoader(ctx, assets.BasePath(assetPath))

	l10n := locale.NewLocalizer(assetPath, "sv", "en")

	r.HandleFunc("GET /", pages.NewHomePage(ctx, l10n, assetLoader.Load))
	r.HandleFunc("GET /home", pages.NewHomePage(ctx, l10n, assetLoader.Load))
	r.HandleFunc("GET /objects", pages.NewThingsPage(ctx, l10n, assetLoader.Load))

	r.HandleFunc("GET /sensors", pages.NewSensorListPage(ctx, l10n, assetLoader.Load, app))
	r.HandleFunc("GET /sensors/{id}", pages.NewSensorDetailsPage(ctx, l10n, assetLoader.Load, app))

	r.HandleFunc("GET /components/sensors/details", RequireHX(sensors.NewSensorDetailsComponentHandler(ctx, l10n, assetLoader.Load, app)))
	r.HandleFunc("POST /components/sensors/details", sensors.NewSaveSensorDetailsComponentHandler(ctx, l10n, assetLoader.Load, app))
	r.HandleFunc("GET /components/tables/sensors", RequireHX(sensors.NewTableSensorsComponentHandler(ctx, l10n, assetLoader.Load, app)))
	r.HandleFunc("GET /components/admin/types", RequireHX(admin.NewMeasurementTypesComponentHandler(ctx, l10n, assetLoader.Load, app)))

	// Handle requests for leaflet images /assets/<leafletcss-sha>/images/<image>.png
	leafletSHA := assetLoader.Load("/css/leaflet.css").SHA256()
	r.HandleFunc(fmt.Sprintf("GET /assets/%s/images/{img}", leafletSHA), func(w http.ResponseWriter, r *http.Request) {
		image := r.PathValue("img")
		http.Redirect(w, r, assetLoader.Load("/images/leaflet-"+image).Path(), http.StatusMovedPermanently)
	})

	r.HandleFunc("GET /assets/{sha}/{filename}", func(w http.ResponseWriter, r *http.Request) {
		sha := r.PathValue("sha")

		a, err := assetLoader.LoadFromSha256(sha)
		if err != nil {
			if err == assets.ErrNotFound {
				w.WriteHeader(http.StatusNotFound)
			} else {
				w.WriteHeader(http.StatusInternalServerError)
			}

			return
		}

		w.Header().Set("Content-Type", a.ContentType())
		w.Header().Set("Content-Length", fmt.Sprintf("%d", a.ContentLength()))
		w.WriteHeader(http.StatusOK)
		w.Write(a.Body())
	})

	r.HandleFunc("GET /favicon.ico", func() http.HandlerFunc {
		faviconPath := assetLoader.Load("/icons/favicon.ico").Path()
		return func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, faviconPath, http.StatusFound)
		}
	}())

	mux.Handle(
		"GET /", logger(ctx, pte.Middleware()(authz.Middleware()(r))),
	)
	mux.Handle(
		"POST /", logger(ctx, pte.Middleware()(authz.Middleware()(r))),
	)

	return &impl{
		webapp:        app,
		router:        mux,
		tokenExchange: pte,
		version:       version,
	}, nil
}

func (a *impl) Router() *http.ServeMux {
	return a.router
}
