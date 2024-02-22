package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/a-h/templ"
	"github.com/diwise/diwise-web/internal/pkg/application"
	"github.com/diwise/diwise-web/internal/pkg/presentation/api/handlers/components/sensors"
	"github.com/diwise/diwise-web/internal/pkg/presentation/api/helpers"
	"github.com/diwise/diwise-web/internal/pkg/presentation/locale"
	"github.com/diwise/diwise-web/internal/pkg/presentation/web/assets"
	"github.com/diwise/diwise-web/internal/pkg/presentation/web/components"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
	"github.com/google/uuid"
)

type Api interface {
	Router() *http.ServeMux
}

type impl struct {
	webapp application.WebApp
	router *http.ServeMux
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

func New(ctx context.Context, mux *http.ServeMux, app application.WebApp, version, assetPath string) (Api, error) {

	if version == "develop" {
		version = version + "-" + uuid.NewString()
	}

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

	r.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {

		acceptLanguage := r.Header.Get("Accept-Language")

		w.Header().Add("Content-Type", "text/html")
		w.Header().Add("Cache-Control", "no-cache")
		w.Header().Add("Strict-Transport-Security", "max-age=86400; includeSubDomains")
		w.WriteHeader(http.StatusOK)

		ctx := context.WithValue(r.Context(), components.CurrentComponent, "home")

		localizer := l10n.For(acceptLanguage)

		component := components.StartPage(
			version, localizer,
			assetLoader.Load, components.Home(localizer, assetLoader.Load),
		)
		component.Render(ctx, w)
	})

	r.HandleFunc("GET /{component}", func() http.HandlerFunc {

		comps := map[string]func(locale.Localizer, assets.AssetLoaderFunc) templ.Component{
			"home":    components.Home,
			"sensors": components.Sensors,
		}

		return func(w http.ResponseWriter, r *http.Request) {

			componentName := r.PathValue("component")
			template, ok := comps[componentName]
			if !ok {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}

			w.Header().Add("Content-Type", "text/html")
			w.Header().Add("Cache-Control", "no-cache")
			w.Header().Add("Strict-Transport-Security", "max-age=86400; includeSubDomains")
			w.WriteHeader(http.StatusOK)

			ctx := context.WithValue(r.Context(), components.CurrentComponent, componentName)

			localizer := l10n.For(r.Header.Get("Accept-Language"))

			component := components.StartPage(
				version, localizer,
				assetLoader.Load, template(localizer, assetLoader.Load),
			)
			component.Render(ctx, w)
		}
	}())

	r.HandleFunc("GET /components/tables/sensors", RequireHX(
		sensors.NewTableSensorsComponentHandler(l10n, assetLoader.Load, app),
	))

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

	mux.Handle("GET /", logger(ctx, r))

	return &impl{
		webapp: app,
		router: mux,
	}, nil
}

func (a *impl) Router() *http.ServeMux {
	return a.router
}
