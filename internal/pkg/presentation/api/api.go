package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/a-h/templ"
	"github.com/diwise/diwise-web/internal/pkg/application"
	"github.com/diwise/diwise-web/internal/pkg/presentation/web/assets"
	"github.com/diwise/diwise-web/internal/pkg/presentation/web/components"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
)

type Api interface {
	Router() *chi.Mux
}

type impl struct {
	webapp application.WebApp
	router *chi.Mux
}

func isHxRequest(r *http.Request) bool {
	isHxRequest := r.Header.Get("HX-Request")
	return isHxRequest == "true"
}

func reloader(version string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			if isHxRequest(r) {
				if strings.HasPrefix(r.URL.Path, "/version/") {
					substrings := strings.Split(r.URL.Path, "/")
					v := substrings[len(substrings)-1]

					if v != version {
						currentURL := r.Header.Get("HX-Current-URL")
						if currentURL == "" {
							currentURL = "/"

						}
						w.Header().Set("HX-Redirect", currentURL)
						w.WriteHeader(http.StatusOK)

						return
					}
				}
			}

			next.ServeHTTP(w, r)
		}

		return http.HandlerFunc(fn)
	}
}

func New(ctx context.Context, app application.WebApp, version, assetPath string) (Api, error) {

	if version == "develop" {
		version = version + "-" + uuid.NewString()
	}

	router := chi.NewRouter()

	router.Use(reloader(version))
	router.Get("/version/{v}", func() http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}
	}())

	router.Group(func(r chi.Router) {
		r.Use(middleware.Logger)

		assetLoader, _ := assets.NewLoader(ctx, assets.BasePath(assetPath))

		r.Get("/", func() http.HandlerFunc {
			return func(w http.ResponseWriter, r *http.Request) {

				w.Header().Add("Content-Type", "text/html")
				w.Header().Add("Cache-Control", "no-cache")
				w.Header().Add("Strict-Transport-Security", "max-age=86400; includeSubDomains")
				w.WriteHeader(http.StatusOK)

				component := components.StartPage(version, assetLoader.Load, components.Home(assetLoader.Load))
				component.Render(r.Context(), w)
			}
		}())

		r.Get("/{component}", func() http.HandlerFunc {

			comps := map[string]templ.Component{
				"/home":    components.Home(assetLoader.Load),
				"/sensors": components.Sensors(assetLoader.Load),
			}

			return func(w http.ResponseWriter, r *http.Request) {

				w.Header().Add("Content-Type", "text/html")
				w.Header().Add("Cache-Control", "no-cache")
				w.Header().Add("Strict-Transport-Security", "max-age=86400; includeSubDomains")
				w.WriteHeader(http.StatusOK)

				component, ok := comps[r.URL.Path]
				if !ok {
					http.Error(w, "not found", http.StatusNotFound)
					return
				}

				if isHxRequest(r) {
					component.Render(r.Context(), w)
					return
				}

				component = components.StartPage(version, assetLoader.Load, component)
				component.Render(r.Context(), w)
			}
		}())

		r.Get("/assets/{sha}/{filename}", func() http.HandlerFunc {
			return func(w http.ResponseWriter, r *http.Request) {
				pathComponents := strings.Split(r.URL.Path, "/")
				sha := pathComponents[2]

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
			}
		}())

		r.Get("/favicon.ico", func() http.HandlerFunc {
			faviconPath := assetLoader.Load("/icons/favicon.ico").Path()
			return func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, faviconPath, http.StatusFound)
			}
		}())
	})

	return &impl{
		webapp: app,
		router: router,
	}, nil
}

func (a *impl) Router() *chi.Mux {
	return a.router
}
