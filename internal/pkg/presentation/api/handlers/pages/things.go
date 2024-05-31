package pages

import (
	"context"
	"net/http"

	"github.com/diwise/diwise-web/internal/pkg/presentation/api/helpers"
	"github.com/diwise/diwise-web/internal/pkg/presentation/locale"
	"github.com/diwise/diwise-web/internal/pkg/presentation/web/assets"
	"github.com/diwise/diwise-web/internal/pkg/presentation/web/components"
)

func NewThingsPage(ctx context.Context, l10n locale.Bundle, assets assets.AssetLoaderFunc) http.HandlerFunc {
	version := helpers.GetVersion(ctx)

	fn := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/html")
		w.Header().Add("Cache-Control", "no-cache")
		w.Header().Add("Strict-Transport-Security", "max-age=86400; includeSubDomains")
		w.WriteHeader(http.StatusOK)

		ctx = helpers.Decorate(
			r.Context(),
			components.CurrentComponent, "objects",
		)

		localizer := l10n.For(r.Header.Get("Accept-Language"))

		component := components.StartPage(
			version, localizer,
			assets, components.Objects(localizer, assets),
		)

		component.Render(ctx, w)

	}
	return http.HandlerFunc(fn)
}
