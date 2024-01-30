package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/diwise/diwise-web/internal/pkg/application"
	"github.com/diwise/diwise-web/internal/pkg/presentation/web/assets"
	"github.com/diwise/diwise-web/internal/pkg/presentation/web/components"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Api interface {
	Router() *chi.Mux
}

type impl struct {
	webapp application.WebApp
	router *chi.Mux
}

func New(ctx context.Context, app application.WebApp, assetPath string) (Api, error) {

	router := chi.NewRouter()
	router.Use(middleware.Logger)

	//logger := logging.GetFromContext(ctx)

	assetLoader, _ := assets.NewLoader(ctx, assets.BasePath(assetPath))

	router.Get("/", func() http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {

			w.Header().Add("Content-Type", "text/html")
			w.Header().Add("Cache-Control", "no-cache")
			w.Header().Add("Strict-Transport-Security", "max-age=86400; includeSubDomains")
			w.WriteHeader(http.StatusOK)

			component := components.StartPage(assetLoader.Load)
			component.Render(r.Context(), w)
		}
	}())

	router.Get("/assets/{sha}/{filename}", func() http.HandlerFunc {
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

	return &impl{
		webapp: app,
		router: router,
	}, nil
}

func (a *impl) Router() *chi.Mux {
	return a.router
}
